# dbbackup v2.1.0 Release Notes

**Release Date:** November 26, 2025  
**Git Tag:** v2.1.0  
**Commit:** 3a08b90

---

## 🎉 What's New in v2.1.0

### ☁️ Cloud Storage Integration (MAJOR FEATURE)

Complete native support for three major cloud providers:

#### **S3/MinIO/Backblaze B2**
- Native S3-compatible backend
- Streaming multipart uploads (>100MB files)
- Path-style and virtual-hosted-style addressing
- LocalStack/MinIO testing support

#### **Azure Blob Storage** 
- Native Azure SDK integration
- Block blob uploads with 100MB staging for large files
- Azurite emulator support for local testing
- SHA-256 metadata storage

#### **Google Cloud Storage**
- Native GCS SDK integration
- 16MB chunked uploads
- Application Default Credentials (ADC)
- fake-gcs-server support for testing

### 🎨 TUI Cloud Configuration

Configure cloud storage directly in interactive mode:
- **Settings Menu** → Cloud Storage section
- Toggle cloud storage on/off
- Select provider (S3, MinIO, B2, Azure, GCS)
- Configure bucket/container, region, credentials
- Enable auto-upload after backups
- Credential masking for security

### 🌐 Cross-Platform Support (10/10 Platforms)

All platforms now build successfully:
- ✅ Linux (x64, ARM64, ARMv7)
- ✅ macOS (Intel, Apple Silicon)
- ✅ Windows (x64, ARM64)
- ✅ FreeBSD (x64)
- ✅ OpenBSD (x64)
- ✅ NetBSD (x64)

**Fixed Issues:**
- Windows: syscall.Rlimit compatibility
- BSD: int64/uint64 type conversions
- OpenBSD: RLIMIT_AS unavailable
- NetBSD: syscall.Statfs API differences

---

## 📋 Complete Feature Set (v2.1.0)

### Database Support
- PostgreSQL (9.x - 16.x)
- MySQL (5.7, 8.x)
- MariaDB (10.x, 11.x)

### Backup Modes
- **Single Database** - Backup one database
- **Cluster Backup** - All databases (PostgreSQL only)
- **Sample Backup** - Reduced-size backups for testing

### Cloud Providers
- **S3** - Amazon S3 (`s3://bucket/path`)
- **MinIO** - Self-hosted S3-compatible (`s3://bucket/path` + endpoint)
- **Backblaze B2** - B2 Cloud Storage (`s3://bucket/path` + endpoint)
- **Azure Blob Storage** - Microsoft Azure (`azure://container/path`)
- **Google Cloud Storage** - Google Cloud (`gcs://bucket/path`)

### Core Features
- ✅ Streaming compression (constant memory usage)
- ✅ Parallel processing (auto CPU detection)
- ✅ SHA-256 verification
- ✅ JSON metadata (.info files)
- ✅ Retention policies (cleanup old backups)
- ✅ Interactive TUI with progress tracking
- ✅ Configuration persistence (.dbbackup.conf)
- ✅ Cloud auto-upload
- ✅ Multipart uploads (>100MB)
- ✅ Progress tracking with ETA

---

## 🚀 Quick Start Examples

### Basic Cloud Backup

```bash
# Configure via TUI
./dbbackup interactive
# Navigate to: Configuration Settings
# Enable: Cloud Storage = true
# Set: Cloud Provider = s3
# Set: Cloud Bucket = my-backups
# Set: Cloud Auto-Upload = true

# Backup will now auto-upload to S3
./dbbackup backup single mydb
```

### Command-Line Cloud Backup

```bash
# S3
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"
./dbbackup backup single mydb --cloud s3://my-bucket/backups/

# Azure
export AZURE_STORAGE_ACCOUNT="myaccount"
export AZURE_STORAGE_KEY="key"
./dbbackup backup single mydb --cloud azure://my-container/backups/

# GCS (with service account)
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
./dbbackup backup single mydb --cloud gcs://my-bucket/backups/
```

### Cloud Restore

```bash
# Restore from S3
./dbbackup restore single s3://my-bucket/backups/mydb_20250126.tar.gz

# Restore from Azure
./dbbackup restore single azure://my-container/backups/mydb_20250126.tar.gz

# Restore from GCS
./dbbackup restore single gcs://my-bucket/backups/mydb_20250126.tar.gz
```

---

## 📦 Installation

### Pre-compiled Binaries

```bash
# Linux x64
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_linux_amd64 -o dbbackup
chmod +x dbbackup

# macOS Intel
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_amd64 -o dbbackup
chmod +x dbbackup

# macOS Apple Silicon
curl -L https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_darwin_arm64 -o dbbackup
chmod +x dbbackup

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://git.uuxo.net/uuxo/dbbackup/raw/branch/main/bin/dbbackup_windows_amd64.exe" -OutFile "dbbackup.exe"
```

### Docker

```bash
docker pull git.uuxo.net/uuxo/dbbackup:latest

# With cloud credentials
docker run --rm \
  -e AWS_ACCESS_KEY_ID="key" \
  -e AWS_SECRET_ACCESS_KEY="secret" \
  -e PGHOST=postgres \
  -e PGUSER=postgres \
  -e PGPASSWORD=secret \
  git.uuxo.net/uuxo/dbbackup:latest \
  backup single mydb --cloud s3://bucket/backups/
```

---

## 🧪 Testing Cloud Storage

### Local Testing with Emulators

```bash
# MinIO (S3-compatible)
docker compose -f docker-compose.minio.yml up -d
./scripts/test_cloud_storage.sh

# Azure (Azurite)
docker compose -f docker-compose.azurite.yml up -d
./scripts/test_azure_storage.sh

# GCS (fake-gcs-server)
docker compose -f docker-compose.gcs.yml up -d
./scripts/test_gcs_storage.sh
```

---

## 📚 Documentation

- [README.md](README.md) - Main documentation
- [CLOUD.md](CLOUD.md) - Complete cloud storage guide
- [CHANGELOG.md](CHANGELOG.md) - Version history
- [DOCKER.md](DOCKER.md) - Docker usage guide
- [AZURE.md](AZURE.md) - Azure-specific guide
- [GCS.md](GCS.md) - GCS-specific guide

---

## 🔄 Upgrade from v2.0

v2.1.0 is **fully backward compatible** with v2.0. Existing backups and configurations work without changes.

**New in v2.1:**
- Cloud storage configuration in TUI
- Auto-upload functionality
- Cross-platform Windows/NetBSD support

**Migration steps:**
1. Update binary: Download latest from `bin/` directory
2. (Optional) Enable cloud: `./dbbackup interactive` → Settings → Cloud Storage
3. (Optional) Configure provider, bucket, credentials
4. Existing local backups remain unchanged

---

## 🐛 Known Issues

None at this time. All 10 platforms building successfully.

**Report issues:** https://git.uuxo.net/uuxo/dbbackup/issues

---

## 🗺️ Roadmap - What's Next?

### v2.2 - Incremental Backups (Planned)
- File-level incremental for PostgreSQL
- Binary log incremental for MySQL
- Differential backup support

### v2.3 - Encryption (Planned)
- AES-256 at-rest encryption
- Encrypted cloud uploads
- Key management

### v2.4 - PITR (Planned)
- WAL archiving (PostgreSQL)
- Binary log archiving (MySQL)
- Restore to specific timestamp

### v2.5 - Enterprise Features (Planned)
- Prometheus metrics
- Remote restore
- Replication slot management

---

## 👥 Contributors

- uuxo (maintainer)

---

## 📄 License

See LICENSE file in repository.

---

**Full Changelog:** https://git.uuxo.net/uuxo/dbbackup/src/branch/main/CHANGELOG.md
