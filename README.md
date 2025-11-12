# dbbackup

![dbbackup](dbbackup.png)

Professional database backup and restore utility for PostgreSQL, MySQL, and MariaDB.

## Key Features

- Multi-database support: PostgreSQL, MySQL, MariaDB
- Backup modes: Single database, cluster, sample data
- Restore operations with safety checks and validation
- Automatic CPU detection and parallel processing
- Streaming compression for large databases
- Interactive terminal UI with progress tracking
- Cross-platform binaries (Linux, macOS, BSD)

## Installation

### Download Pre-compiled Binary

Linux x86_64:

```bash
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup
chmod +x dbbackup
```

Linux ARM64:

```bash
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_arm64 -o dbbackup
chmod +x dbbackup
```

macOS Intel:

```bash
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup
chmod +x dbbackup
```

macOS Apple Silicon:

```bash
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup
chmod +x dbbackup
```

Other platforms available in `bin/` directory: FreeBSD, OpenBSD, NetBSD.

### Build from Source

Requires Go 1.19 or later:

```bash
git clone https://git.uuxo.net/uuxo/dbbackup.git
cd dbbackup
go build
```

## Quick Start

### Interactive Mode

PostgreSQL (peer authentication):

```bash
sudo -u postgres ./dbbackup interactive
```

MySQL/MariaDB:

```bash
./dbbackup interactive --db-type mysql --user root --password secret
```

Menu-driven interface for all operations. Press arrow keys to navigate, Enter to select.

#### Interactive Features

- **Backup Selection**: Choose backup type (single, cluster, sample)
- **Database Selection**: Browse and select database
- **Configuration Settings**: Set compression, parallelism, CPU workload, performance options
- **CPU Workload Profiles**: Balanced, CPU-Intensive, or I/O-Intensive (auto-adjusts Jobs/DumpJobs)
- **Backup Management**: List, restore, verify, and delete backup archives
- **Safety Checks**: Archive validation, disk space verification
- **Progress Tracking**: Real-time progress with ETA estimation
- **Restore Options**: Smart database cleanup detection, safety confirmations

### Command Line Mode

Backup single database:

```bash
./dbbackup backup single myapp_db
```

Backup entire cluster (PostgreSQL):

```bash
./dbbackup backup cluster
```

Restore database:

```bash
./dbbackup restore single backup.dump --target myapp_db --create
```

Restore full cluster:

```bash
./dbbackup restore cluster cluster_backup.tar.gz --confirm
```

## Commands

### Global Flags (Available for all commands)

| Flag | Description | Default |
|------|-------------|---------|
| `-d, --db-type` | postgres, mysql, mariadb | postgres |
| `--host` | Database host | localhost |
| `--port` | Database port | 5432 (postgres), 3306 (mysql) |
| `--user` | Database user | root |
| `--password` | Database password | (empty) |
| `--database` | Database name | postgres |
| `--backup-dir` | Backup directory | /root/db_backups |
| `--compression` | Compression level 0-9 | 6 |
| `--ssl-mode` | disable, prefer, require, verify-ca, verify-full | prefer |
| `--insecure` | Disable SSL/TLS | false |
| `--jobs` | Parallel jobs | 8 |
| `--dump-jobs` | Parallel dump jobs | 8 |
| `--max-cores` | Maximum CPU cores | 16 |
| `--cpu-workload` | cpu-intensive, io-intensive, balanced | balanced |
| `--auto-detect-cores` | Auto-detect CPU cores | true |
| `--debug` | Enable debug logging | false |
| `--no-color` | Disable colored output | false |

### Backup Operations

#### Single Database

Backup a single database to compressed archive:

```bash
./dbbackup backup single DATABASE_NAME [OPTIONS]
```

**Common Options:**

- `--host STRING` - Database host (default: localhost)
- `--port INT` - Database port (default: 5432 PostgreSQL, 3306 MySQL)
- `--user STRING` - Database user (default: postgres)
- `--password STRING` - Database password
- `--db-type STRING` - Database type: postgres, mysql, mariadb (default: postgres)
- `--backup-dir STRING` - Backup directory (default: /var/lib/pgsql/db_backups)
- `--compression INT` - Compression level 0-9 (default: 6)
- `--insecure` - Disable SSL/TLS
- `--ssl-mode STRING` - SSL mode: disable, prefer, require, verify-ca, verify-full

**Examples:**

```bash
# Basic backup
./dbbackup backup single production_db

# Remote database with custom settings
./dbbackup backup single myapp_db \
  --host db.example.com \
  --port 5432 \
  --user backup_user \
  --password secret \
  --compression 9 \
  --backup-dir /mnt/backups

# MySQL database
./dbbackup backup single wordpress \
  --db-type mysql \
  --user root \
  --password secret
```

Supported formats:
- PostgreSQL: Custom format (.dump) or SQL (.sql)
- MySQL/MariaDB: SQL (.sql)

#### Cluster Backup (PostgreSQL)

Backup all databases in PostgreSQL cluster including roles and tablespaces:

```bash
./dbbackup backup cluster [OPTIONS]
```

**Performance Options:**

- `--max-cores INT` - Maximum CPU cores (default: auto-detect)
- `--cpu-workload STRING` - Workload type: cpu-intensive, io-intensive, balanced (default: balanced)
- `--jobs INT` - Parallel jobs (default: auto-detect based on workload)
- `--dump-jobs INT` - Parallel dump jobs (default: auto-detect based on workload)
- `--cluster-parallelism INT` - Concurrent database operations (default: 2, configurable via CLUSTER_PARALLELISM env var)

**Examples:**

```bash
# Standard cluster backup
sudo -u postgres ./dbbackup backup cluster

# High-performance backup
sudo -u postgres ./dbbackup backup cluster \
  --compression 3 \
  --max-cores 16 \
  --cpu-workload cpu-intensive \
  --jobs 16
```

Output: tar.gz archive containing all databases and globals.

#### Sample Backup

Create reduced-size backup for testing/development:

```bash
./dbbackup backup sample DATABASE_NAME [OPTIONS]
```

**Options:**

- `--sample-strategy STRING` - Strategy: ratio, percent, count (default: ratio)
- `--sample-value FLOAT` - Sample value based on strategy (default: 10)

**Examples:**

```bash
# Keep 10% of all rows
./dbbackup backup sample myapp_db --sample-strategy percent --sample-value 10

# Keep 1 in 100 rows
./dbbackup backup sample myapp_db --sample-strategy ratio --sample-value 100

# Keep 5000 rows per table
./dbbackup backup sample myapp_db --sample-strategy count --sample-value 5000
```

**Warning:** Sample backups may break referential integrity.

### Restore Operations

#### Single Database Restore

Restore database from backup file:

```bash
./dbbackup restore single BACKUP_FILE [OPTIONS]
```

**Options:**

- `--target STRING` - Target database name (required)
- `--create` - Create database if it doesn't exist
- `--clean` - Drop and recreate database before restore
- `--jobs INT` - Parallel restore jobs (default: 4)
- `--verbose` - Show detailed progress
- `--no-progress` - Disable progress indicators
- `--confirm` - Execute restore (required for safety, dry-run by default)
- `--dry-run` - Preview without executing
- `--force` - Skip safety checks

**Examples:**

```bash
# Basic restore
./dbbackup restore single /backups/myapp_20250112.dump --target myapp_restored

# Restore with database creation
./dbbackup restore single backup.dump \
  --target myapp_db \
  --create \
  --jobs 8

# Clean restore (drops existing database)
./dbbackup restore single backup.dump \
  --target myapp_db \
  --clean \
  --verbose
```

Supported formats:
- PostgreSQL: .dump, .dump.gz, .sql, .sql.gz
- MySQL: .sql, .sql.gz

#### Cluster Restore (PostgreSQL)

Restore entire PostgreSQL cluster from archive:

```bash
./dbbackup restore cluster ARCHIVE_FILE [OPTIONS]
```

**Options:**

- `--confirm` - Confirm and execute restore (required for safety)
- `--dry-run` - Show what would be done without executing
- `--force` - Skip safety checks
- `--jobs INT` - Parallel decompression jobs (default: auto)
- `--verbose` - Show detailed progress
- `--no-progress` - Disable progress indicators

**Examples:**

```bash
# Standard cluster restore
sudo -u postgres ./dbbackup restore cluster cluster_backup.tar.gz --confirm

# Dry-run to preview
sudo -u postgres ./dbbackup restore cluster cluster_backup.tar.gz --dry-run

# High-performance restore
sudo -u postgres ./dbbackup restore cluster cluster_backup.tar.gz \
  --confirm \
  --jobs 16 \
  --verbose
```

**Safety Features:**

- Archive integrity validation
- Disk space checks (4x archive size recommended)
- Automatic database cleanup detection (interactive mode)
- Progress tracking with ETA estimation

#### Restore List

Show available backup archives in backup directory:

```bash
./dbbackup restore list
```

### System Commands

#### Status Check

Check database connection and configuration:

```bash
./dbbackup status [OPTIONS]
```

Shows: Database type, host, port, user, connection status, available databases.

#### Preflight Checks

Run pre-backup validation checks:

```bash
./dbbackup preflight [OPTIONS]
```

Verifies: Database connection, required tools, disk space, permissions.

#### List Databases

List available databases:

```bash
./dbbackup list [OPTIONS]
```

#### CPU Information

Display CPU configuration and optimization settings:

```bash
./dbbackup cpu
```

Shows: CPU count, model, workload recommendation, suggested parallel jobs.

#### Version

Display version information:

```bash
./dbbackup version
```

## Configuration

### PostgreSQL Authentication

PostgreSQL uses different authentication methods based on system configuration.

**Peer/Ident Authentication (Linux Default)**

Run as postgres system user:

```bash
sudo -u postgres ./dbbackup backup cluster
```

**Password Authentication**

Option 1: .pgpass file (recommended for automation):

```bash
echo "localhost:5432:*:postgres:password" > ~/.pgpass
chmod 0600 ~/.pgpass
./dbbackup backup single mydb --user postgres
```

Option 2: Environment variable:

```bash
export PGPASSWORD=your_password
./dbbackup backup single mydb --user postgres
```

Option 3: Command line flag:

```bash
./dbbackup backup single mydb --user postgres --password your_password
```

### MySQL/MariaDB Authentication

**Option 1: Command line**

```bash
./dbbackup backup single mydb --db-type mysql --user root --password secret
```

**Option 2: Environment variable**

```bash
export MYSQL_PWD=your_password
./dbbackup backup single mydb --db-type mysql --user root
```

**Option 3: Configuration file**

```bash
cat > ~/.my.cnf << EOF
[client]
user=backup_user
password=your_password
host=localhost
EOF
chmod 0600 ~/.my.cnf
```

### Environment Variables

PostgreSQL:

```bash
export PG_HOST=localhost
export PG_PORT=5432
export PG_USER=postgres
export PGPASSWORD=password
```

MySQL/MariaDB:

```bash
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PWD=password
```

General:

```bash
export BACKUP_DIR=/var/backups/databases
export COMPRESS_LEVEL=6
export CLUSTER_TIMEOUT_MIN=240
```

### Database Types

- `postgres` - PostgreSQL
- `mysql` - MySQL
- `mariadb` - MariaDB

Select via:
- CLI: `-d postgres` or `--db-type postgres`
- Interactive: Arrow keys to cycle through options

## Performance

### Memory Usage

Streaming architecture maintains constant memory usage:

| Database Size | Memory Usage |
|---------------|--------------|
| 1-10 GB      | ~800 MB      |
| 10-50 GB     | ~900 MB      |
| 50-100 GB    | ~950 MB      |
| 100+ GB      | <1 GB        |

### Large Database Optimization

- Databases >5GB automatically use plain format with streaming compression
- Parallel compression via pigz (if available)
- Per-database timeout: 4 hours default
- Automatic format selection based on size

### CPU Optimization

Automatically detects CPU configuration and optimizes parallelism:

```bash
./dbbackup cpu
```

Manual override:

```bash
./dbbackup backup cluster \
  --max-cores 32 \
  --jobs 32 \
  --cpu-workload cpu-intensive
```

### Parallelism

```bash
./dbbackup backup cluster --jobs 16 --dump-jobs 16
```

- `--jobs` - Compression/decompression parallel jobs
- `--dump-jobs` - Database dump parallel jobs
- `--max-cores` - Limit CPU cores (default: 16)
- Cluster operations use worker pools with configurable parallelism (default: 2 concurrent databases)
- Set `CLUSTER_PARALLELISM` environment variable to adjust concurrent database operations

### CPU Workload

```bash
./dbbackup backup cluster --cpu-workload cpu-intensive
```

Options: `cpu-intensive`, `io-intensive`, `balanced` (default)

Workload types automatically adjust Jobs and DumpJobs:
- **Balanced**: Jobs = PhysicalCores, DumpJobs = PhysicalCores/2 (min 2)
- **CPU-Intensive**: Jobs = PhysicalCores×2, DumpJobs = PhysicalCores (more parallelism)
- **I/O-Intensive**: Jobs = PhysicalCores/2 (min 1), DumpJobs = 2 (less parallelism to avoid I/O contention)

Configure in interactive mode via Configuration Settings menu.

### Compression

```bash
./dbbackup backup single mydb --compression 9
```

- Level 0 = No compression (fastest)
- Level 6 = Balanced (default)
- Level 9 = Maximum compression (slowest)

### SSL/TLS Configuration

SSL modes: `disable`, `prefer`, `require`, `verify-ca`, `verify-full`

```bash
# Disable SSL
./dbbackup backup single mydb --insecure

# Require SSL
./dbbackup backup single mydb --ssl-mode require

# Verify certificate
./dbbackup backup single mydb --ssl-mode verify-full
```

## Disaster Recovery

Complete automated disaster recovery test:

```bash
sudo ./disaster_recovery_test.sh
```

This script:

1. Backs up entire cluster with maximum performance
2. Documents pre-backup state
3. Destroys all user databases (confirmation required)
4. Restores full cluster from backup
5. Verifies restoration success

**Warning:** Destructive operation. Use only in test environments.

## Troubleshooting

### Connection Issues

**Test connectivity:**

```bash
./dbbackup status
```

**PostgreSQL peer authentication error:**

```bash
sudo -u postgres ./dbbackup status
```

**SSL/TLS issues:**

```bash
./dbbackup status --insecure
```

### Out of Memory

**Check memory:**

```bash
free -h
dmesg | grep -i oom
```

**Add swap space:**

```bash
sudo fallocate -l 16G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

**Reduce parallelism:**

```bash
./dbbackup backup cluster --jobs 4 --dump-jobs 4
```

### Debug Mode

Enable detailed logging:

```bash
./dbbackup backup single mydb --debug
```

### Common Errors

- **"Ident authentication failed"** - Run as matching OS user or configure password authentication
- **"Permission denied"** - Check database user privileges
- **"Disk space check failed"** - Ensure 4x archive size available
- **"Archive validation failed"** - Backup file corrupted or incomplete

## Building

Build for all platforms:

```bash
./build_all.sh
```

Binaries created in `bin/` directory.

## Requirements

### System Requirements

- Linux, macOS, FreeBSD, OpenBSD, NetBSD
- 1 GB RAM minimum (2 GB recommended for large databases)
- Disk space: 30-50% of database size for backups

### Software Requirements

**PostgreSQL:**
- Client tools: psql, pg_dump, pg_dumpall, pg_restore
- PostgreSQL 10 or later

**MySQL/MariaDB:**
- Client tools: mysql, mysqldump
- MySQL 5.7+ or MariaDB 10.3+

**Optional:**
- pigz (parallel compression)
- pv (progress monitoring)

## Best Practices

1. **Test restores regularly** - Verify backups work
2. **Monitor disk space** - Ensure adequate space
3. **Use compression** - Balance speed/space (level 3-6)
4. **Automate backups** - cron or systemd timers
5. **Secure credentials** - .pgpass/.my.cnf with 0600 permissions
6. **Keep versions** - Multiple backup versions for point-in-time recovery
7. **Off-site storage** - Remote backup copies
8. **Document procedures** - Maintain restore runbooks

## Project Structure

```
dbbackup/
├── main.go                      # Entry point
├── cmd/                         # CLI commands
├── internal/
│   ├── backup/                  # Backup engine
│   ├── restore/                 # Restore engine
│   ├── config/                  # Configuration
│   ├── database/                # Database drivers
│   ├── cpu/                     # CPU detection
│   ├── logger/                  # Logging
│   ├── progress/                # Progress tracking
│   └── tui/                     # Interactive UI
├── bin/                         # Pre-compiled binaries
├── disaster_recovery_test.sh    # DR testing script
└── build_all.sh                 # Multi-platform build
```

## Support

- Repository: https://git.uuxo.net/uuxo/dbbackup
- Issues: Use repository issue tracker

## License

MIT License

## Recent Improvements

### Performance Optimizations
- **Parallel Cluster Operations**: Worker pool pattern for concurrent database backup/restore (3-5x speedup)
- **Memory Efficiency**: Streaming command output eliminates OOM errors on large restores
- **Optimized Goroutines**: Ticker-based progress indicators reduce CPU overhead
- **Configurable Concurrency**: Control parallel database operations via CLUSTER_PARALLELISM

### Interactive UI Enhancements
- **CPU Workload Profiles**: Configure workload type in settings (auto-adjusts parallelism)
- **Backup Management**: Delete archives directly from TUI with confirmation
- **Better Callbacks**: Fixed confirmation dialogs to execute correct actions
- **Cleaner Interface**: Configuration consolidated in Settings menu

### Bug Fixes
- Fixed OOM error during large cluster restores
- Fixed delete backup incorrectly triggering cluster backup
- Fixed signal handler cleanup preventing goroutine leaks
- Improved error handling and user feedback

## Why dbbackup?

- **Reliable** - Comprehensive safety checks, validation, and error handling
- **Simple** - Intuitive menu-driven interface or straightforward CLI
- **Fast** - Automatic CPU detection, parallel processing, streaming compression
- **Efficient** - Minimal memory footprint, even for huge databases (constant ~1GB regardless of size)
- **Flexible** - Multiple backup modes, compression levels, performance options
- **Safe** - Dry-run by default, archive verification, smart database cleanup
- **Complete** - Full cluster backup/restore, multiple formats
- **Scalable** - Handles databases from megabytes to terabytes

dbbackup is production-ready for backup and disaster recovery operations on PostgreSQL, MySQL, and MariaDB databases of any size.
