# dbbackup

![dbbackup](dbbackup.png)

Database backup utility for PostgreSQL and MySQL with support for large databases.

## Recent Changes

### Large Database Handling

- Streaming compression reduces memory usage by ~90%
- Native pgx v5 driver reduces memory by ~48% compared to lib/pq
- Automatic format selection based on database size
- Per-database timeout configuration (default: 240 minutes)
- Parallel compression support via pigz when available

### Memory Usage

| Database Size | Memory Usage |
|---------------|--------------|
| 10GB | ~850MB |
| 25GB | ~920MB |
| 50GB | ~940MB |
| 100GB+ | <1GB |

### Progress Tracking

- Real-time progress indicators
- Step-by-step operation tracking
- Structured logging with timestamps
- Operation history

## Features

- PostgreSQL and MySQL support
- Single database, sample, and cluster backup modes
- CPU detection and parallel job optimization
- Interactive terminal interface
- Cross-platform binaries (Linux, macOS, Windows, BSD)
- SSL/TLS support
- Configurable compression levels

## Installation

### Pre-compiled Binaries

Download the binary for your platform:

```bash
# Linux (Intel/AMD)
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup
chmod +x dbbackup

# macOS (Intel)
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup
chmod +x dbbackup

# macOS (Apple Silicon)
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup
chmod +x dbbackup
```

### Build from Source

```bash
git clone https://git.uuxo.net/uuxo/dbbackup.git
cd dbbackup
go build -o dbbackup main.go
```

## Usage

### Interactive Mode

```bash
dbbackup interactive
```

Interactive mode provides menu navigation with arrow keys and automatic status updates.

### Command Line

```bash
# Single database backup
dbbackup backup single myapp_db

# Sample backup (10% of data)
dbbackup backup sample myapp_db --sample-ratio 10

# Full cluster backup (PostgreSQL)
dbbackup backup cluster

# With custom settings
dbbackup backup single myapp_db \
  --host db.example.com \
  --port 5432 \
  --user backup_user \
  --ssl-mode require
```

### System Commands

```bash
# Check connection status
dbbackup status

# Run preflight checks
dbbackup preflight

# List databases and backups
dbbackup list

# Show CPU information
dbbackup cpu
```

## Configuration

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--host` | Database host | `localhost` |
| `--port` | Database port | `5432` (PostgreSQL), `3306` (MySQL) |
| `--user` | Database user | `postgres` |
| `--database` | Database name | `postgres` |
| `-d`, `--db-type` | Database type | `postgres` |
| `--ssl-mode` | SSL mode | `prefer` |
| `--jobs` | Parallel jobs | Auto-detected |
| `--dump-jobs` | Parallel dump jobs | Auto-detected |
| `--compression` | Compression level (0-9) | `6` |
| `--backup-dir` | Backup directory | `/var/lib/pgsql/db_backups` |

### PostgreSQL

Use the `postgres` system user for local connections:
```bash
sudo -u postgres dbbackup backup cluster
```

SSL modes: `disable`, `prefer`, `require`, `verify-ca`, `verify-full`

Cluster operations (backup/restore/verify) are PostgreSQL-only.

### MySQL / MariaDB

Set `--db-type mysql` or `--db-type mariadb`:
```bash
dbbackup backup single mydb \
  --db-type mysql \
  --host 127.0.0.1 \
  --user backup_user \
  --password ****
```

MySQL backups are created as `.sql.gz` files.

### Environment Variables

```bash
# Database
export PG_HOST=localhost
export PG_PORT=5432
export PG_USER=postgres
export PGPASSWORD=secret
export MYSQL_HOST=localhost
export MYSQL_PWD=secret

# Backup
export BACKUP_DIR=/var/backups
export COMPRESS_LEVEL=6
export CLUSTER_TIMEOUT_MIN=240  # Cluster timeout in minutes

# Swap file management (Linux + root only)
export AUTO_SWAP=false
export SWAP_FILE_SIZE_GB=8
export SWAP_FILE_PATH=/tmp/dbbackup_swap
```

## Architecture

```
dbbackup/
├── cmd/                    # CLI commands
├── internal/
│   ├── config/            # Configuration
│   ├── database/          # Database drivers
│   ├── backup/            # Backup engine
│   ├── cpu/               # CPU detection
│   ├── logger/            # Logging
│   ├── progress/          # Progress indicators
│   └── tui/               # Terminal UI
└── bin/                   # Binaries
```

### Supported Platforms

Linux (amd64, arm64, armv7), macOS (amd64, arm64), Windows (amd64, arm64), FreeBSD, OpenBSD, NetBSD

## Performance

### CPU Detection

The tool detects CPU configuration and adjusts parallelism automatically:

```bash
dbbackup cpu
```

### Large Database Handling

Streaming architecture maintains constant memory usage regardless of database size. Databases >5GB automatically use plain format. Parallel compression via pigz is used when available.

### Memory Usage Notes

- Small databases (<1GB): ~500MB
- Medium databases (1-10GB): ~800MB
- Large databases (10-50GB): ~900MB  
- Huge databases (50GB+): ~1GB

## Troubleshooting

### Connection Issues

```bash
# Test connection
dbbackup status

# Disable SSL
dbbackup status --insecure

# Use postgres user (Linux)
sudo -u postgres dbbackup status
```

### Out of Memory Issues

Check kernel logs for OOM events:
```bash
dmesg | grep -i oom
free -h
```

Enable swap file management (Linux + root):
```bash
export AUTO_SWAP=true
export SWAP_FILE_SIZE_GB=8
sudo dbbackup backup cluster
```

Or manually add swap:
```bash
sudo fallocate -l 8G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

### Debug Mode

```bash
dbbackup backup single mydb --debug
```

## Documentation

- [HUGE_DATABASE_QUICK_START.md](HUGE_DATABASE_QUICK_START.md) - Quick start for large databases
- [LARGE_DATABASE_OPTIMIZATION_PLAN.md](LARGE_DATABASE_OPTIMIZATION_PLAN.md) - Optimization details
- [PRIORITY2_PGX_INTEGRATION.md](PRIORITY2_PGX_INTEGRATION.md) - pgx v5 integration

## License

MIT License

## Repository

https://git.uuxo.net/uuxo/dbbackup