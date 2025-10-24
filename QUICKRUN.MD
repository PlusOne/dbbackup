# DB Backup Tool - Quick Start Guide

## 🌟 NEW: Real-Time Progress Tracking!

The database backup tool now includes **enhanced progress tracking** with:
- � **Live progress bars** with percentage completion
- ⏱️ **Real-time timing** and performance metrics  
- 📝 **Detailed logging** with timestamps
- 🎨 **Beautiful interactive UI** with color indicators

## �🚀 Quick Installation

### Option 1: Pre-compiled Binary (Recommended)
```bash
# Linux AMD64
./bin/dbbackup_linux_amd64
chmod +x ./bin/dbbackup_linux_amd64

# Linux ARM64  
./bin/dbbackup_linux_arm64
chmod +x ./bin/dbbackup_linux_arm64

# macOS Intel
./bin/dbbackup_darwin_amd64
chmod +x ./bin/dbbackup_darwin_amd64

# macOS M1/M2
./bin/dbbackup_darwin_arm64  
chmod +x ./bin/dbbackup_darwin_arm64

# Windows
./bin/dbbackup_windows_amd64.exe
```

### Option 2: Build from Source
```bash
git clone https://github.com/your-repo/dbbackup.git
cd dbbackup
go build -o dbbackup .
```

## ⚡ Quick Start with Progress Tracking

### 🎨 Enhanced Interactive Mode (Best for Beginners)
```bash
# Start with real-time progress tracking
dbbackup interactive --database your_database

# Example with PostgreSQL
dbbackup interactive --database postgres --host localhost --user postgres
```

> Tip: In the interactive menu, tap the left/right arrows (or `t`) to toggle between PostgreSQL and MySQL/MariaDB before starting a task.

### 📊 Command Line with Progress
```bash
# Single backup with live progress
dbbackup backup single your_database --progress

# Cluster backup with detailed logging  
dbbackup backup cluster --progress --verbose --timestamps
```

## 🎬 Progress Tracking in Action

### Real-Time Progress Display
```bash
🔄 PostgreSQL Backup [67%] - Compressing archive...
    [███████████████▒▒▒▒▒▒▒] 
    ⏱️  Elapsed: 1m 45.2s | ETA: 42s
    📁 Files: 18/25 processed
    💾 Data: 1.4GB/2.1GB transferred
    
    Steps:
    ✅ Prepare backup directory
    ✅ Build backup command  
    ✅ Execute database backup
    🔄 Verify backup file
    ⏳ Create metadata file
    
    Details:
    database: postgres | type: single | compression: 6
    output_file: /backups/db_postgres_20241203_143527.dump
```

### Post-Operation Summary
```bash
✅ Single database backup completed: db_postgres_20241203_143527.dump

📊 Operation Summary:
  Total: 1 | Completed: 1 | Failed: 0 | Running: 0
  Total Duration: 2m 18.7s

📁 Backup Details:
  File: /backups/db_postgres_20241203_143527.dump
  Size: 1.1GB (compressed from 2.1GB)
  Verification: PASSED
  Metadata: Created successfully
```

### Command Line (For Scripts/Automation)

#### PostgreSQL Examples
```bash
# Single database backup (auto-optimized)
dbbackup backup single myapp_db --db-type postgres

# Sample backup (10% of data)
dbbackup backup sample myapp_db --sample-ratio 10 --db-type postgres

# Full cluster backup
dbbackup backup cluster --db-type postgres

# Check connection
dbbackup status --db-type postgres
```

#### MySQL Examples
```bash
# Single database backup
dbbackup backup single myapp_db --db-type mysql

# Using the short flag for database selection
dbbackup backup single myapp_db -d mysql

# Sample backup
dbbackup backup sample myapp_db --sample-ratio 10 --db-type mysql

# Check connection
dbbackup status --db-type mysql
```

## 🧠 CPU Optimization Commands

```bash
# Show CPU information and recommendations
dbbackup cpu

# Auto-optimize for your hardware
dbbackup backup single mydb --auto-detect-cores

# Manual configuration for big servers
dbbackup backup cluster --jobs 16 --dump-jobs 8 --max-cores 32
```

## 🔧 Common Options

| Option | Description | Example |
|--------|-------------|---------|
| `--host` | Database host | `--host db.example.com` |
| `--port` | Database port | `--port 5432` |
| `--user` | Database user | `--user backup_user` |
| `-d`, `--db-type` | Database type (`postgres`, `mysql`, `mariadb`) | `-d mysql` |
| `--insecure` | Disable SSL | `--insecure` |
| `--jobs` | Parallel jobs | `--jobs 8` |
| `--debug` | Debug mode | `--debug` |

### PostgreSQL Quick Options

- Default target is PostgreSQL; explicitly add `--db-type postgres` when switching between engines in scripts.
- Run as the `postgres` OS user for local clusters: `sudo -u postgres dbbackup ...` ensures socket authentication succeeds.
- Pick an SSL strategy: omit `--ssl-mode` for local sockets, use `--ssl-mode require` (or stricter) for remote TLS, or `--insecure` to disable TLS.
- Cluster backups (`backup cluster`) and plain `.dump` restores are PostgreSQL-only operations.

### MySQL / MariaDB Quick Options

- Select the engine with `--db-type mysql` (or `mariadb`); supply `--host`, `--port 3306`, `--user`, `--password`, and `--database` explicitly.
- Use `--insecure` to emit `--skip-ssl`; choose `--ssl-mode require|verify-ca|verify-identity` when TLS is mandatory.
- MySQL dumps are gzipped SQL scripts (`*.sql.gz`); the restore preview shows `gunzip -c ... | mysql` so you know the exact pipeline.
- Credentials can also come from `MYSQL_PWD` or option files if you prefer not to use the `--password` flag.

## 📁 Available Binaries

Choose the right binary for your platform:

- **Linux**: `dbbackup_linux_amd64`, `dbbackup_linux_arm64`
- **macOS**: `dbbackup_darwin_amd64`, `dbbackup_darwin_arm64`
- **Windows**: `dbbackup_windows_amd64.exe`
- **BSD**: `dbbackup_freebsd_amd64`, `dbbackup_openbsd_amd64`

## 🆘 Need Help?

```bash
# General help
dbbackup --help

# Command-specific help
dbbackup backup --help
dbbackup backup single --help

# Check CPU configuration
dbbackup cpu

# Test connection
dbbackup status --debug
```

For complete documentation, see `README.md`.