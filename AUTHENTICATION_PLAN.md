# Database Authentication Enhancement Plan

## Current Situation Analysis

### PostgreSQL Authentication Methods (by Distribution)

#### Current System: CentOS Stream 10
- **Local (Unix socket)**: `peer` authentication
  - Requires OS username = PostgreSQL username
  - Example: `sudo -u postgres ./dbbackup status --user postgres` ✅
  - Fails: `./dbbackup status --user postgres` ❌ (peer auth failed)

- **TCP (localhost)**: `ident` authentication  
  - Uses identd protocol to verify OS username
  - Similar to peer but over TCP

#### Common PostgreSQL Auth Methods Across Distributions
1. **peer** - Unix socket only, OS user must match DB user
2. **ident** - TCP/IP, uses identd to verify OS user
3. **md5/scram-sha-256** - Password-based (most common for remote)
4. **trust** - No authentication (development only)
5. **cert** - SSL certificate-based
6. **ldap/pam** - Enterprise integration

### MySQL/MariaDB Authentication
- Typically uses password-based authentication by default
- Can use unix_socket plugin (similar to peer)
- Less likely to have peer/ident issues

## Problem Statement

When user runs:
```bash
./dbbackup status --user postgres
```

The tool attempts to connect as "postgres" user, but:
1. **Root user context**: OS user is "root", PostgreSQL expects "postgres"
2. **Peer auth fails**: `FATAL: Peer authentication failed for user "postgres"`
3. **User must know**: Need `sudo -u postgres` or provide password

## Solution Strategy: Multi-Level Authentication

### Level 1: Smart OS User Detection (Quick Win)
**Goal**: Detect when OS user ≠ DB user and provide helpful guidance

**Implementation**:
```go
// Check if OS user matches requested DB user
currentOSUser := getCurrentUser()
requestedDBUser := cfg.User

if currentOSUser != requestedDBUser {
    // Check authentication method
    authMethod := detectPostgreSQLAuthMethod(cfg.Host, cfg.Port)
    
    if authMethod == "peer" || authMethod == "ident" {
        // Peer/ident requires OS user = DB user
        if cfg.Password == "" {
            // No password provided, suggest sudo
            log.Warn("Authentication mismatch detected",
                "os_user", currentOSUser,
                "db_user", requestedDBUser,
                "auth_method", authMethod)
            fmt.Printf("\n⚠️  Authentication Note:\n")
            fmt.Printf("   PostgreSQL is using '%s' authentication\n", authMethod)
            fmt.Printf("   OS user '%s' cannot authenticate as DB user '%s'\n", 
                currentOSUser, requestedDBUser)
            fmt.Printf("\n💡 Solutions:\n")
            fmt.Printf("   1. Run as matching user: sudo -u %s %s\n", 
                requestedDBUser, os.Args[0])
            fmt.Printf("   2. Provide password: %s --password <password>\n", 
                os.Args[0])
            fmt.Printf("   3. Set PGPASSWORD environment variable\n")
            fmt.Printf("   4. Configure ~/.pgpass file\n\n")
            
            return fmt.Errorf("authentication method requires matching OS user")
        }
    }
}
```

### Level 2: Auto-Sudo Wrapper (Medium Effort)
**Goal**: Automatically re-execute with sudo when needed

**Implementation**:
```go
func autoSudoIfNeeded(cfg *Config) error {
    currentUser := getCurrentUser()
    
    // Check if we need sudo and aren't already using it
    if currentUser != cfg.User && os.Getenv("DBBACKUP_SUDO_RETRY") == "" {
        authMethod := detectAuthMethod(cfg)
        
        if authMethod == "peer" || authMethod == "ident" {
            if cfg.Password == "" {
                fmt.Printf("🔄 Auto-retrying with sudo as user '%s'...\n", cfg.User)
                
                // Re-execute with sudo
                cmd := exec.Command("sudo", "-u", cfg.User, os.Args[0], os.Args[1:]...)
                cmd.Env = append(os.Environ(), "DBBACKUP_SUDO_RETRY=1")
                cmd.Stdin = os.Stdin
                cmd.Stdout = os.Stdout
                cmd.Stderr = os.Stderr
                
                if err := cmd.Run(); err != nil {
                    return fmt.Errorf("sudo retry failed: %w", err)
                }
                
                os.Exit(cmd.ProcessState.ExitCode())
            }
        }
    }
    
    return nil
}
```

### Level 3: pgpass Support (High Value)
**Goal**: Use ~/.pgpass file for password-less authentication

**Implementation**:
```go
// Check ~/.pgpass and /var/lib/pgsql/.pgpass
func loadPasswordFromPgpass(cfg *Config) (string, bool) {
    pgpassLocations := []string{
        filepath.Join(os.Getenv("HOME"), ".pgpass"),
        "/var/lib/pgsql/.pgpass",
        filepath.Join("/home", cfg.User, ".pgpass"),
    }
    
    for _, pgpassPath := range pgpassLocations {
        if password := parsePgpass(pgpassPath, cfg); password != "" {
            return password, true
        }
    }
    
    return "", false
}

// Format: hostname:port:database:username:password
func parsePgpass(path string, cfg *Config) string {
    file, err := os.Open(path)
    if err != nil {
        return ""
    }
    defer file.Close()
    
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        
        parts := strings.Split(line, ":")
        if len(parts) != 5 {
            continue
        }
        
        host, port, db, user, pass := parts[0], parts[1], parts[2], parts[3], parts[4]
        
        // Match hostname (* = wildcard)
        if host != "*" && host != cfg.Host {
            continue
        }
        
        // Match port (* = wildcard)
        if port != "*" && port != strconv.Itoa(cfg.Port) {
            continue
        }
        
        // Match database (* = wildcard)
        if db != "*" && db != cfg.Database {
            continue
        }
        
        // Match user (* = wildcard)
        if user != "*" && user != cfg.User {
            continue
        }
        
        return pass
    }
    
    return ""
}
```

### Level 4: Smart Environment Detection (Advanced)
**Goal**: Detect distribution and suggest optimal configuration

**Implementation**:
```go
type OSDistribution struct {
    Name              string
    Family            string // debian, redhat, arch, etc.
    PostgreSQLVersion string
    DefaultAuthMethod string
    SocketLocation    string
    SuggestedUser     string
}

func detectDistribution() *OSDistribution {
    // Read /etc/os-release
    data, err := os.ReadFile("/etc/os-release")
    if err != nil {
        return &OSDistribution{Name: "unknown"}
    }
    
    content := string(data)
    dist := &OSDistribution{}
    
    // Parse os-release
    for _, line := range strings.Split(content, "\n") {
        if strings.HasPrefix(line, "ID=") {
            dist.Name = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
        }
        if strings.HasPrefix(line, "ID_LIKE=") {
            dist.Family = strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"")
        }
    }
    
    // Distribution-specific defaults
    switch dist.Name {
    case "centos", "rhel", "fedora":
        dist.DefaultAuthMethod = "peer"
        dist.SocketLocation = "/var/run/postgresql"
        dist.SuggestedUser = "postgres"
        
    case "debian", "ubuntu":
        dist.DefaultAuthMethod = "peer"
        dist.SocketLocation = "/var/run/postgresql"
        dist.SuggestedUser = "postgres"
        
    case "arch", "manjaro":
        dist.DefaultAuthMethod = "peer"
        dist.SocketLocation = "/run/postgresql"
        dist.SuggestedUser = "postgres"
        
    case "alpine":
        dist.DefaultAuthMethod = "md5"
        dist.SocketLocation = "/run/postgresql"
        dist.SuggestedUser = "postgres"
    }
    
    return dist
}
```

## Implementation Phases

### Phase 1: Detection & Guidance (1-2 hours)
- ✅ Detect OS user vs DB user mismatch
- ✅ Detect PostgreSQL authentication method (peer/ident/md5)
- ✅ Provide helpful error messages with solutions
- ✅ Show example commands for current system

**Files to modify**:
- `internal/config/config.go` - Add OS user detection
- `internal/database/postgresql.go` - Add auth method detection
- `cmd/root.go` - Add pre-connection validation

### Phase 2: pgpass Support (2-3 hours)
- ✅ Read and parse ~/.pgpass file
- ✅ Support wildcard matching
- ✅ Check multiple pgpass locations
- ✅ Fall back to password prompt if needed

**Files to modify**:
- `internal/config/config.go` - Add pgpass loading
- `internal/database/postgresql.go` - Integrate pgpass passwords

### Phase 3: Auto-Sudo (3-4 hours) - OPTIONAL
- ⚠️ Automatically detect when sudo is needed
- ⚠️ Re-execute command with sudo -u
- ⚠️ Preserve all arguments and flags
- ⚠️ Handle interactive prompts

**Considerations**:
- Security implications of auto-sudo
- May surprise users (implicit behavior change)
- Could interfere with scripting/automation

### Phase 4: Distribution-Aware Setup (4-5 hours) - OPTIONAL
- Detect Linux distribution
- Provide distribution-specific guidance
- Auto-configure optimal settings
- Generate setup scripts for first-run

## Recommended Approach: Phase 1 + Phase 2

**Why this combination?**
1. **Phase 1**: Immediate value - users understand what's wrong
2. **Phase 2**: Standard PostgreSQL solution - no surprises
3. **Skip Phase 3**: Auto-sudo can be confusing/dangerous
4. **Skip Phase 4**: Users know their own distribution

**User Experience Flow**:
```bash
# User runs without proper auth
$ ./dbbackup status --user postgres
⚠️  Authentication Note:
   PostgreSQL is using 'peer' authentication
   OS user 'root' cannot authenticate as DB user 'postgres'

💡 Solutions:
   1. Run as matching user: sudo -u postgres ./dbbackup
   2. Provide password: ./dbbackup --password <password>
   3. Set PGPASSWORD environment variable
   4. Configure ~/.pgpass file (recommended)

📝 To create ~/.pgpass file:
   echo "localhost:5432:*:postgres:yourpassword" > ~/.pgpass
   chmod 0600 ~/.pgpass

# User fixes authentication
$ sudo -u postgres ./dbbackup status --user postgres
✅ Connected successfully
```

## Testing Matrix

### PostgreSQL Authentication Methods
- [ ] **peer** (Unix socket) - CentOS/RHEL default
- [ ] **ident** (TCP/IP) - Some distributions
- [ ] **md5** (Password) - Common for remote
- [ ] **scram-sha-256** (Password) - Modern PostgreSQL
- [ ] **trust** (No auth) - Development only

### Operating Systems
- [ ] **CentOS Stream 10** (current system)
- [ ] **Ubuntu 22.04/24.04** (most popular)
- [ ] **Debian 12** (stable)
- [ ] **Fedora 40** (cutting edge)
- [ ] **Alpine Linux** (containers)

### Scenarios
- [ ] Root user connecting as postgres user
- [ ] Postgres user connecting as postgres user
- [ ] Regular user with pgpass file
- [ ] Regular user with PGPASSWORD env
- [ ] Regular user with --password flag
- [ ] TCP vs Unix socket connections
- [ ] Remote database connections

## Security Considerations

1. **pgpass file permissions**: Must be 0600 (owner read/write only)
2. **Password in command line**: Discourage --password flag (visible in ps)
3. **PGPASSWORD env**: Better than command line, but still visible
4. **Auto-sudo**: Could be security risk if not carefully implemented
5. **Error messages**: Don't expose sensitive connection details

## Backward Compatibility

✅ **No breaking changes** - All existing workflows continue to work:
- `sudo -u postgres ./dbbackup` ✅
- `PGPASSWORD=secret ./dbbackup` ✅  
- `./dbbackup --password secret` ✅
- Current socket detection logic ✅

## Conclusion

**Recommended Implementation**: Phase 1 + Phase 2
- **Effort**: 3-5 hours total
- **Value**: High - users can authenticate without sudo
- **Risk**: Low - using standard PostgreSQL mechanisms
- **Complexity**: Medium - well-defined scope

**Skip**: Phase 3 (Auto-sudo)
- Can surprise users with implicit behavior
- Security implications
- Not standard PostgreSQL practice

**Defer**: Phase 4 (Distribution detection)
- Nice-to-have but not essential
- Users generally know their own system
- Can be added later if needed
