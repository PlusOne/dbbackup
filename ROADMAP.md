# dbbackup Version 2.0 Roadmap

## Current Status: v1.1 (Production Ready)
- ✅ 24/24 automated tests passing (100%)
- ✅ PostgreSQL, MySQL, MariaDB support
- ✅ Interactive TUI + CLI
- ✅ Cluster backup/restore
- ✅ Docker support
- ✅ Cross-platform binaries

---

## Version 2.0 Vision: Enterprise-Grade Features

Transform dbbackup into an enterprise-ready backup solution with cloud storage, incremental backups, PITR, and encryption.

**Target Release:** Q2 2026 (3-4 months)

---

## Priority Matrix

```
                    HIGH IMPACT
                         │
    ┌────────────────────┼────────────────────┐
    │                    │                    │
    │  Cloud Storage ⭐  │ Incremental ⭐⭐⭐ │
    │  Verification      │ PITR ⭐⭐⭐       │
    │  Retention         │ Encryption ⭐⭐   │
LOW │                    │                    │ HIGH
EFFORT ─────────────────┼──────────────────── EFFORT
    │                    │                    │
    │  Metrics           │ Web UI (optional)  │
    │  Remote Restore    │ Replication Slots  │
    │                    │                    │
    └────────────────────┼────────────────────┘
                         │
                    LOW IMPACT
```

---

## Development Phases

### Phase 1: Foundation (Weeks 1-4)

**Sprint 1: Verification & Retention (2 weeks)**

**Goals:**
- Backup integrity verification with SHA-256 checksums
- Automated retention policy enforcement
- Structured backup metadata

**Features:**
- ✅ Generate SHA-256 checksums during backup
- ✅ Verify backups before/after restore
- ✅ Automatic cleanup of old backups
- ✅ Retention policy: days + minimum count
- ✅ Backup metadata in JSON format

**Deliverables:**
```bash
# New commands
dbbackup verify backup.dump
dbbackup cleanup --retention-days 30 --min-backups 5

# Metadata format
{
  "version": "2.0",
  "timestamp": "2026-01-15T10:30:00Z",
  "database": "production",
  "size_bytes": 1073741824,
  "sha256": "abc123...",
  "db_version": "PostgreSQL 15.3",
  "compression": "gzip-9"
}
```

**Implementation:**
- `internal/verification/` - Checksum calculation and validation
- `internal/retention/` - Policy enforcement
- `internal/metadata/` - Backup metadata management

---

**Sprint 2: Cloud Storage (2 weeks)**

**Goals:**
- Upload backups to cloud storage
- Support multiple cloud providers
- Download and restore from cloud

**Providers:**
- ✅ AWS S3
- ✅ MinIO (S3-compatible)
- ✅ Backblaze B2
- ✅ Azure Blob Storage (optional)
- ✅ Google Cloud Storage (optional)

**Configuration:**
```toml
[cloud]
enabled = true
provider = "s3"  # s3, minio, azure, gcs, b2
auto_upload = true

[cloud.s3]
bucket = "db-backups"
region = "us-east-1"
endpoint = "s3.amazonaws.com"  # Custom for MinIO
access_key = "..."  # Or use IAM role
secret_key = "..."
```

**New Commands:**
```bash
# Upload existing backup
dbbackup cloud upload backup.dump

# List cloud backups
dbbackup cloud list

# Download from cloud
dbbackup cloud download backup_id

# Restore directly from cloud
dbbackup restore single s3://bucket/backup.dump --target mydb
```

**Dependencies:**
```go
"github.com/aws/aws-sdk-go-v2/service/s3"
"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
"cloud.google.com/go/storage"
```

---

### Phase 2: Advanced Backup (Weeks 5-10)

**Sprint 3: Incremental Backups (3 weeks)**

**Goals:**
- Reduce backup time and storage
- File-level incremental for PostgreSQL
- Binary log incremental for MySQL

**PostgreSQL Strategy:**
```
Full Backup (Base)
    ├─ Incremental 1 (changed files since base)
    ├─ Incremental 2 (changed files since inc1)
    └─ Incremental 3 (changed files since inc2)
```

**MySQL Strategy:**
```
Full Backup
    ├─ Binary Log 1 (changes since full)
    ├─ Binary Log 2
    └─ Binary Log 3
```

**Implementation:**
```bash
# Create base backup
dbbackup backup single mydb --mode full

# Create incremental
dbbackup backup single mydb --mode incremental

# Restore (automatically applies incrementals)
dbbackup restore single backup.dump --apply-incrementals
```

**File Structure:**
```
backups/
├── mydb_full_20260115.dump
├── mydb_full_20260115.meta
├── mydb_incr_20260116.dump      # Contains only changes
├── mydb_incr_20260116.meta      # Points to base: mydb_full_20260115
└── mydb_incr_20260117.dump
```

---

**Sprint 4: Security & Encryption (2 weeks)**

**Goals:**
- Encrypt backups at rest
- Secure key management
- Encrypted cloud uploads

**Features:**
- ✅ AES-256-GCM encryption
- ✅ Argon2 key derivation
- ✅ Multiple key sources (file, env, vault)
- ✅ Encrypted metadata

**Configuration:**
```toml
[encryption]
enabled = true
algorithm = "aes-256-gcm"
key_file = "/etc/dbbackup/encryption.key"

# Or use environment variable
# DBBACKUP_ENCRYPTION_KEY=base64key...
```

**Commands:**
```bash
# Generate encryption key
dbbackup keys generate

# Encrypt existing backup
dbbackup encrypt backup.dump

# Decrypt backup
dbbackup decrypt backup.dump.enc

# Automatic encryption
dbbackup backup single mydb --encrypt
```

**File Format:**
```
+------------------+
| Encryption Header|  (IV, algorithm, key ID)
+------------------+
| Encrypted Data   |  (AES-256-GCM)
+------------------+
| Auth Tag         |  (HMAC for integrity)
+------------------+
```

---

**Sprint 5: Point-in-Time Recovery - PITR (4 weeks)**

**Goals:**
- Restore to any point in time
- WAL archiving for PostgreSQL
- Binary log archiving for MySQL

**PostgreSQL Implementation:**

```toml
[pitr]
enabled = true
wal_archive_dir = "/backups/wal_archive"
wal_retention_days = 7

# PostgreSQL config (auto-configured by dbbackup)
# archive_mode = on
# archive_command = '/usr/local/bin/dbbackup archive-wal %p %f'
```

**Commands:**
```bash
# Enable PITR
dbbackup pitr enable

# Archive WAL manually
dbbackup archive-wal /var/lib/postgresql/pg_wal/000000010000000000000001

# Restore to point-in-time
dbbackup restore single backup.dump \
  --target-time "2026-01-15 14:30:00" \
  --target mydb

# Show available restore points
dbbackup pitr timeline
```

**WAL Archive Structure:**
```
wal_archive/
├── 000000010000000000000001
├── 000000010000000000000002
├── 000000010000000000000003
└── timeline.json
```

**MySQL Implementation:**
```bash
# Archive binary logs
dbbackup binlog archive --start-datetime "2026-01-15 00:00:00"

# PITR restore
dbbackup restore single backup.sql \
  --target-time "2026-01-15 14:30:00" \
  --apply-binlogs
```

---

### Phase 3: Enterprise Features (Weeks 11-16)

**Sprint 6: Observability & Integration (3 weeks)**

**Features:**

1. **Prometheus Metrics**
```go
# Exposed metrics
dbbackup_backup_duration_seconds
dbbackup_backup_size_bytes
dbbackup_backup_success_total
dbbackup_restore_duration_seconds
dbbackup_last_backup_timestamp
dbbackup_cloud_upload_duration_seconds
```

**Endpoint:**
```bash
# Start metrics server
dbbackup metrics serve --port 9090

# Scrape endpoint
curl http://localhost:9090/metrics
```

2. **Remote Restore**
```bash
# Restore to remote server
dbbackup restore single backup.dump \
  --remote-host db-replica-01 \
  --remote-user postgres \
  --remote-port 22 \
  --confirm
```

3. **Replication Slots (PostgreSQL)**
```bash
# Create replication slot for continuous WAL streaming
dbbackup replication create-slot backup_slot

# Stream WALs via replication
dbbackup replication stream backup_slot
```

4. **Webhook Notifications**
```toml
[notifications]
enabled = true
webhook_url = "https://slack.com/webhook/..."
notify_on = ["backup_complete", "backup_failed", "restore_complete"]
```

---

## Technical Architecture

### New Directory Structure

```
internal/
├── cloud/              # Cloud storage backends
│   ├── interface.go
│   ├── s3.go
│   ├── azure.go
│   └── gcs.go
├── encryption/         # Encryption layer
│   ├── aes.go
│   ├── keys.go
│   └── vault.go
├── incremental/        # Incremental backup engine
│   ├── postgres.go
│   └── mysql.go
├── pitr/              # Point-in-time recovery
│   ├── wal.go
│   ├── binlog.go
│   └── timeline.go
├── verification/      # Backup verification
│   ├── checksum.go
│   └── validate.go
├── retention/         # Retention policy
│   └── cleanup.go
├── metrics/           # Prometheus metrics
│   └── exporter.go
└── replication/       # Replication management
    └── slots.go
```

### Required Dependencies

```go
// Cloud storage
"github.com/aws/aws-sdk-go-v2/service/s3"
"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
"cloud.google.com/go/storage"

// Encryption
"crypto/aes"
"crypto/cipher"
"golang.org/x/crypto/argon2"

// Metrics
"github.com/prometheus/client_golang/prometheus"
"github.com/prometheus/client_golang/prometheus/promhttp"

// PostgreSQL replication
"github.com/jackc/pgx/v5/pgconn"

// Fast file scanning for incrementals
"github.com/karrick/godirwalk"
```

---

## Testing Strategy

### v2.0 Test Coverage Goals
- Minimum 90% code coverage
- Integration tests for all cloud providers
- End-to-end PITR scenarios
- Performance benchmarks for incremental backups
- Encryption/decryption validation
- Multi-database restore tests

### New Test Suites
```bash
# Cloud storage tests
./run_qa_tests.sh --suite cloud

# Incremental backup tests
./run_qa_tests.sh --suite incremental

# PITR tests
./run_qa_tests.sh --suite pitr

# Encryption tests
./run_qa_tests.sh --suite encryption

# Full v2.0 suite
./run_qa_tests.sh --suite v2
```

---

## Migration Path

### v1.x → v2.0 Compatibility
- ✅ All v1.x backups readable in v2.0
- ✅ Configuration auto-migration
- ✅ Metadata format upgrade
- ✅ Backward-compatible commands

### Deprecation Timeline
- v2.0: Warning for old config format
- v2.1: Full migration required
- v3.0: Old format no longer supported

---

## Documentation Updates

### New Docs
- `CLOUD.md` - Cloud storage configuration
- `INCREMENTAL.md` - Incremental backup guide
- `PITR.md` - Point-in-time recovery
- `ENCRYPTION.md` - Encryption setup
- `METRICS.md` - Prometheus integration

---

## Success Metrics

### v2.0 Goals
- 🎯 95%+ test coverage
- 🎯 Support 1TB+ databases with incrementals
- 🎯 PITR with <5 minute granularity
- 🎯 Cloud upload/download >100MB/s
- 🎯 Encryption overhead <10%
- 🎯 Full compatibility with pgBackRest for PostgreSQL
- 🎯 Industry-leading MySQL PITR solution

---

## Release Schedule

- **v2.0-alpha** (End Sprint 3): Cloud + Verification
- **v2.0-beta** (End Sprint 5): + Incremental + PITR
- **v2.0-rc1** (End Sprint 6): + Enterprise features
- **v2.0 GA** (Q2 2026): Production release

---

## What Makes v2.0 Unique

After v2.0, dbbackup will be:

✅ **Only multi-database tool** with full PITR support  
✅ **Best-in-class UX** (TUI + CLI + Docker + K8s)  
✅ **Feature parity** with pgBackRest (PostgreSQL)  
✅ **Superior to mysqldump** with incremental + PITR  
✅ **Cloud-native** with multi-provider support  
✅ **Enterprise-ready** with encryption + metrics  
✅ **Zero-config** for 80% of use cases  

---

## Contributing

Want to contribute to v2.0? Check out:
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [Good First Issues](https://git.uuxo.net/uuxo/dbbackup/issues?labels=good-first-issue)
- [v2.0 Milestone](https://git.uuxo.net/uuxo/dbbackup/milestone/2)

---

## Questions?

Open an issue or start a discussion:
- Issues: https://git.uuxo.net/uuxo/dbbackup/issues
- Discussions: https://git.uuxo.net/uuxo/dbbackup/discussions

---

**Next Step:** Sprint 1 - Backup Verification & Retention (January 2026)
