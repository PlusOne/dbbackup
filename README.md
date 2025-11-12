# dbbackup# dbbackup# dbbackup



![dbbackup](dbbackup.png)

Professional database backup and restore utility for PostgreSQL, MySQL, and MariaDB with support for large databases and cluster operations.Database backup utility for PostgreSQL and MySQL with support for large databases.

Professional database backup and restore utility for PostgreSQL, MySQL, and MariaDB with support for large databases and cluster operations.



## Overview

## Overview## Recent Changes (November 2025)

`dbbackup` is a production-ready database backup tool designed for reliability, performance, and ease of use. It provides both interactive and command-line interfaces for single database backups, cluster-wide operations, and disaster recovery scenarios.



### Key Features

`dbbackup` is a production-ready database backup tool designed for reliability, performance, and ease of use. It provides both interactive and command-line interfaces for single database backups, cluster-wide operations, and disaster recovery scenarios.### 🎯 ETA Estimation for Long Operations

- **Multi-Database Support**: PostgreSQL, MySQL, and MariaDB

- **Backup Modes**: Single database, sample data, and full cluster- Real-time progress tracking with time estimates

- **Restore Operations**: Single database and full cluster restore with safety checks

- **Performance**: Automatic CPU detection, parallel processing, and streaming compression### Key Features- Shows elapsed time and estimated time remaining

- **Large Database Handling**: Optimized for databases from gigabytes to terabytes

- **Interactive Interface**: Full-featured terminal UI with real-time progress tracking- Format: "X/Y (Z%) | Elapsed: 25m | ETA: ~40m remaining"

- **Cross-Platform**: Pre-compiled binaries for Linux, macOS, FreeBSD, OpenBSD, NetBSD

- **Production Ready**: Comprehensive error handling, logging, and safety checks- **Multi-Database Support**: PostgreSQL, MySQL, and MariaDB- Particularly useful for 2+ hour cluster backups



## Installation- **Backup Modes**: Single database, sample data, and full cluster- Works with both CLI and TUI modes



### Pre-compiled Binaries- **Restore Operations**: Single database and full cluster restore with safety checks



Download the appropriate binary for your platform:- **Performance**: Automatic CPU detection, parallel processing, and streaming compression### 🔐 Authentication Detection & Smart Guidance



```bash- **Large Database Handling**: Optimized for databases from gigabytes to terabytes- Detects OS user vs DB user mismatches

# Linux (x86_64)

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup- **Interactive Interface**: Full-featured terminal UI with real-time progress tracking- Identifies PostgreSQL authentication methods (peer/ident/md5)

chmod +x dbbackup

- **Cross-Platform**: Pre-compiled binaries for Linux, macOS, FreeBSD, OpenBSD, NetBSD- Shows helpful error messages with 4 solutions before connection attempt

# Linux (ARM64)

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_arm64 -o dbbackup- **Production Ready**: Comprehensive error handling, logging, and safety checks- Auto-loads passwords from `~/.pgpass` file

chmod +x dbbackup

- Prevents confusing TLS/authentication errors in TUI mode

# macOS (Intel)

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup## Installation- Works across all Linux distributions

chmod +x dbbackup



# macOS (Apple Silicon)

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup### Pre-compiled Binaries### 🗄️ MariaDB Support

chmod +x dbbackup

- MariaDB now selectable as separate database type in interactive mode

# FreeBSD, OpenBSD, NetBSD - see bin/ directory for other platforms

```Download the appropriate binary for your platform:- Press Enter to cycle: PostgreSQL → MySQL → MariaDB



### Build from Source- Stored as distinct type in configuration



Requirements: Go 1.19 or later```bash



```bash# Linux (x86_64)### 🎨 UI Improvements

git clone https://git.uuxo.net/uuxo/dbbackup.git

cd dbbackupcurl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup- Conservative terminal colors for better compatibility

go build

```chmod +x dbbackup- Fixed operation history navigation (arrow keys, viewport scrolling)



## Quick Start- Clean plain text display without styling artifacts



### Interactive Mode (Recommended)# Linux (ARM64)- 15-item viewport with scroll indicators



The interactive terminal interface provides guided backup and restore operations:curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_arm64 -o dbbackup



```bashchmod +x dbbackup### Large Database Handling

# PostgreSQL (requires peer authentication)

sudo -u postgres ./dbbackup interactive- Streaming compression reduces memory usage by ~90%



# MySQL/MariaDB# macOS (Intel)- Native pgx v5 driver reduces memory by ~48% compared to lib/pq

./dbbackup interactive --db-type mysql --user root --password <password>

```curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup- Automatic format selection based on database size



### Command Line Interfacechmod +x dbbackup- Per-database timeout configuration (default: 240 minutes)



```bash- Parallel compression support via pigz when available

# Single database backup

./dbbackup backup single myapp_production# macOS (Apple Silicon)



# Full cluster backup (PostgreSQL only)curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup### Memory Usage

./dbbackup backup cluster

chmod +x dbbackup

# Restore from backup

./dbbackup restore single /path/to/backup.dump --target myapp_production| Database Size | Memory Usage |



# Cluster restore with safety checks# FreeBSD, OpenBSD, NetBSD - see bin/ directory for other platforms|---------------|--------------|

./dbbackup restore cluster /path/to/cluster_backup.tar.gz --confirm

``````| 10GB | ~850MB |



## Usage| 25GB | ~920MB |



### Backup Operations### Build from Source| 50GB | ~940MB |



#### Single Database Backup| 100GB+ | <1GB |



```bashRequirements: Go 1.19 or later

./dbbackup backup single <database_name> [options]

### Progress Tracking

Options:

  --host string              Database host (default "localhost")```bash

  --port int                 Database port (default 5432 for PostgreSQL, 3306 for MySQL)

  --user string              Database user (default "postgres")git clone https://git.uuxo.net/uuxo/dbbackup.git- Real-time progress indicators

  --password string          Database password

  --backup-dir string        Backup directory (default "/var/lib/pgsql/db_backups")cd dbbackup- Step-by-step operation tracking

  --compression int          Compression level 0-9 (default 6)

  --db-type string           Database type: postgres, mysql, mariadb (default "postgres")go build -o dbbackup- Structured logging with timestamps

  --insecure                 Disable SSL/TLS

``````- Operation history



Example:

```bash

./dbbackup backup single production_db \## Quick Start## Features

  --host db.example.com \

  --user backup_user \

  --password <password> \

  --compression 9 \### Interactive Mode (Recommended)- PostgreSQL and MySQL support

  --backup-dir /mnt/backups

```- Single database, sample, and cluster backup modes



#### Cluster Backup (PostgreSQL)The interactive terminal interface provides guided backup and restore operations:- CPU detection and parallel job optimization



Backs up all databases in a PostgreSQL cluster including roles and tablespaces:- Interactive terminal interface



```bash```bash- Cross-platform binaries (Linux, macOS, Windows, BSD)

./dbbackup backup cluster [options]

# PostgreSQL (requires peer authentication)- SSL/TLS support

Options:

  --max-cores int           Maximum CPU cores to use (default: auto-detect)sudo -u postgres ./dbbackup interactive- Configurable compression levels

  --cpu-workload string     Workload type: cpu-intensive, io-intensive, balanced (default "balanced")

  --jobs int                Number of parallel jobs (default: auto-detect)

```

# MySQL/MariaDB## Installation

Example:

```bash./dbbackup interactive --db-type mysql --user root --password <password>

sudo -u postgres ./dbbackup backup cluster \

  --compression 3 \```### Pre-compiled Binaries

  --max-cores 16 \

  --cpu-workload cpu-intensive

```

### Command Line InterfaceDownload the binary for your platform:

#### Sample Backup



Create backups with reduced data for testing/development:

```bash```bash

```bash

./dbbackup backup sample <database_name> [options]# Single database backup# Linux (Intel/AMD)



Options:./dbbackup backup single myapp_productioncurl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup

  --sample-strategy string  Strategy: ratio, percent, count (default "ratio")

  --sample-value float      Sample value based on strategy (default 10)chmod +x dbbackup

```

# Full cluster backup (PostgreSQL only)

Examples:

```bash./dbbackup backup cluster# macOS (Intel)

# Keep 10% of rows

./dbbackup backup sample myapp_db --sample-strategy percent --sample-value 10curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup



# Keep 1 in 100 rows# Restore from backupchmod +x dbbackup

./dbbackup backup sample myapp_db --sample-strategy ratio --sample-value 100

./dbbackup restore single /path/to/backup.dump --target myapp_production

# Keep 10,000 rows per table

./dbbackup backup sample myapp_db --sample-strategy count --sample-value 10000# macOS (Apple Silicon)

```

# Cluster restore with safety checkscurl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup

### Restore Operations

./dbbackup restore cluster /path/to/cluster_backup.tar.gz --confirmchmod +x dbbackup

#### Single Database Restore

``````

```bash

./dbbackup restore single <backup_file> [options]



Options:## Usage### Build from Source

  --target string           Target database name (required)

  --create                  Create database if it doesn't exist

  --clean                   Drop and recreate database before restore

  --jobs int                Number of parallel restore jobs (default 4)### Backup Operations```bash

```

git clone https://git.uuxo.net/uuxo/dbbackup.git

Example:

```bash#### Single Database Backupcd dbbackup

./dbbackup restore single /backups/myapp_20250111.dump \

  --target myapp_restored \go build -o dbbackup main.go

  --create \

  --jobs 8```bash```

```

./dbbackup backup single <database_name> [options]

#### Cluster Restore (PostgreSQL)

## Usage

Restore an entire PostgreSQL cluster from backup:

Options:

```bash

./dbbackup restore cluster <archive_file> [options]  --host string              Database host (default "localhost")### Interactive Mode



Options:  --port int                 Database port (default 5432 for PostgreSQL, 3306 for MySQL)

  --confirm                 Confirm and execute restore (required for safety)

  --dry-run                 Show what would be done without executing  --user string              Database user (default "postgres")```bash

  --force                   Skip safety checks

  --jobs int                Number of parallel decompression jobs (default: auto)  --password string          Database password# PostgreSQL - must match OS user for peer authentication

```

  --backup-dir string        Backup directory (default "/var/lib/pgsql/db_backups")sudo -u postgres dbbackup interactive

Example:

```bash  --compression int          Compression level 0-9 (default 6)

sudo -u postgres ./dbbackup restore cluster /backups/cluster_20250111.tar.gz --confirm

```  --db-type string           Database type: postgres, mysql, mariadb (default "postgres")# Or specify user explicitly



**Safety Features:**  --insecure                 Disable SSL/TLSsudo -u postgres dbbackup interactive --user postgres

- Pre-restore validation of archive integrity

- Disk space checks```

- Verification of required tools (psql, pg_restore, tar, gzip)

- Automatic detection and cleanup of existing databases (interactive mode)# MySQL/MariaDB

- Progress tracking with ETA estimation

Example:dbbackup interactive --db-type mysql --user root

### Disaster Recovery

```bash```

For complete disaster recovery scenarios, use the included script:

./dbbackup backup single production_db \

```bash

sudo ./disaster_recovery_test.sh  --host db.example.com \Interactive mode provides menu navigation with arrow keys and automatic status updates.

```

  --user backup_user \

This script performs:

1. Full cluster backup with maximum performance  --password <password> \**Authentication Note:** For PostgreSQL with peer authentication, run as the postgres user to avoid connection errors.

2. Documentation of current state

3. Controlled destruction of all user databases (with confirmation)  --compression 9 \

4. Complete cluster restoration

5. Verification of database integrity  --backup-dir /mnt/backups### Command Line



**Warning:** This is a destructive operation. Only use in test environments or genuine disaster recovery scenarios.```



## Configuration```bash



### PostgreSQL Authentication#### Cluster Backup (PostgreSQL)# Single database backup



PostgreSQL authentication varies by system configuration. The tool automatically detects issues and provides solutions.dbbackup backup single myapp_db



#### Peer/Ident Authentication (Default on Linux)Backs up all databases in a PostgreSQL cluster including roles and tablespaces:



Run as the PostgreSQL system user:# Sample backup (10% of data)



```bash```bashdbbackup backup sample myapp_db --sample-ratio 10

sudo -u postgres ./dbbackup backup cluster

```./dbbackup backup cluster [options]



#### Password Authentication# Full cluster backup (PostgreSQL)



Option 1 - Using .pgpass file (recommended for automation):Options:dbbackup backup cluster

```bash

echo "localhost:5432:*:postgres:password" > ~/.pgpass  --max-cores int           Maximum CPU cores to use (default: auto-detect)

chmod 0600 ~/.pgpass

./dbbackup backup single mydb --user postgres  --cpu-workload string     Workload type: cpu-intensive, io-intensive, balanced (default "balanced")# With custom settings

```

  --jobs int                Number of parallel jobs (default: auto-detect)dbbackup backup single myapp_db \

Option 2 - Environment variable:

```bash```  --host db.example.com \

export PGPASSWORD=your_password

./dbbackup backup single mydb --user postgres  --port 5432 \

```

Example:  --user backup_user \

Option 3 - Command line flag:

```bash```bash  --ssl-mode require

./dbbackup backup single mydb --user postgres --password your_password

```sudo -u postgres ./dbbackup backup cluster \```



### MySQL/MariaDB Authentication  --compression 3 \



```bash  --max-cores 16 \### System Commands

# Command line

./dbbackup backup single mydb --db-type mysql --user root --password your_password  --cpu-workload cpu-intensive



# Environment variable``````bash

export MYSQL_PWD=your_password

./dbbackup backup single mydb --db-type mysql --user root# Check connection status



# Configuration file#### Sample Backupdbbackup status

cat > ~/.my.cnf << EOF

[client]

user=backup_user

password=your_passwordCreate backups with reduced data for testing/development:# Run preflight checks

host=localhost

EOFdbbackup preflight

chmod 0600 ~/.my.cnf

``````bash



### Environment Variables./dbbackup backup sample <database_name> [options]# List databases and backups



```bashdbbackup list

# PostgreSQL

export PG_HOST=localhostOptions:

export PG_PORT=5432

export PG_USER=postgres  --sample-strategy string  Strategy: ratio, percent, count (default "ratio")# Show CPU information

export PGPASSWORD=password

  --sample-value float      Sample value based on strategy (default 10)dbbackup cpu

# MySQL/MariaDB

export MYSQL_HOST=localhost``````

export MYSQL_PORT=3306

export MYSQL_USER=root

export MYSQL_PWD=password

Examples:## Configuration

# General

export BACKUP_DIR=/var/backups/databases```bash

export COMPRESS_LEVEL=6

export CLUSTER_TIMEOUT_MIN=240# Keep 10% of rows### Command Line Flags

```

./dbbackup backup sample myapp_db --sample-strategy percent --sample-value 10

## Performance

| Flag | Description | Default |

### CPU Optimization

# Keep 1 in 100 rows|------|-------------|---------|

The tool automatically detects CPU configuration and optimizes parallel operations:

./dbbackup backup sample myapp_db --sample-strategy ratio --sample-value 100| `--host` | Database host | `localhost` |

```bash

./dbbackup cpu| `--port` | Database port | `5432` (PostgreSQL), `3306` (MySQL) |

```

# Keep 10,000 rows per table| `--user` | Database user | `postgres` |

Manual override:

```bash./dbbackup backup sample myapp_db --sample-strategy count --sample-value 10000| `--database` | Database name | `postgres` |

./dbbackup backup cluster --max-cores 32 --jobs 32 --cpu-workload cpu-intensive

``````| `-d`, `--db-type` | Database type | `postgres` |



### Memory Usage| `--ssl-mode` | SSL mode | `prefer` |



Streaming architecture maintains constant memory usage:### Restore Operations| `--jobs` | Parallel jobs | Auto-detected |



| Database Size | Memory Usage || `--dump-jobs` | Parallel dump jobs | Auto-detected |

|---------------|--------------|

| 1-10 GB      | ~800 MB      |#### Single Database Restore| `--compression` | Compression level (0-9) | `6` |

| 10-50 GB     | ~900 MB      |

| 50-100 GB    | ~950 MB      || `--backup-dir` | Backup directory | `/var/lib/pgsql/db_backups` |

| 100+ GB      | <1 GB        |

```bash

### Large Database Support

./dbbackup restore single <backup_file> [options]### PostgreSQL

- Databases >5GB automatically use optimized plain format with streaming compression

- Parallel compression via pigz (if available) for maximum throughput

- Per-database timeout configuration (default: 4 hours)

- Automatic format selection based on sizeOptions:#### Authentication Methods



## System Commands  --target string           Target database name (required)



```bash  --create                  Create database if it doesn't existPostgreSQL uses different authentication methods depending on your system configuration:

# Check database connection and configuration

./dbbackup status  --clean                   Drop and recreate database before restore



# Run pre-flight checks before backup  --jobs int                Number of parallel restore jobs (default 4)**Peer Authentication (most common on Linux):**

./dbbackup preflight

``````bash

# List available databases

./dbbackup list# Must run as postgres user



# Display CPU informationExample:sudo -u postgres dbbackup backup cluster

./dbbackup cpu

```bash

# Show version information

./dbbackup version./dbbackup restore single /backups/myapp_20250111.dump \# If you see this error: "Ident authentication failed for user postgres"

```

  --target myapp_restored \# Use one of these solutions:

## Troubleshooting

  --create \```

### Connection Issues

  --jobs 8

Test connectivity:

```bash```**Solution 1: Use matching OS user (recommended)**

./dbbackup status

``````bash



For PostgreSQL peer authentication errors:#### Cluster Restore (PostgreSQL)sudo -u postgres dbbackup status --user postgres

```bash

sudo -u postgres ./dbbackup status```

```

Restore an entire PostgreSQL cluster from backup:

For SSL/TLS issues:

```bash**Solution 2: Configure ~/.pgpass file**

./dbbackup status --insecure

``````bash```bash



### Out of Memory./dbbackup restore cluster <archive_file> [options]echo "localhost:5432:*:postgres:your_password" > ~/.pgpass



If experiencing memory issues with very large databases:chmod 0600 ~/.pgpass



1. Check available memory:Options:dbbackup status --user postgres

```bash

free -h  --confirm                 Confirm and execute restore (required for safety)```

dmesg | grep -i oom

```  --dry-run                 Show what would be done without executing



2. Add swap space:  --force                   Skip safety checks**Solution 3: Set PGPASSWORD environment variable**

```bash

sudo fallocate -l 16G /swapfile  --jobs int                Number of parallel decompression jobs (default: auto)```bash

sudo chmod 600 /swapfile

sudo mkswap /swapfile```export PGPASSWORD=your_password

sudo swapon /swapfile

```dbbackup status --user postgres



3. Reduce parallelism:Example:```

```bash

./dbbackup backup cluster --jobs 4 --dump-jobs 4```bash

```

sudo -u postgres ./dbbackup restore cluster /backups/cluster_20250111.tar.gz --confirm**Solution 4: Use --password flag**

### Debug Mode

``````bash

Enable detailed logging:

```bashdbbackup status --user postgres --password your_password

./dbbackup backup single mydb --debug

```**Safety Features:**```



### Common Error Messages- Pre-restore validation of archive integrity



**"Ident authentication failed"** - Run as matching OS user or configure password authentication- Disk space checks#### SSL Configuration



**"Permission denied"** - Check database user privileges or run with appropriate system user- Verification of required tools (psql, pg_restore, tar, gzip)



**"Disk space check failed"** - Ensure sufficient space in backup directory (4x archive size recommended)- Automatic detection and cleanup of existing databases (interactive mode)SSL modes: `disable`, `prefer`, `require`, `verify-ca`, `verify-full`



**"Archive validation failed"** - Backup file may be corrupted or incomplete- Progress tracking with ETA estimation



## Building All Platform BinariesCluster operations (backup/restore/verify) are PostgreSQL-only.



To build binaries for all supported platforms:### Disaster Recovery



```bash### MySQL / MariaDB

./build_all.sh

```For complete disaster recovery scenarios, use the included script:



Binaries will be created in the `bin/` directory.Set `--db-type mysql` or `--db-type mariadb`:



## Project Structure```bash```bash



```sudo ./disaster_recovery_test.shdbbackup backup single mydb \

dbbackup/

├── main.go                 # Application entry point```  --db-type mysql \

├── cmd/                    # CLI command implementations

├── internal/  --host 127.0.0.1 \

│   ├── backup/            # Backup engine

│   ├── restore/           # Restore engine with safety checksThis script performs:  --user backup_user \

│   ├── config/            # Configuration management

│   ├── database/          # Database drivers (PostgreSQL, MySQL)1. Full cluster backup with maximum performance  --password ****

│   ├── cpu/               # CPU detection and optimization

│   ├── logger/            # Structured logging2. Documentation of current state```

│   ├── progress/          # Progress tracking and ETA estimation

│   └── tui/               # Interactive terminal interface3. Controlled destruction of all user databases (with confirmation)

├── bin/                   # Pre-compiled binaries

├── disaster_recovery_test.sh  # Disaster recovery testing script4. Complete cluster restorationMySQL backups are created as `.sql.gz` files.

└── build_all.sh           # Multi-platform build script

```5. Verification of database integrity



## Requirements### Environment Variables



### System Requirements**Warning:** This is a destructive operation. Only use in test environments or genuine disaster recovery scenarios.



- Linux, macOS, FreeBSD, OpenBSD, or NetBSD```bash

- 1 GB RAM minimum (2 GB recommended for large databases)

- Sufficient disk space for backups (typically 30-50% of database size)## Configuration# Database



### Software Requirementsexport PG_HOST=localhost



#### PostgreSQL### PostgreSQL Authenticationexport PG_PORT=5432

- PostgreSQL client tools (psql, pg_dump, pg_dumpall, pg_restore)

- PostgreSQL 10 or later recommendedexport PG_USER=postgres



#### MySQL/MariaDBPostgreSQL authentication varies by system configuration. The tool automatically detects issues and provides solutions.export PGPASSWORD=secret

- MySQL/MariaDB client tools (mysql, mysqldump)

- MySQL 5.7+ or MariaDB 10.3+ recommendedexport MYSQL_HOST=localhost



#### Optional#### Peer/Ident Authentication (Default on Linux)export MYSQL_PWD=secret

- pigz (for parallel compression)

- pv (for progress monitoring)



## Best PracticesRun as the PostgreSQL system user:# Backup



1. **Test Restores Regularly** - Verify backups can be restored successfullyexport BACKUP_DIR=/var/backups

2. **Monitor Disk Space** - Ensure adequate space for backup operations

3. **Use Compression** - Balance between speed and space (level 3-6 recommended)```bashexport COMPRESS_LEVEL=6

4. **Automate Backups** - Schedule regular backups via cron or systemd timers

5. **Secure Credentials** - Use .pgpass or .my.cnf files with proper permissions (0600)sudo -u postgres ./dbbackup backup clusterexport CLUSTER_TIMEOUT_MIN=240  # Cluster timeout in minutes

6. **Version Control** - Keep multiple backup versions for point-in-time recovery

7. **Off-Site Storage** - Copy backups to remote storage for disaster recovery```

8. **Document Procedures** - Maintain runbooks for restore operations

# Swap file management (Linux + root only)

## Support

#### Password Authenticationexport AUTO_SWAP=false

For issues, questions, or contributions:

export SWAP_FILE_SIZE_GB=8

- Repository: https://git.uuxo.net/uuxo/dbbackup

- Report issues via the repository issue trackerOption 1 - Using .pgpass file (recommended for automation):export SWAP_FILE_PATH=/tmp/dbbackup_swap



## License```bash```



MIT License - see repository for detailsecho "localhost:5432:*:postgres:password" > ~/.pgpass


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
