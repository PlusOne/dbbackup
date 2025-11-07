package auth

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"dbbackup/internal/config"
)

// AuthMethod represents a PostgreSQL authentication method
type AuthMethod string

const (
	AuthPeer         AuthMethod = "peer"
	AuthIdent        AuthMethod = "ident"
	AuthMD5          AuthMethod = "md5"
	AuthScramSHA256  AuthMethod = "scram-sha-256"
	AuthPassword     AuthMethod = "password"
	AuthTrust        AuthMethod = "trust"
	AuthUnknown      AuthMethod = "unknown"
)

// DetectPostgreSQLAuthMethod attempts to detect the authentication method
// by reading pg_hba.conf or checking common patterns
func DetectPostgreSQLAuthMethod(host string, port int, user string) AuthMethod {
	// For localhost connections, check pg_hba.conf
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		if method := checkPgHbaConf(user); method != AuthUnknown {
			return method
		}
	}

	// Default assumptions based on connection type
	if host == "localhost" {
		return AuthPeer // Most common for local connections
	}

	return AuthMD5 // Most common for remote connections
}

// checkPgHbaConf reads pg_hba.conf to determine authentication method
func checkPgHbaConf(user string) AuthMethod {
	// Common pg_hba.conf locations
	locations := []string{
		"/var/lib/pgsql/data/pg_hba.conf",
		"/etc/postgresql/*/main/pg_hba.conf",
		"/var/lib/postgresql/data/pg_hba.conf",
		"/usr/local/pgsql/data/pg_hba.conf",
	}

	// Try to find pg_hba.conf using psql if available
	if hbaFile := findHbaFileViaPostgres(); hbaFile != "" {
		locations = append([]string{hbaFile}, locations...)
	}

	for _, location := range locations {
		if method := parsePgHbaConf(location, user); method != AuthUnknown {
			return method
		}
	}

	return AuthUnknown
}

// findHbaFileViaPostgres asks PostgreSQL for the hba_file location
func findHbaFileViaPostgres() string {
	cmd := exec.Command("psql", "-U", "postgres", "-t", "-c", "SHOW hba_file;")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// parsePgHbaConf parses pg_hba.conf and returns the authentication method
func parsePgHbaConf(path string, user string) AuthMethod {
	// Try with sudo if we can't read directly
	file, err := os.Open(path)
	if err != nil {
		// Try with sudo
		cmd := exec.Command("sudo", "cat", path)
		output, err := cmd.Output()
		if err != nil {
			return AuthUnknown
		}
		return parseHbaContent(string(output), user)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var content strings.Builder
	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	return parseHbaContent(content.String(), user)
}

// parseHbaContent parses the content of pg_hba.conf
func parseHbaContent(content string, user string) AuthMethod {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse HBA line: TYPE DATABASE USER ADDRESS METHOD
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		connType := fields[0]
		// database := fields[1]
		hbaUser := fields[2]
		var method string

		if connType == "local" {
			// local all all peer
			if len(fields) >= 4 {
				method = fields[3]
			}
		} else if connType == "host" {
			// host all all 127.0.0.1/32 md5
			if len(fields) >= 5 {
				method = fields[4]
			}
		}

		// Check if this rule applies to our user
		if hbaUser == "all" || hbaUser == user {
			switch strings.ToLower(method) {
			case "peer":
				return AuthPeer
			case "ident":
				return AuthIdent
			case "md5":
				return AuthMD5
			case "scram-sha-256":
				return AuthScramSHA256
			case "password":
				return AuthPassword
			case "trust":
				return AuthTrust
			}
		}
	}

	return AuthUnknown
}

// CheckAuthenticationMismatch checks if there's an authentication mismatch
// and returns helpful guidance if needed
func CheckAuthenticationMismatch(cfg *config.Config) (bool, string) {
	// Only check for PostgreSQL
	if !cfg.IsPostgreSQL() {
		return false, ""
	}

	// Get current OS user
	currentOSUser := config.GetCurrentOSUser()
	requestedDBUser := cfg.User

	// If users match, no problem
	if currentOSUser == requestedDBUser {
		return false, ""
	}

	// If password is provided, user can authenticate
	if cfg.Password != "" {
		return false, ""
	}

	// Detect authentication method
	authMethod := DetectPostgreSQLAuthMethod(cfg.Host, cfg.Port, cfg.User)

	// peer and ident require OS user = DB user
	if authMethod == AuthPeer || authMethod == AuthIdent {
		return true, buildAuthMismatchMessage(currentOSUser, requestedDBUser, authMethod)
	}

	return false, ""
}

// buildAuthMismatchMessage creates a helpful error message
func buildAuthMismatchMessage(osUser, dbUser string, method AuthMethod) string {
	var msg strings.Builder

	msg.WriteString("\n⚠️  Authentication Mismatch Detected\n")
	msg.WriteString(strings.Repeat("=", 60) + "\n\n")
	
	msg.WriteString(fmt.Sprintf("   PostgreSQL is using '%s' authentication\n", method))
	msg.WriteString(fmt.Sprintf("   OS user '%s' cannot authenticate as DB user '%s'\n\n", osUser, dbUser))
	
	msg.WriteString("💡 Solutions (choose one):\n\n")
	
	msg.WriteString(fmt.Sprintf("   1. Run as matching user:\n"))
	msg.WriteString(fmt.Sprintf("      sudo -u %s %s\n\n", dbUser, getCommandLine()))
	
	msg.WriteString("   2. Configure ~/.pgpass file (recommended):\n")
	msg.WriteString(fmt.Sprintf("      echo \"localhost:5432:*:%s:your_password\" > ~/.pgpass\n", dbUser))
	msg.WriteString("      chmod 0600 ~/.pgpass\n\n")
	
	msg.WriteString("   3. Set PGPASSWORD environment variable:\n")
	msg.WriteString(fmt.Sprintf("      export PGPASSWORD=your_password\n"))
	msg.WriteString(fmt.Sprintf("      %s\n\n", getCommandLine()))
	
	msg.WriteString("   4. Provide password via flag:\n")
	msg.WriteString(fmt.Sprintf("      %s --password your_password\n\n", getCommandLine()))
	
	msg.WriteString("📝 Note: For production use, ~/.pgpass or PGPASSWORD are recommended\n")
	msg.WriteString("         to avoid exposing passwords in command history.\n\n")
	
	msg.WriteString(strings.Repeat("=", 60) + "\n")

	return msg.String()
}

// getCommandLine reconstructs the current command line
func getCommandLine() string {
	if len(os.Args) == 0 {
		return "./dbbackup"
	}
	
	// Build command without password if present
	var parts []string
	skipNext := false
	
	for _, arg := range os.Args {
		if skipNext {
			skipNext = false
			continue
		}
		
		if arg == "--password" || arg == "-p" {
			skipNext = true
			continue
		}
		
		if strings.HasPrefix(arg, "--password=") {
			continue
		}
		
		parts = append(parts, arg)
	}
	
	return strings.Join(parts, " ")
}

// LoadPasswordFromPgpass attempts to load password from .pgpass file
func LoadPasswordFromPgpass(cfg *config.Config) (string, bool) {
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

// parsePgpass reads and parses .pgpass file
// Format: hostname:port:database:username:password
func parsePgpass(path string, cfg *config.Config) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	// Check file permissions (should be 0600)
	info, err := file.Stat()
	if err != nil {
		return ""
	}

	// Warn if permissions are too open (but still use it)
	if info.Mode().Perm() != 0600 {
		// Silently skip - PostgreSQL also skips world-readable pgpass files
		return ""
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
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
