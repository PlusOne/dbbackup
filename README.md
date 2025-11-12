# dbbackup# dbbackup



![dbbackup](dbbackup.png)![dbbackup](dbbackup.png)



Professional database backup and restore utility for PostgreSQL, MySQL, and MariaDB with support for large databases and cluster operations.Database backup utility for PostgreSQL and MySQL with support for large databases.



## Overview## Recent Changes (November 2025)



`dbbackup` is a production-ready database backup tool designed for reliability, performance, and ease of use. It provides both interactive and command-line interfaces for single database backups, cluster-wide operations, and disaster recovery scenarios.### 🎯 ETA Estimation for Long Operations

- Real-time progress tracking with time estimates

### Key Features- Shows elapsed time and estimated time remaining

- Format: "X/Y (Z%) | Elapsed: 25m | ETA: ~40m remaining"

- **Multi-Database Support**: PostgreSQL, MySQL, and MariaDB- Particularly useful for 2+ hour cluster backups

- **Backup Modes**: Single database, sample data, and full cluster- Works with both CLI and TUI modes

- **Restore Operations**: Single database and full cluster restore with safety checks

- **Performance**: Automatic CPU detection, parallel processing, and streaming compression### 🔐 Authentication Detection & Smart Guidance

- **Large Database Handling**: Optimized for databases from gigabytes to terabytes- Detects OS user vs DB user mismatches

- **Interactive Interface**: Full-featured terminal UI with real-time progress tracking- Identifies PostgreSQL authentication methods (peer/ident/md5)

- **Cross-Platform**: Pre-compiled binaries for Linux, macOS, FreeBSD, OpenBSD, NetBSD- Shows helpful error messages with 4 solutions before connection attempt

- **Production Ready**: Comprehensive error handling, logging, and safety checks- Auto-loads passwords from `~/.pgpass` file

- Prevents confusing TLS/authentication errors in TUI mode

## Installation- Works across all Linux distributions



### Pre-compiled Binaries### 🗄️ MariaDB Support

- MariaDB now selectable as separate database type in interactive mode

Download the appropriate binary for your platform:- Press Enter to cycle: PostgreSQL → MySQL → MariaDB

- Stored as distinct type in configuration

```bash

# Linux (x86_64)### 🎨 UI Improvements

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup- Conservative terminal colors for better compatibility

chmod +x dbbackup- Fixed operation history navigation (arrow keys, viewport scrolling)

- Clean plain text display without styling artifacts

# Linux (ARM64)- 15-item viewport with scroll indicators

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_arm64 -o dbbackup

chmod +x dbbackup### Large Database Handling

- Streaming compression reduces memory usage by ~90%

# macOS (Intel)- Native pgx v5 driver reduces memory by ~48% compared to lib/pq

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup- Automatic format selection based on database size

chmod +x dbbackup- Per-database timeout configuration (default: 240 minutes)

- Parallel compression support via pigz when available

# macOS (Apple Silicon)

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup### Memory Usage

chmod +x dbbackup

| Database Size | Memory Usage |

# FreeBSD, OpenBSD, NetBSD - see bin/ directory for other platforms|---------------|--------------|

```| 10GB | ~850MB |

| 25GB | ~920MB |

### Build from Source| 50GB | ~940MB |

| 100GB+ | <1GB |

Requirements: Go 1.19 or later

### Progress Tracking

```bash

git clone https://git.uuxo.net/uuxo/dbbackup.git- Real-time progress indicators

cd dbbackup- Step-by-step operation tracking

go build -o dbbackup- Structured logging with timestamps

```- Operation history



## Quick Start## Features



### Interactive Mode (Recommended)- PostgreSQL and MySQL support

- Single database, sample, and cluster backup modes

The interactive terminal interface provides guided backup and restore operations:- CPU detection and parallel job optimization

- Interactive terminal interface

```bash- Cross-platform binaries (Linux, macOS, Windows, BSD)

# PostgreSQL (requires peer authentication)- SSL/TLS support

sudo -u postgres ./dbbackup interactive- Configurable compression levels



# MySQL/MariaDB## Installation

./dbbackup interactive --db-type mysql --user root --password <password>

```### Pre-compiled Binaries



### Command Line InterfaceDownload the binary for your platform:



```bash```bash

# Single database backup# Linux (Intel/AMD)

./dbbackup backup single myapp_productioncurl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup

chmod +x dbbackup

# Full cluster backup (PostgreSQL only)

./dbbackup backup cluster# macOS (Intel)

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup

# Restore from backupchmod +x dbbackup

./dbbackup restore single /path/to/backup.dump --target myapp_production

# macOS (Apple Silicon)

# Cluster restore with safety checkscurl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup

./dbbackup restore cluster /path/to/cluster_backup.tar.gz --confirmchmod +x dbbackup

``````



## Usage### Build from Source



### Backup Operations```bash

git clone https://git.uuxo.net/uuxo/dbbackup.git

#### Single Database Backupcd dbbackup

go build -o dbbackup main.go

```bash```

./dbbackup backup single <database_name> [options]

## Usage

Options:

  --host string              Database host (default "localhost")### Interactive Mode

  --port int                 Database port (default 5432 for PostgreSQL, 3306 for MySQL)

  --user string              Database user (default "postgres")```bash

  --password string          Database password# PostgreSQL - must match OS user for peer authentication

  --backup-dir string        Backup directory (default "/var/lib/pgsql/db_backups")sudo -u postgres dbbackup interactive

  --compression int          Compression level 0-9 (default 6)

  --db-type string           Database type: postgres, mysql, mariadb (default "postgres")# Or specify user explicitly

  --insecure                 Disable SSL/TLSsudo -u postgres dbbackup interactive --user postgres

```

# MySQL/MariaDB

Example:dbbackup interactive --db-type mysql --user root

```bash```

./dbbackup backup single production_db \

  --host db.example.com \Interactive mode provides menu navigation with arrow keys and automatic status updates.

  --user backup_user \

  --password <password> \**Authentication Note:** For PostgreSQL with peer authentication, run as the postgres user to avoid connection errors.

  --compression 9 \

  --backup-dir /mnt/backups### Command Line

```

```bash

#### Cluster Backup (PostgreSQL)# Single database backup

dbbackup backup single myapp_db

Backs up all databases in a PostgreSQL cluster including roles and tablespaces:

# Sample backup (10% of data)

```bashdbbackup backup sample myapp_db --sample-ratio 10

./dbbackup backup cluster [options]

# Full cluster backup (PostgreSQL)

Options:dbbackup backup cluster

  --max-cores int           Maximum CPU cores to use (default: auto-detect)

  --cpu-workload string     Workload type: cpu-intensive, io-intensive, balanced (default "balanced")# With custom settings

  --jobs int                Number of parallel jobs (default: auto-detect)dbbackup backup single myapp_db \

```  --host db.example.com \

  --port 5432 \

Example:  --user backup_user \

```bash  --ssl-mode require

sudo -u postgres ./dbbackup backup cluster \```

  --compression 3 \

  --max-cores 16 \### System Commands

  --cpu-workload cpu-intensive

``````bash

# Check connection status

#### Sample Backupdbbackup status



Create backups with reduced data for testing/development:# Run preflight checks

dbbackup preflight

```bash

./dbbackup backup sample <database_name> [options]# List databases and backups

dbbackup list

Options:

  --sample-strategy string  Strategy: ratio, percent, count (default "ratio")# Show CPU information

  --sample-value float      Sample value based on strategy (default 10)dbbackup cpu

``````



Examples:## Configuration

```bash

# Keep 10% of rows### Command Line Flags

./dbbackup backup sample myapp_db --sample-strategy percent --sample-value 10

| Flag | Description | Default |

# Keep 1 in 100 rows|------|-------------|---------|

./dbbackup backup sample myapp_db --sample-strategy ratio --sample-value 100| `--host` | Database host | `localhost` |

| `--port` | Database port | `5432` (PostgreSQL), `3306` (MySQL) |

# Keep 10,000 rows per table| `--user` | Database user | `postgres` |

./dbbackup backup sample myapp_db --sample-strategy count --sample-value 10000| `--database` | Database name | `postgres` |

```| `-d`, `--db-type` | Database type | `postgres` |

| `--ssl-mode` | SSL mode | `prefer` |

### Restore Operations| `--jobs` | Parallel jobs | Auto-detected |

| `--dump-jobs` | Parallel dump jobs | Auto-detected |

#### Single Database Restore| `--compression` | Compression level (0-9) | `6` |

| `--backup-dir` | Backup directory | `/var/lib/pgsql/db_backups` |

```bash

./dbbackup restore single <backup_file> [options]### PostgreSQL



Options:#### Authentication Methods

  --target string           Target database name (required)

  --create                  Create database if it doesn't existPostgreSQL uses different authentication methods depending on your system configuration:

  --clean                   Drop and recreate database before restore

  --jobs int                Number of parallel restore jobs (default 4)**Peer Authentication (most common on Linux):**

``````bash

# Must run as postgres user

Example:sudo -u postgres dbbackup backup cluster

```bash

./dbbackup restore single /backups/myapp_20250111.dump \# If you see this error: "Ident authentication failed for user postgres"

  --target myapp_restored \# Use one of these solutions:

  --create \```

  --jobs 8

```**Solution 1: Use matching OS user (recommended)**

```bash

#### Cluster Restore (PostgreSQL)sudo -u postgres dbbackup status --user postgres

```

Restore an entire PostgreSQL cluster from backup:

**Solution 2: Configure ~/.pgpass file**

```bash```bash

./dbbackup restore cluster <archive_file> [options]echo "localhost:5432:*:postgres:your_password" > ~/.pgpass

chmod 0600 ~/.pgpass

Options:dbbackup status --user postgres

  --confirm                 Confirm and execute restore (required for safety)```

  --dry-run                 Show what would be done without executing

  --force                   Skip safety checks**Solution 3: Set PGPASSWORD environment variable**

  --jobs int                Number of parallel decompression jobs (default: auto)```bash

```export PGPASSWORD=your_password

dbbackup status --user postgres

Example:```

```bash

sudo -u postgres ./dbbackup restore cluster /backups/cluster_20250111.tar.gz --confirm**Solution 4: Use --password flag**

``````bash

dbbackup status --user postgres --password your_password

**Safety Features:**```

- Pre-restore validation of archive integrity

- Disk space checks#### SSL Configuration

- Verification of required tools (psql, pg_restore, tar, gzip)

- Automatic detection and cleanup of existing databases (interactive mode)SSL modes: `disable`, `prefer`, `require`, `verify-ca`, `verify-full`

- Progress tracking with ETA estimation

Cluster operations (backup/restore/verify) are PostgreSQL-only.

### Disaster Recovery

### MySQL / MariaDB

For complete disaster recovery scenarios, use the included script:

Set `--db-type mysql` or `--db-type mariadb`:

```bash```bash

sudo ./disaster_recovery_test.shdbbackup backup single mydb \

```  --db-type mysql \

  --host 127.0.0.1 \

This script performs:  --user backup_user \

1. Full cluster backup with maximum performance  --password ****

2. Documentation of current state```

3. Controlled destruction of all user databases (with confirmation)

4. Complete cluster restorationMySQL backups are created as `.sql.gz` files.

5. Verification of database integrity

### Environment Variables

**Warning:** This is a destructive operation. Only use in test environments or genuine disaster recovery scenarios.

```bash

## Configuration# Database

export PG_HOST=localhost

### PostgreSQL Authenticationexport PG_PORT=5432

export PG_USER=postgres

PostgreSQL authentication varies by system configuration. The tool automatically detects issues and provides solutions.export PGPASSWORD=secret

export MYSQL_HOST=localhost

#### Peer/Ident Authentication (Default on Linux)export MYSQL_PWD=secret



Run as the PostgreSQL system user:# Backup

export BACKUP_DIR=/var/backups

```bashexport COMPRESS_LEVEL=6

sudo -u postgres ./dbbackup backup clusterexport CLUSTER_TIMEOUT_MIN=240  # Cluster timeout in minutes

```

# Swap file management (Linux + root only)

#### Password Authenticationexport AUTO_SWAP=false

export SWAP_FILE_SIZE_GB=8

Option 1 - Using .pgpass file (recommended for automation):export SWAP_FILE_PATH=/tmp/dbbackup_swap

```bash```

echo "localhost:5432:*:postgres:password" > ~/.pgpass

chmod 0600 ~/.pgpass## Architecture

./dbbackup backup single mydb --user postgres

``````

dbbackup/

Option 2 - Environment variable:├── cmd/                    # CLI commands

```bash├── internal/

export PGPASSWORD=your_password│   ├── config/            # Configuration

./dbbackup backup single mydb --user postgres│   ├── database/          # Database drivers

```│   ├── backup/            # Backup engine

│   ├── cpu/               # CPU detection

Option 3 - Command line flag:│   ├── logger/            # Logging

```bash│   ├── progress/          # Progress indicators

./dbbackup backup single mydb --user postgres --password your_password│   └── tui/               # Terminal UI

```└── bin/                   # Binaries

```

### MySQL/MariaDB Authentication

### Supported Platforms

```bash

# Command lineLinux (amd64, arm64, armv7), macOS (amd64, arm64), Windows (amd64, arm64), FreeBSD, OpenBSD, NetBSD

./dbbackup backup single mydb --db-type mysql --user root --password your_password

## Performance

# Environment variable

export MYSQL_PWD=your_password### CPU Detection

./dbbackup backup single mydb --db-type mysql --user root

The tool detects CPU configuration and adjusts parallelism automatically:

# Configuration file

cat > ~/.my.cnf << EOF```bash

[client]dbbackup cpu

user=backup_user```

password=your_password

host=localhost### Large Database Handling

EOF

chmod 0600 ~/.my.cnfStreaming architecture maintains constant memory usage regardless of database size. Databases >5GB automatically use plain format. Parallel compression via pigz is used when available.

```

### Memory Usage Notes

### Environment Variables

- Small databases (<1GB): ~500MB

```bash- Medium databases (1-10GB): ~800MB

# PostgreSQL- Large databases (10-50GB): ~900MB  

export PG_HOST=localhost- Huge databases (50GB+): ~1GB

export PG_PORT=5432

export PG_USER=postgres## Troubleshooting

export PGPASSWORD=password

### Connection Issues

# MySQL/MariaDB

export MYSQL_HOST=localhost**Authentication Errors (PostgreSQL):**

export MYSQL_PORT=3306

export MYSQL_USER=rootIf you see: `FATAL: Peer authentication failed for user "postgres"` or `FATAL: Ident authentication failed`

export MYSQL_PWD=password

The tool will automatically show you 4 solutions:

# General1. Run as matching OS user: `sudo -u postgres dbbackup`

export BACKUP_DIR=/var/backups/databases2. Configure ~/.pgpass file (recommended for automation)

export COMPRESS_LEVEL=63. Set PGPASSWORD environment variable

export CLUSTER_TIMEOUT_MIN=2404. Use --password flag

```

**Test connection:**

## Performance```bash

dbbackup status

### CPU Optimization

# Disable SSL

The tool automatically detects CPU configuration and optimizes parallel operations:dbbackup status --insecure



```bash# Use postgres user (Linux)

./dbbackup cpusudo -u postgres dbbackup status

``````



Manual override:### Out of Memory Issues

```bash

./dbbackup backup cluster --max-cores 32 --jobs 32 --cpu-workload cpu-intensiveCheck kernel logs for OOM events:

``````bash

dmesg | grep -i oom

### Memory Usagefree -h

```

Streaming architecture maintains constant memory usage:

Enable swap file management (Linux + root):

| Database Size | Memory Usage |```bash

|---------------|--------------|export AUTO_SWAP=true

| 1-10 GB      | ~800 MB      |export SWAP_FILE_SIZE_GB=8

| 10-50 GB     | ~900 MB      |sudo dbbackup backup cluster

| 50-100 GB    | ~950 MB      |```

| 100+ GB      | <1 GB        |

Or manually add swap:

### Large Database Support```bash

sudo fallocate -l 8G /swapfile

- Databases >5GB automatically use optimized plain format with streaming compressionsudo chmod 600 /swapfile

- Parallel compression via pigz (if available) for maximum throughputsudo mkswap /swapfile

- Per-database timeout configuration (default: 4 hours)sudo swapon /swapfile

- Automatic format selection based on size```



## System Commands### Debug Mode



```bash```bash

# Check database connection and configurationdbbackup backup single mydb --debug

./dbbackup status```



# Run pre-flight checks before backup## Documentation

./dbbackup preflight

- [AUTHENTICATION_PLAN.md](AUTHENTICATION_PLAN.md) - Authentication handling across distributions

# List available databases- [PROGRESS_IMPLEMENTATION.md](PROGRESS_IMPLEMENTATION.md) - ETA estimation implementation

./dbbackup list- [HUGE_DATABASE_QUICK_START.md](HUGE_DATABASE_QUICK_START.md) - Quick start for large databases

- [LARGE_DATABASE_OPTIMIZATION_PLAN.md](LARGE_DATABASE_OPTIMIZATION_PLAN.md) - Optimization details

# Display CPU information- [PRIORITY2_PGX_INTEGRATION.md](PRIORITY2_PGX_INTEGRATION.md) - pgx v5 integration

./dbbackup cpu

## License

# Show version information

./dbbackup versionMIT License

```

## Repository

## Troubleshooting

https://git.uuxo.net/uuxo/dbbackup
### Connection Issues

Test connectivity:
```bash
./dbbackup status
```

For PostgreSQL peer authentication errors:
```bash
sudo -u postgres ./dbbackup status
```

For SSL/TLS issues:
```bash
./dbbackup status --insecure
```

### Out of Memory

If experiencing memory issues with very large databases:

1. Check available memory:
```bash
free -h
dmesg | grep -i oom
```

2. Add swap space:
```bash
sudo fallocate -l 16G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

3. Reduce parallelism:
```bash
./dbbackup backup cluster --jobs 4 --dump-jobs 4
```

### Debug Mode

Enable detailed logging:
```bash
./dbbackup backup single mydb --debug
```

### Common Error Messages

**"Ident authentication failed"** - Run as matching OS user or configure password authentication

**"Permission denied"** - Check database user privileges or run with appropriate system user

**"Disk space check failed"** - Ensure sufficient space in backup directory (4x archive size recommended)

**"Archive validation failed"** - Backup file may be corrupted or incomplete

## Building All Platform Binaries

To build binaries for all supported platforms:

```bash
./build_all.sh
```

Binaries will be created in the `bin/` directory.

## Project Structure

```
dbbackup/
├── main.go                 # Application entry point
├── cmd/                    # CLI command implementations
├── internal/
│   ├── backup/            # Backup engine
│   ├── restore/           # Restore engine with safety checks
│   ├── config/            # Configuration management
│   ├── database/          # Database drivers (PostgreSQL, MySQL)
│   ├── cpu/               # CPU detection and optimization
│   ├── logger/            # Structured logging
│   ├── progress/          # Progress tracking and ETA estimation
│   └── tui/               # Interactive terminal interface
├── bin/                   # Pre-compiled binaries
├── disaster_recovery_test.sh  # Disaster recovery testing script
└── build_all.sh           # Multi-platform build script
```

## Requirements

### System Requirements

- Linux, macOS, FreeBSD, OpenBSD, or NetBSD
- 1 GB RAM minimum (2 GB recommended for large databases)
- Sufficient disk space for backups (typically 30-50% of database size)

### Software Requirements

#### PostgreSQL
- PostgreSQL client tools (psql, pg_dump, pg_dumpall, pg_restore)
- PostgreSQL 10 or later recommended

#### MySQL/MariaDB
- MySQL/MariaDB client tools (mysql, mysqldump)
- MySQL 5.7+ or MariaDB 10.3+ recommended

#### Optional
- pigz (for parallel compression)
- pv (for progress monitoring)

## Best Practices

1. **Test Restores Regularly** - Verify backups can be restored successfully
2. **Monitor Disk Space** - Ensure adequate space for backup operations
3. **Use Compression** - Balance between speed and space (level 3-6 recommended)
4. **Automate Backups** - Schedule regular backups via cron or systemd timers
5. **Secure Credentials** - Use .pgpass or .my.cnf files with proper permissions (0600)
6. **Version Control** - Keep multiple backup versions for point-in-time recovery
7. **Off-Site Storage** - Copy backups to remote storage for disaster recovery
8. **Document Procedures** - Maintain runbooks for restore operations

## Support

For issues, questions, or contributions:

- Repository: https://git.uuxo.net/uuxo/dbbackup
- Report issues via the repository issue tracker

## License

MIT License - see repository for details
