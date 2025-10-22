# DB Backup Tool - Advanced Database Backup Solution

A comprehensive, high-performance database backup and restore solution with **multi-database support** (PostgreSQL & MySQL), **intelligent CPU optimization**, **real-time progress tracking**, and **beautiful interactive UI**.

## 🌟 NEW: Enhanced Progress Tracking & Logging

### 📊 **Real-Time Progress Monitoring**
- **Live Progress Bars**: Visual progress indicators with percentage completion
- **Step-by-Step Tracking**: Detailed breakdown of each operation phase
- **Time Estimates**: Elapsed time and estimated completion times
- **Data Transfer Metrics**: Real-time file counts and byte transfer statistics

### 📝 **Comprehensive Logging System**
- **Timestamped Entries**: All operations logged with precise timestamps
- **Structured Metadata**: Rich operation context and performance metrics
- **Error Aggregation**: Detailed error reporting with troubleshooting information
- **Operation History**: Complete audit trail of all backup/restore activities

### 🎨 **Enhanced Interactive Experience**
- **Animated UI**: Spinners, progress bars, and color-coded status indicators
- **Operation Dashboard**: Real-time monitoring of active and completed operations
- **Status Summaries**: Post-operation reports with duration and file statistics
- **History Viewer**: Browse past operations with detailed metrics

### 💡 **Smart Progress Features**
```bash
# Live backup progress example
🔄 PostgreSQL Backup [78%] - Compressing archive...
    [███████████████▒▒▒▒▒] 
    ⏱️  Elapsed: 2m 45.3s | Files: 24/30 | Data: 1.8GB/2.3GB
    📝 Current: Creating metadata file
    
✅ Backup completed in 3m 12.7s
📁 Output: /backups/db_postgres_20241203_143527.dump (1.1GB)
🔍 Verification: PASSED
```

## 🚀 Key Features

### ✨ **Core Functionality**
- **Multi-Database Support**: PostgreSQL and MySQL with unified interface
- **Multiple Backup Modes**: Single database, sample backups, full cluster backups
- **Cross-Platform**: Pre-compiled binaries for Linux, macOS, Windows, and BSD systems
- **Interactive TUI**: Beautiful terminal interface with real-time progress indicators

### 🧠 **Intelligent CPU Optimization**
- **Automatic CPU Detection**: Detects physical and logical cores across platforms
- **Workload-Aware Scaling**: Optimizes parallelism based on workload type
- **Big Server Support**: Configurable CPU limits for high-core systems
- **Performance Tuning**: Separate optimization for backup and restore operations

### 🔧 **Advanced Configuration**
- **SSL/TLS Support**: Full SSL configuration with multiple modes
- **Compression**: Configurable compression levels (0-9)
- **Environment Integration**: Environment variable and CLI flag support
- **Flexible Paths**: Configurable backup directories and naming

## 📦 Installation

### Pre-compiled Binaries (Recommended)

Download the appropriate binary for your platform from the `bin/` directory:

```bash
# Linux (Intel/AMD)
wget https://github.com/your-repo/dbbackup/releases/latest/download/dbbackup_linux_amd64
chmod +x dbbackup_linux_amd64
sudo mv dbbackup_linux_amd64 /usr/local/bin/dbbackup

# macOS (Intel)
wget https://github.com/your-repo/dbbackup/releases/latest/download/dbbackup_darwin_amd64
chmod +x dbbackup_darwin_amd64
sudo mv dbbackup_darwin_amd64 /usr/local/bin/dbbackup

# macOS (Apple Silicon)
wget https://github.com/your-repo/dbbackup/releases/latest/download/dbbackup_darwin_arm64
chmod +x dbbackup_darwin_arm64
sudo mv dbbackup_darwin_arm64 /usr/local/bin/dbbackup

# Windows (download and rename)
# Download dbbackup_windows_amd64.exe and use directly
```

### Build from Source

```bash
git clone https://github.com/your-repo/dbbackup.git
cd dbbackup
go build -o dbbackup .
```

### Cross-Platform Build

```bash
# Build for all platforms
./build_all.sh
```

## 🖥️ Usage Examples

### Interactive Mode with Progress Tracking (Recommended)

```bash
# Start enhanced interactive interface with real-time progress
dbbackup interactive --database your_database

# Interactive mode with progress monitoring
dbbackup menu --database postgres --host localhost --user postgres

# Alternative UI command
dbbackup ui --database myapp_db --progress
```

### Enhanced Progress Tracking Commands

#### Real-Time Progress Monitoring

```bash
# Single backup with detailed progress tracking
dbbackup backup single myapp_db --progress --verbose

# Sample backup with progress indicators
dbbackup backup sample myapp_db --sample-ratio 10 --progress

# Cluster backup with comprehensive logging
dbbackup backup cluster --progress --detailed --timestamps
```

#### Operation Status & History

```bash
# View current operation status
dbbackup status --detailed

# Show operation history with metrics
dbbackup status --history --performance

# Monitor active operations
dbbackup status --active --refresh-interval 2s
```

#### Progress Feature Examples

```bash
# Backup with file-by-file progress
dbbackup backup single large_db --progress --show-files

# Backup with byte-level transfer tracking
dbbackup backup cluster --progress --show-bytes --compression 9

# Restore with step-by-step progress
dbbackup restore backup.dump --progress --verify --show-steps
```

### Command Line Interface

#### Basic Backup Operations

```bash
# Single database backup (auto-optimized for your CPU)
dbbackup backup single myapp_db --db-type postgres

# Sample backup (10% of data)
dbbackup backup sample myapp_db --sample-ratio 10

# Full cluster backup (PostgreSQL only)
dbbackup backup cluster --db-type postgres
```

#### CPU-Optimized Operations

```bash
# Auto-detect and optimize for your hardware
dbbackup backup single myapp_db --auto-detect-cores

# Manual CPU configuration for big servers
dbbackup backup cluster --jobs 16 --dump-jobs 8 --max-cores 32

# Set workload type for optimal performance
dbbackup backup single myapp_db --cpu-workload io-intensive

# Show CPU information and recommendations
dbbackup cpu
```

#### Database Connectivity

```bash
# PostgreSQL with SSL
dbbackup backup single mydb \
  --host db.example.com \
  --port 5432 \
  --user backup_user \
  --ssl-mode require

# MySQL with compression
dbbackup backup single mydb \
  --db-type mysql \
  --host mysql.example.com \
  --port 3306 \
  --compression 9

# Local PostgreSQL (socket connection)
sudo -u postgres dbbackup backup cluster --insecure
```

#### System Diagnostics

```bash
# Check connection status
dbbackup status

# Run preflight checks
dbbackup preflight

# List databases and archives
dbbackup list

# Show CPU optimization settings
dbbackup cpu
```

## ⚙️ Configuration

### CPU Optimization Settings

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `--auto-detect-cores` | Enable automatic CPU detection | `true` | `--auto-detect-cores` |
| `--jobs` | Parallel restore jobs | Auto-detected | `--jobs 8` |
| `--dump-jobs` | Parallel backup jobs | Auto-detected | `--dump-jobs 4` |
| `--max-cores` | Maximum cores to use | Auto-detected | `--max-cores 16` |
| `--cpu-workload` | Workload type | `balanced` | `--cpu-workload cpu-intensive` |

#### Workload Types:
- **`balanced`**: Uses logical cores (default)
- **`cpu-intensive`**: Uses physical cores (best for dumps)
- **`io-intensive`**: Uses 2x logical cores (best for restores)

### Database Configuration

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `--host` | Database host | `localhost` | `--host db.example.com` |
| `--port` | Database port | `5432` (PG), `3306` (MySQL) | `--port 5432` |
| `--user` | Database user | `postgres` (PG), `root` (MySQL) | `--user backup_user` |
| `--database` | Database name | `postgres` | `--database myapp_db` |
| `--db-type` | Database type | `postgres` | `--db-type mysql` |
| `--ssl-mode` | SSL mode | `prefer` | `--ssl-mode require` |
| `--insecure` | Disable SSL | `false` | `--insecure` |

### Environment Variables

```bash
# Database connection
export PG_HOST=localhost
export PG_PORT=5432
export PG_USER=postgres
export PGPASSWORD=secret
export DB_TYPE=postgres

# CPU optimization
export AUTO_DETECT_CORES=true
export CPU_WORKLOAD_TYPE=balanced
export MAX_CORES=16

# Backup settings
export BACKUP_DIR=/var/backups
export COMPRESS_LEVEL=6
```

## 🏗️ Architecture

### Package Structure
```
dbbackup/
├── cmd/                    # CLI commands (Cobra framework)
├── internal/
│   ├── config/            # Configuration management
│   ├── database/          # Database abstraction layer
│   ├── backup/            # Backup engine
│   ├── cpu/               # CPU detection and optimization
│   ├── logger/            # Structured logging
│   ├── progress/          # Progress indicators
│   └── tui/               # Terminal user interface
└── bin/                   # Cross-platform binaries
```

### Supported Platforms

| Platform | Architecture | Binary Name |
|----------|-------------|-------------|
| **Linux** | amd64 | `dbbackup_linux_amd64` |
| **Linux** | arm64 | `dbbackup_linux_arm64` |
| **Linux** | armv7 | `dbbackup_linux_arm_armv7` |
| **macOS** | amd64 (Intel) | `dbbackup_darwin_amd64` |
| **macOS** | arm64 (Apple Silicon) | `dbbackup_darwin_arm64` |
| **Windows** | amd64 | `dbbackup_windows_amd64.exe` |
| **Windows** | arm64 | `dbbackup_windows_arm64.exe` |
| **FreeBSD** | amd64 | `dbbackup_freebsd_amd64` |
| **OpenBSD** | amd64 | `dbbackup_openbsd_amd64` |
| **NetBSD** | amd64 | `dbbackup_netbsd_amd64` |

## 🚀 Performance Optimization

### Automatic CPU Detection

The tool automatically detects your system's CPU configuration and optimizes job counts:

```bash
# View detected CPU information
dbbackup cpu

# Example output:
# Architecture: amd64
# Logical Cores: 16
# Physical Cores: 8
# Model: Intel Xeon CPU E5-2667 v4
# Recommended jobs (balanced): 16
```

### Big Server Optimization

For high-core systems with large databases:

```bash
# Example: 32-core server with large database
dbbackup backup cluster \
  --jobs 24 \
  --dump-jobs 12 \
  --max-cores 32 \
  --cpu-workload cpu-intensive \
  --compression 9
```

### Memory Considerations

- **Small databases** (< 1GB): Use default settings
- **Medium databases** (1-10GB): Increase jobs to logical cores
- **Large databases** (> 10GB): Use physical cores for dumps, logical cores for restores
- **Very large databases** (> 100GB): Consider I/O-intensive workload type

## 🔍 Troubleshooting

### Common Issues

#### CPU Detection Issues
```bash
# If auto-detection fails, manually set values
dbbackup backup single mydb --auto-detect-cores=false --jobs 4 --dump-jobs 2
```

#### Connection Issues
```bash
# Test connection
dbbackup status --debug

# Common fixes
dbbackup status --insecure                    # Disable SSL
dbbackup status --ssl-mode=disable           # Explicit SSL disable
sudo -u postgres dbbackup status             # Use postgres user (Linux)
```

#### Performance Issues
```bash
# Check CPU optimization
dbbackup cpu

# Try different workload types
dbbackup backup single mydb --cpu-workload io-intensive
```

### Debug Mode

```bash
# Enable detailed logging
dbbackup backup single mydb --debug
```

## 📋 Comparison with Original Bash Script

| Feature | Bash Script | Go Implementation |
|---------|-------------|-------------------|
| **Database Support** | PostgreSQL only | PostgreSQL + MySQL |
| **CPU Detection** | Basic | Advanced multi-platform |
| **User Interface** | Text-based | Beautiful interactive TUI |
| **Error Handling** | Shell-based | Type-safe, comprehensive |
| **Performance** | Shell overhead | Native binary speed |
| **Cross-Platform** | Linux only | 10+ platforms |
| **Dependencies** | Many external tools | Self-contained binary |
| **Maintainability** | Monolithic script | Modular packages |

## 📄 License

Released under MIT License. See LICENSE file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 🆘 Support

- **Issues**: Report bugs and feature requests via GitHub Issues
- **Documentation**: Check the `bin/README.md` for binary-specific information
- **Examples**: See the `examples/` directory for more usage examples

---

**Built with ❤️ using Go** - High-performance, type-safe, and cross-platform database backup solution.