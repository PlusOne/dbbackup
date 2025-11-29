# DB Backup Tool - Pre-compiled Binaries

This directory contains pre-compiled binaries for the DB Backup Tool across multiple platforms and architectures.

## Build Information
- **Version**: 3.0.0
- **Build Time**: 2025-11-29_10:43:00_UTC
- **Git Commit**: 0d8e851

## Recent Updates (v1.1.0)
- ✅ Fixed TUI progress display with line-by-line output
- ✅ Added interactive configuration settings menu
- ✅ Improved menu navigation and responsiveness  
- ✅ Enhanced completion status handling
- ✅ Better CPU detection and optimization
- ✅ Silent mode support for TUI operations

## Available Binaries

### Linux
- `dbbackup_linux_amd64` - Linux 64-bit (Intel/AMD)
- `dbbackup_linux_arm64` - Linux 64-bit (ARM)
- `dbbackup_linux_arm_armv7` - Linux 32-bit (ARMv7)

### macOS
- `dbbackup_darwin_amd64` - macOS 64-bit (Intel)
- `dbbackup_darwin_arm64` - macOS 64-bit (Apple Silicon)

### Windows
- `dbbackup_windows_amd64.exe` - Windows 64-bit (Intel/AMD)
- `dbbackup_windows_arm64.exe` - Windows 64-bit (ARM)

### BSD Systems
- `dbbackup_freebsd_amd64` - FreeBSD 64-bit
- `dbbackup_openbsd_amd64` - OpenBSD 64-bit
- `dbbackup_netbsd_amd64` - NetBSD 64-bit

## Usage

1. Download the appropriate binary for your platform
2. Make it executable (Unix-like systems): `chmod +x dbbackup_*`
3. Run: `./dbbackup_* --help`

## Interactive Mode

Launch the interactive TUI menu for easy configuration and operation:

```bash
# Interactive mode with TUI menu
./dbbackup_linux_amd64

# Features:
# - Interactive configuration settings
# - Real-time progress display
# - Operation history and status
# - CPU detection and optimization
```

## Command Line Mode

Direct command line usage with line-by-line progress:

```bash
# Show CPU information and optimization settings
./dbbackup_linux_amd64 cpu

# Auto-optimize for your hardware
./dbbackup_linux_amd64 backup cluster --auto-detect-cores

# Manual CPU configuration  
./dbbackup_linux_amd64 backup single mydb --jobs 8 --dump-jobs 4

# Line-by-line progress output
./dbbackup_linux_amd64 backup cluster --progress-type line
```

## CPU Detection

All binaries include advanced CPU detection capabilities:
- Automatic core detection for optimal parallelism
- Support for different workload types (CPU-intensive, I/O-intensive, balanced)
- Platform-specific optimizations for Linux, macOS, and Windows
- Interactive CPU configuration in TUI mode

## Support

For issues or questions, please refer to the main project documentation.
