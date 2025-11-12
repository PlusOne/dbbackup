# dbbackup# dbbackup



![dbbackup](dbbackup.png)![dbbackup](dbbackup.png)



Database backup and restore utility for PostgreSQL, MySQL, and MariaDB.Professional database backup and restore utility for PostgreSQL, MySQL, and MariaDB.



## Installation## Overview



### Pre-built Binaries**dbbackup** is a production-ready database backup tool providing both interactive and command-line interfaces for backup and restore operations. Designed for reliability, performance, and ease of use.



Download for your platform:### Key Features



```bash- Multi-database support: PostgreSQL, MySQL, MariaDB

# Linux x86_64- Backup modes: Single database, cluster, sample data

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup- Restore operations with safety checks and validation

chmod +x dbbackup- Automatic CPU detection and parallel processing

- Streaming compression for large databases

# Linux ARM64- Interactive terminal UI with progress tracking

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_arm64 -o dbbackup- Cross-platform binaries (Linux, macOS, BSD)

chmod +x dbbackup

## Installation

# macOS Intel

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup### Download Pre-compiled Binary

chmod +x dbbackup

```bash

# macOS Apple Silicon# Linux x86_64

curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackupcurl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup

chmod +x dbbackupchmod +x dbbackup

```

# Linux ARM64

Other platforms available in `bin/` directory (FreeBSD, OpenBSD, NetBSD).curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_arm64 -o dbbackup

chmod +x dbbackup

### Build from Source

# macOS Intel

```bashcurl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup

git clone https://git.uuxo.net/uuxo/dbbackup.gitchmod +x dbbackup

cd dbbackup

go build# macOS Apple Silicon

```curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup

chmod +x dbbackup

## Quick Start```



### Interactive Mode (Recommended)Other platforms available in `bin/` directory: FreeBSD, OpenBSD, NetBSD, Windows.



```bash### Build from Source

./dbbackup interactive

```Requires Go 1.19 or later:



Menu-driven interface for all operations. Press arrow keys to navigate, Enter to select.```bash

git clone https://git.uuxo.net/uuxo/dbbackup.git

#### Interactive Featurescd dbbackup

go build

- **Backup Selection**: Choose backup type (single, cluster, sample)```

- **Database Selection**: Browse and select database

- **Configuration**: Set compression, parallelism, performance options## Quick Start

- **Safety Checks**: Archive validation, disk space verification

- **Progress Tracking**: Real-time progress with ETA estimation### Interactive Mode

- **Restore Options**: Smart database cleanup detection, safety confirmations

```bash

### Command Line Mode# PostgreSQL (peer authentication)

sudo -u postgres ./dbbackup interactive

```bash

# Backup single database# MySQL/MariaDB

./dbbackup backup single mydb./dbbackup interactive --db-type mysql --user root --password secret

```

# Backup entire cluster (PostgreSQL)

./dbbackup backup cluster### Command Line



# Restore from backup```bash

./dbbackup restore single mydb.dump.gz --target mydb_restored --confirm# Backup single database

./dbbackup backup single myapp_db

# Restore full cluster

./dbbackup restore cluster cluster_backup.tar.gz --confirm# Backup entire cluster (PostgreSQL)

```./dbbackup backup cluster



## Commands# Restore database

./dbbackup restore single backup.dump --target myapp_db --create

### Global Flags (Available for all commands)

# Restore cluster

| Flag | Description | Default |./dbbackup restore cluster cluster_backup.tar.gz --confirm

|------|-------------|---------|```

| `-d, --db-type` | postgres, mysql, mariadb | postgres |

| `--host` | Database host | localhost |## Backup Operations

| `--port` | Database port | 5432 (postgres), 3306 (mysql) |

| `--user` | Database user | root |### Single Database

| `--password` | Database password | (empty) |

| `--database` | Database name | postgres |Backup a single database to compressed archive:

| `--backup-dir` | Backup directory | /root/db_backups |

| `--compression` | Compression level 0-9 | 6 |```bash

| `--ssl-mode` | disable, prefer, require, verify-ca, verify-full | prefer |./dbbackup backup single DATABASE_NAME [OPTIONS]

| `--insecure` | Disable SSL/TLS | false |```

| `--jobs` | Parallel jobs | 8 |

| `--dump-jobs` | Parallel dump jobs | 8 |**Common Options:**

| `--max-cores` | Maximum CPU cores | 16 |

| `--cpu-workload` | cpu-intensive, io-intensive, balanced | balanced |- `--host STRING` - Database host (default: localhost)

| `--auto-detect-cores` | Auto-detect CPU cores | true |- `--port INT` - Database port (default: 5432 PostgreSQL, 3306 MySQL)

| `--debug` | Enable debug logging | false |- `--user STRING` - Database user (default: postgres)

| `--no-color` | Disable colored output | false |- `--password STRING` - Database password

- `--db-type STRING` - Database type: postgres, mysql, mariadb (default: postgres)

### Backup Commands- `--backup-dir STRING` - Backup directory (default: /var/lib/pgsql/db_backups)

- `--compression INT` - Compression level 0-9 (default: 6)

#### backup single - Single Database Backup- `--insecure` - Disable SSL/TLS

- `--ssl-mode STRING` - SSL mode: disable, prefer, require, verify-ca, verify-full

Create backup of one database with full schema and data.

**Examples:**

```bash

./dbbackup backup single <database> [flags]```bash

```# Basic backup

./dbbackup backup single production_db

Example:

```bash# Remote database with custom settings

./dbbackup backup single myapp_db \./dbbackup backup single myapp \

  --host db.example.com \  --host db.example.com \

  --user backup_user \  --port 5432 \

  --password secret \  --user backup_user \

  --compression 9  --password secret \

```  --compression 9 \

  --backup-dir /mnt/backups

Supported formats:

- PostgreSQL: Custom format (.dump) or SQL (.sql)# MySQL database

- MySQL/MariaDB: SQL (.sql)./dbbackup backup single wordpress \

  --db-type mysql \

#### backup cluster - Full Cluster Backup  --user root \

  --password secret

Complete backup of all databases, roles, and tablespaces (PostgreSQL only).```



```bash### Cluster Backup (PostgreSQL)

./dbbackup backup cluster [flags]

```Backup all databases in PostgreSQL cluster including roles and tablespaces:



Example:```bash

```bash./dbbackup backup cluster [OPTIONS]

sudo -u postgres ./dbbackup backup cluster \```

  --compression 3 \

  --max-cores 16 \**Performance Options:**

  --cpu-workload cpu-intensive

```- `--max-cores INT` - Maximum CPU cores (default: auto-detect)

- `--cpu-workload STRING` - Workload type: cpu-intensive, io-intensive, balanced (default: balanced)

Output: tar.gz archive containing all databases and globals.- `--jobs INT` - Parallel jobs (default: auto-detect)

- `--dump-jobs INT` - Parallel dump jobs (default: auto-detect)

#### backup sample - Sample Database Backup

**Examples:**

Reduced dataset backup for testing/development.

```bash

```bash# Standard cluster backup

./dbbackup backup sample <database> [flags]sudo -u postgres ./dbbackup backup cluster

```

# High-performance backup

Sample flags:sudo -u postgres ./dbbackup backup cluster \

- `--sample-strategy` - ratio, percent, count (default: ratio)  --compression 3 \

- `--sample-value` - Value based on strategy (default: 10)  --max-cores 16 \

- `--sample-ratio` - Take every Nth record  --cpu-workload cpu-intensive \

- `--sample-percent` - Take N% of records  --jobs 16

- `--sample-count` - Take first N records per table```



Examples:### Sample Backup

```bash

# Every 10th recordCreate reduced-size backup for testing/development:

./dbbackup backup sample mydb --sample-ratio 10

```bash

# 20% of records./dbbackup backup sample DATABASE_NAME [OPTIONS]

./dbbackup backup sample mydb --sample-percent 20```



# First 5000 records per table**Options:**

./dbbackup backup sample mydb --sample-count 5000

```- `--sample-strategy STRING` - Strategy: ratio, percent, count (default: ratio)

- `--sample-value FLOAT` - Sample value based on strategy (default: 10)

**Warning:** Sample backups may break referential integrity.

**Examples:**

### Restore Commands

```bash

#### restore single - Restore Single Database# Keep 10% of all rows

./dbbackup backup sample myapp_db --sample-strategy percent --sample-value 10

Restore database from backup archive.

# Keep 1 in 100 rows

```bash./dbbackup backup sample myapp_db --sample-strategy ratio --sample-value 100

./dbbackup restore single <archive-file> [flags]

```# Keep 5000 rows per table

./dbbackup backup sample myapp_db --sample-strategy count --sample-value 5000

Restore flags:```

- `--target` - Target database name (defaults to original)

- `--confirm` - Execute restore (required, dry-run by default)## Restore Operations

- `--clean` - Drop and recreate target database

- `--create` - Create database if not exists### Single Database Restore

- `--dry-run` - Preview without executing

- `--force` - Skip safety checksRestore database from backup file:

- `--verbose` - Detailed progress

- `--no-progress` - Disable progress indicators```bash

./dbbackup restore single BACKUP_FILE [OPTIONS]

Examples:```

```bash

# Preview restore**Options:**

./dbbackup restore single mydb.dump.gz

- `--target STRING` - Target database name (required)

# Restore to original database- `--create` - Create database if it doesn't exist

./dbbackup restore single mydb.dump.gz --confirm- `--clean` - Drop and recreate database before restore

- `--jobs INT` - Parallel restore jobs (default: 4)

# Restore to different database- `--verbose` - Show detailed progress

./dbbackup restore single mydb.dump.gz --target mydb_test --confirm- `--no-progress` - Disable progress indicators



# Clean target before restore**Examples:**

./dbbackup restore single mydb.dump.gz --clean --confirm

```bash

# Create if not exists# Basic restore

./dbbackup restore single mydb.dump.gz --create --confirm./dbbackup restore single /backups/myapp_20250112.dump --target myapp_restored

```

# Restore with database creation

Supported formats:./dbbackup restore single backup.dump \

- PostgreSQL: .dump, .dump.gz, .sql, .sql.gz  --target myapp_db \

- MySQL: .sql, .sql.gz  --create \

  --jobs 8

#### restore cluster - Restore Full Cluster

# Clean restore (drops existing database)

Restore all databases from cluster backup (PostgreSQL only)../dbbackup restore single backup.dump \

  --target myapp_db \

```bash  --clean \

./dbbackup restore cluster <archive-file> [flags]  --verbose

``````



Restore flags:### Cluster Restore (PostgreSQL)

- `--confirm` - Execute restore (required, dry-run by default)

- `--dry-run` - Preview without executingRestore entire PostgreSQL cluster from archive:

- `--force` - Skip safety checks

- `--jobs` - Parallel decompression jobs (0 = auto)```bash

- `--verbose` - Detailed progress./dbbackup restore cluster ARCHIVE_FILE [OPTIONS]

- `--no-progress` - Disable progress indicators```



Example:**Options:**

```bash

./dbbackup restore cluster cluster_backup_20240101.tar.gz --confirm- `--confirm` - Confirm and execute restore (required for safety)

```- `--dry-run` - Show what would be done without executing

- `--force` - Skip safety checks

#### restore list - List Available Backups- `--jobs INT` - Parallel decompression jobs (default: auto)

- `--verbose` - Show detailed progress

```bash

./dbbackup restore list**Examples:**

```

```bash

Shows available backup archives in backup directory.# Standard cluster restore

sudo -u postgres ./dbbackup restore cluster cluster_backup.tar.gz --confirm

### System Commands

# Dry-run to preview

#### status - Connection Statussudo -u postgres ./dbbackup restore cluster cluster_backup.tar.gz --dry-run



```bash# High-performance restore

./dbbackup statussudo -u postgres ./dbbackup restore cluster cluster_backup.tar.gz \

```  --confirm \

  --jobs 16 \

Tests database connection and displays current configuration.  --verbose

```

#### preflight - Pre-flight Checks

**Safety Features:**

```bash

./dbbackup preflight- Archive integrity validation

```- Disk space checks (4x archive size recommended)

- Tool verification (psql, pg_restore, tar, gzip)

Runs checks before backup/restore operations:- Automatic database cleanup detection (interactive mode)

- Database connectivity- Progress tracking with ETA estimation

- Required tools availability

- Disk space## System Commands

- Permissions

### Status Check

#### list - List Databases

Check database connection and configuration:

```bash

./dbbackup list```bash

```./dbbackup status [OPTIONS]

```

Lists all available databases.

Shows: Database type, host, port, user, connection status, available databases.

#### cpu - CPU Information

### Preflight Checks

```bash

./dbbackup cpuRun pre-backup validation checks:

```

```bash

Shows CPU configuration and optimization recommendations../dbbackup preflight [OPTIONS]

```

#### verify - Verify Archive

Verifies: Database connection, required tools, disk space, permissions.

```bash

./dbbackup verify <archive-file>### List Databases

```

List available databases:

Validates backup archive integrity.

```bash

## Authentication./dbbackup list [OPTIONS]

```

### PostgreSQL Peer Authentication (Linux)

### CPU Information

Run as postgres user:

```bashDisplay CPU configuration and optimization settings:

sudo -u postgres ./dbbackup backup cluster

``````bash

./dbbackup cpu

### PostgreSQL Password Authentication```



Option 1 - .pgpass file (recommended):Shows: CPU count, model, workload recommendation, suggested parallel jobs.

```bash

echo "localhost:5432:*:postgres:password" > ~/.pgpass### Version

chmod 0600 ~/.pgpass

./dbbackup backup single mydb --user postgresDisplay version information:

```

```bash

Option 2 - Environment variable:./dbbackup version

```bash```

export PGPASSWORD=password

./dbbackup backup single mydb --user postgres## Configuration

```

### PostgreSQL Authentication

Option 3 - Command line:

```bashPostgreSQL uses different authentication methods based on system configuration.

./dbbackup backup single mydb --user postgres --password password

```#### Peer/Ident Authentication (Linux Default)



### MySQL/MariaDB AuthenticationRun as postgres system user:



Option 1 - .my.cnf file:```bash

```bashsudo -u postgres ./dbbackup backup cluster

cat > ~/.my.cnf << EOF```

[client]

user=backup_user#### Password Authentication

password=password

host=localhost**Option 1: .pgpass file (recommended for automation)**

EOF

chmod 0600 ~/.my.cnf```bash

./dbbackup backup single mydb --db-type mysqlecho "localhost:5432:*:postgres:password" > ~/.pgpass

```chmod 0600 ~/.pgpass

./dbbackup backup single mydb --user postgres

Option 2 - Environment variable:```

```bash

export MYSQL_PWD=password**Option 2: Environment variable**

./dbbackup backup single mydb --db-type mysql --user root

``````bash

export PGPASSWORD=your_password

Option 3 - Command line:./dbbackup backup single mydb --user postgres

```bash```

./dbbackup backup single mydb --db-type mysql --user root --password password

```**Option 3: Command line flag**



## Configuration```bash

./dbbackup backup single mydb --user postgres --password your_password

### Environment Variables```



```bash### MySQL/MariaDB Authentication

# Database connection

PG_HOST=localhost**Option 1: Command line**

PG_PORT=5432

PG_USER=postgres```bash

PGPASSWORD=password./dbbackup backup single mydb --db-type mysql --user root --password secret

MYSQL_HOST=localhost```

MYSQL_PORT=3306

MYSQL_USER=root**Option 2: Environment variable**

MYSQL_PWD=password

```bash

# Backup settingsexport MYSQL_PWD=your_password

BACKUP_DIR=/var/backups/databases./dbbackup backup single mydb --db-type mysql --user root

COMPRESS_LEVEL=6```

CLUSTER_TIMEOUT_MIN=240

```**Option 3: Configuration file**



### Database Types```bash

cat > ~/.my.cnf << EOF

- `postgres` - PostgreSQL[client]

- `mysql` - MySQLuser=backup_user

- `mariadb` - MariaDBpassword=your_password

host=localhost

Select via:EOF

- CLI: `-d postgres` or `--db-type postgres`chmod 0600 ~/.my.cnf

- Interactive: Arrow keys to cycle through options```



### Performance Options### Environment Variables



#### Parallelism```bash

# PostgreSQL

```bashexport PG_HOST=localhost

./dbbackup backup cluster --jobs 16 --dump-jobs 16export PG_PORT=5432

```export PG_USER=postgres

export PGPASSWORD=password

- `--jobs` - Compression/decompression parallel jobs

- `--dump-jobs` - Database dump parallel jobs# MySQL/MariaDB

- `--max-cores` - Limit CPU cores (default: 16)export MYSQL_HOST=localhost

export MYSQL_PORT=3306

#### CPU Workloadexport MYSQL_USER=root

export MYSQL_PWD=password

```bash

./dbbackup backup cluster --cpu-workload cpu-intensive# General

```export BACKUP_DIR=/var/backups/databases

export COMPRESS_LEVEL=6

Options: `cpu-intensive`, `io-intensive`, `balanced` (default)export CLUSTER_TIMEOUT_MIN=240

```

#### Compression

### SSL/TLS Configuration

```bash

./dbbackup backup single mydb --compression 9SSL modes: `disable`, `prefer`, `require`, `verify-ca`, `verify-full`

```

```bash

- Level 0 = No compression (fastest)# Disable SSL

- Level 6 = Balanced (default)./dbbackup backup single mydb --insecure

- Level 9 = Maximum compression (slowest)

# Require SSL

## Disaster Recovery./dbbackup backup single mydb --ssl-mode require



Complete automated disaster recovery test:# Verify certificate

./dbbackup backup single mydb --ssl-mode verify-full

```bash```

sudo ./disaster_recovery_test.sh

```## Performance



This script:### Memory Usage

1. Backs up entire cluster with maximum performance

2. Documents pre-backup stateStreaming architecture maintains constant memory usage:

3. Destroys all user databases (confirmation required)

4. Restores full cluster from backup| Database Size | Memory Usage |

5. Verifies restoration success|---------------|--------------|

| 1-10 GB      | ~800 MB      |

**Warning:** Destructive operation. Use only in test environments.| 10-50 GB     | ~900 MB      |

| 50-100 GB    | ~950 MB      |

## Troubleshooting| 100+ GB      | <1 GB        |



### Connection Issues### Large Database Optimization



Test connection:- Databases >5GB automatically use plain format with streaming compression

```bash- Parallel compression via pigz (if available)

./dbbackup status- Per-database timeout: 4 hours default

```- Automatic format selection based on size



PostgreSQL authentication error:### CPU Optimization

```bash

sudo -u postgres ./dbbackup statusAutomatically detects CPU configuration and optimizes parallelism:

```

```bash

SSL/TLS issues:./dbbackup cpu

```bash```

./dbbackup status --insecure

```Manual override:



### Out of Memory```bash

./dbbackup backup cluster \

Check swap:  --max-cores 32 \

```bash  --jobs 32 \

free -h  --cpu-workload cpu-intensive

dmesg | grep -i oom```

```

## Disaster Recovery

Add swap:

```bashUse the included script for complete disaster recovery testing:

sudo fallocate -l 16G /swapfile

sudo chmod 600 /swapfile```bash

sudo mkswap /swapfilesudo ./disaster_recovery_test.sh

sudo swapon /swapfile```

```

**Process:**

Reduce parallelism:

```bash1. Full cluster backup with maximum performance

./dbbackup backup cluster --jobs 4 --dump-jobs 42. Document current database state

```3. Drop all user databases (with confirmation)

4. Restore entire cluster from backup

### Debug Mode5. Verify database integrity and counts



Enable detailed logging:**Warning:** Destructive operation. Only use in test environments or genuine disaster recovery.

```bash

./dbbackup backup single mydb --debug## Troubleshooting

```

### Connection Issues

## Why dbbackup?

**Test connectivity:**

- **Reliable**: Comprehensive safety checks, validation, and error handling

- **Simple**: Intuitive menu-driven interface or straightforward CLI```bash

- **Fast**: Automatic CPU detection, parallel processing, streaming compression./dbbackup status

- **Efficient**: Minimal memory footprint, even for huge databases```

- **Flexible**: Multiple backup modes, compression levels, performance options

- **Safe**: Dry-run by default, archive verification, smart database cleanup**PostgreSQL peer authentication error:**

- **Complete**: Full cluster backup/restore, point-in-time recovery, multiple formats

```bash

dbbackup is production-ready for backup and disaster recovery operations on PostgreSQL, MySQL, and MariaDB databases of any size.sudo -u postgres ./dbbackup status

```

## License

**SSL/TLS issues:**

MIT

```bash

## Repository./dbbackup status --insecure

```

https://git.uuxo.net/uuxo/dbbackup

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
