# Google Cloud Storage Integration

This guide covers using **Google Cloud Storage (GCS)** with `dbbackup` for secure, scalable cloud backup storage.

## Table of Contents

- [Quick Start](#quick-start)
- [URI Syntax](#uri-syntax)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [Usage Examples](#usage-examples)
- [Advanced Features](#advanced-features)
- [Testing with fake-gcs-server](#testing-with-fake-gcs-server)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Quick Start

### 1. GCP Setup

1. Create a GCS bucket in Google Cloud Console
2. Set up authentication (choose one):
   - **Service Account**: Create and download JSON key file
   - **Application Default Credentials**: Use gcloud CLI
   - **Workload Identity**: For GKE clusters

### 2. Basic Backup

```bash
# Backup PostgreSQL to GCS (using ADC)
dbbackup backup postgres \
  --host localhost \
  --database mydb \
  --output backup.sql \
  --cloud "gs://mybucket/backups/db.sql"
```

### 3. Restore from GCS

```bash
# Restore from GCS backup
dbbackup restore postgres \
  --source "gs://mybucket/backups/db.sql" \
  --host localhost \
  --database mydb_restored
```

## URI Syntax

### Basic Format

```
gs://bucket/path/to/backup.sql
gcs://bucket/path/to/backup.sql
```

Both `gs://` and `gcs://` prefixes are supported.

### URI Components

| Component | Required | Description | Example |
|-----------|----------|-------------|---------|
| `bucket` | Yes | GCS bucket name | `mybucket` |
| `path` | Yes | Object path within bucket | `backups/db.sql` |
| `credentials` | No | Path to service account JSON | `/path/to/key.json` |
| `project` | No | GCP project ID | `my-project-id` |
| `endpoint` | No | Custom endpoint (emulator) | `http://localhost:4443` |

### URI Examples

**Production GCS (Application Default Credentials):**
```
gs://prod-backups/postgres/db.sql
```

**With Service Account:**
```
gs://prod-backups/postgres/db.sql?credentials=/path/to/service-account.json
```

**With Project ID:**
```
gs://prod-backups/postgres/db.sql?project=my-project-id
```

**fake-gcs-server Emulator:**
```
gs://test-backups/postgres/db.sql?endpoint=http://localhost:4443/storage/v1
```

**With Path Prefix:**
```
gs://backups/production/postgres/2024/db.sql
```

## Authentication

### Method 1: Application Default Credentials (Recommended)

Use gcloud CLI to set up ADC:

```bash
# Login with your Google account
gcloud auth application-default login

# Or use service account for server environments
gcloud auth activate-service-account --key-file=/path/to/key.json

# Use simplified URI (credentials from environment)
dbbackup backup postgres --cloud "gs://mybucket/backups/backup.sql"
```

### Method 2: Service Account JSON

Download service account key from GCP Console:

1. Go to **IAM & Admin** → **Service Accounts**
2. Create or select a service account
3. Click **Keys** → **Add Key** → **Create new key** → **JSON**
4. Download the JSON file

**Use in URI:**
```bash
dbbackup backup postgres \
  --cloud "gs://mybucket/backup.sql?credentials=/path/to/service-account.json"
```

**Or via environment:**
```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
dbbackup backup postgres --cloud "gs://mybucket/backup.sql"
```

### Method 3: Workload Identity (GKE)

For Kubernetes workloads:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: dbbackup-sa
  annotations:
    iam.gke.io/gcp-service-account: dbbackup@project.iam.gserviceaccount.com
```

Then use ADC in your pod:

```bash
dbbackup backup postgres --cloud "gs://mybucket/backup.sql"
```

### Required IAM Permissions

Service account needs these roles:

- **Storage Object Creator**: Upload backups
- **Storage Object Viewer**: List and download backups
- **Storage Object Admin**: Delete backups (for cleanup)

Or use predefined role: **Storage Admin**

```bash
# Grant permissions
gcloud projects add-iam-policy-binding PROJECT_ID \
  --member="serviceAccount:dbbackup@PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin"
```

## Configuration

### Bucket Setup

Create a bucket before first use:

```bash
# gcloud CLI
gsutil mb -p PROJECT_ID -c STANDARD -l us-central1 gs://mybucket/

# Or let dbbackup create it (requires permissions)
dbbackup cloud upload file.sql "gs://mybucket/file.sql?create=true&project=PROJECT_ID"
```

### Storage Classes

GCS offers multiple storage classes:

- **Standard**: Frequent access (default)
- **Nearline**: Access <1/month (lower cost)
- **Coldline**: Access <1/quarter (very low cost)
- **Archive**: Long-term retention (lowest cost)

Set the class when creating bucket:

```bash
gsutil mb -c NEARLINE gs://mybucket/
```

### Lifecycle Management

Configure automatic transitions and deletion:

```json
{
  "lifecycle": {
    "rule": [
      {
        "action": {"type": "SetStorageClass", "storageClass": "NEARLINE"},
        "condition": {"age": 30, "matchesPrefix": ["backups/"]}
      },
      {
        "action": {"type": "SetStorageClass", "storageClass": "ARCHIVE"},
        "condition": {"age": 90, "matchesPrefix": ["backups/"]}
      },
      {
        "action": {"type": "Delete"},
        "condition": {"age": 365, "matchesPrefix": ["backups/"]}
      }
    ]
  }
}
```

Apply lifecycle configuration:

```bash
gsutil lifecycle set lifecycle.json gs://mybucket/
```

### Regional Configuration

Choose bucket location for better performance:

```bash
# US regions
gsutil mb -l us-central1 gs://mybucket/
gsutil mb -l us-east1 gs://mybucket/

# EU regions
gsutil mb -l europe-west1 gs://mybucket/

# Multi-region
gsutil mb -l us gs://mybucket/
gsutil mb -l eu gs://mybucket/
```

## Usage Examples

### Backup with Auto-Upload

```bash
# PostgreSQL backup with automatic GCS upload
dbbackup backup postgres \
  --host localhost \
  --database production_db \
  --output /backups/db.sql \
  --cloud "gs://prod-backups/postgres/$(date +%Y%m%d_%H%M%S).sql" \
  --compression 6
```

### Backup All Databases

```bash
# Backup entire PostgreSQL cluster to GCS
dbbackup backup postgres \
  --host localhost \
  --all-databases \
  --output-dir /backups \
  --cloud "gs://prod-backups/postgres/cluster/"
```

### Verify Backup

```bash
# Verify backup integrity
dbbackup verify "gs://prod-backups/postgres/backup.sql"
```

### List Backups

```bash
# List all backups in bucket
dbbackup cloud list "gs://prod-backups/postgres/"

# List with pattern
dbbackup cloud list "gs://prod-backups/postgres/2024/"

# Or use gsutil
gsutil ls gs://prod-backups/postgres/
```

### Download Backup

```bash
# Download from GCS to local
dbbackup cloud download \
  "gs://prod-backups/postgres/backup.sql" \
  /local/path/backup.sql
```

### Delete Old Backups

```bash
# Manual delete
dbbackup cloud delete "gs://prod-backups/postgres/old_backup.sql"

# Automatic cleanup (keep last 7 backups)
dbbackup cleanup "gs://prod-backups/postgres/" --keep 7
```

### Scheduled Backups

```bash
#!/bin/bash
# GCS backup script (run via cron)

DATE=$(date +%Y%m%d_%H%M%S)
GCS_URI="gs://prod-backups/postgres/${DATE}.sql"

dbbackup backup postgres \
  --host localhost \
  --database production_db \
  --output /tmp/backup.sql \
  --cloud "${GCS_URI}" \
  --compression 9

# Cleanup old backups
dbbackup cleanup "gs://prod-backups/postgres/" --keep 30
```

**Crontab:**
```cron
# Daily at 2 AM
0 2 * * * /usr/local/bin/gcs-backup.sh >> /var/log/gcs-backup.log 2>&1
```

**Systemd Timer:**
```ini
# /etc/systemd/system/gcs-backup.timer
[Unit]
Description=Daily GCS Database Backup

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

## Advanced Features

### Chunked Upload

For large files, dbbackup automatically uses GCS chunked upload:

- **Chunk Size**: 16MB per chunk
- **Streaming**: Direct streaming from source
- **Checksum**: SHA-256 integrity verification

```bash
# Large database backup (automatically uses chunked upload)
dbbackup backup postgres \
  --host localhost \
  --database huge_db \
  --output /backups/huge.sql \
  --cloud "gs://backups/huge.sql"
```

### Progress Tracking

```bash
# Backup with progress display
dbbackup backup postgres \
  --host localhost \
  --database mydb \
  --output backup.sql \
  --cloud "gs://backups/backup.sql" \
  --progress
```

### Concurrent Operations

```bash
# Backup multiple databases in parallel
dbbackup backup postgres \
  --host localhost \
  --all-databases \
  --output-dir /backups \
  --cloud "gs://backups/cluster/" \
  --parallelism 4
```

### Custom Metadata

Backups include SHA-256 checksums as object metadata:

```bash
# View metadata using gsutil
gsutil stat gs://backups/backup.sql
```

### Object Versioning

Enable versioning to protect against accidental deletion:

```bash
# Enable versioning
gsutil versioning set on gs://mybucket/

# List all versions
gsutil ls -a gs://mybucket/backup.sql

# Restore previous version
gsutil cp gs://mybucket/backup.sql#VERSION /local/backup.sql
```

### Customer-Managed Encryption Keys (CMEK)

Use your own encryption keys:

```bash
# Create encryption key in Cloud KMS
gcloud kms keyrings create backup-keyring --location=us-central1
gcloud kms keys create backup-key --location=us-central1 --keyring=backup-keyring --purpose=encryption

# Set default CMEK for bucket
gsutil kms encryption gs://mybucket/ projects/PROJECT/locations/us-central1/keyRings/backup-keyring/cryptoKeys/backup-key
```

## Testing with fake-gcs-server

### Setup fake-gcs-server Emulator

**Docker Compose:**
```yaml
services:
  gcs-emulator:
    image: fsouza/fake-gcs-server:latest
    ports:
      - "4443:4443"
    command: -scheme http -public-host localhost:4443
```

**Start:**
```bash
docker-compose -f docker-compose.gcs.yml up -d
```

### Create Test Bucket

```bash
# Using curl
curl -X POST "http://localhost:4443/storage/v1/b?project=test-project" \
  -H "Content-Type: application/json" \
  -d '{"name": "test-backups"}'
```

### Test Backup

```bash
# Backup to fake-gcs-server
dbbackup backup postgres \
  --host localhost \
  --database testdb \
  --output test.sql \
  --cloud "gs://test-backups/test.sql?endpoint=http://localhost:4443/storage/v1"
```

### Run Integration Tests

```bash
# Run comprehensive test suite
./scripts/test_gcs_storage.sh
```

Tests include:
- PostgreSQL and MySQL backups
- Upload/download operations
- Large file handling (200MB+)
- Verification and cleanup
- Restore operations

## Best Practices

### 1. Security

- **Never commit credentials** to version control
- Use **Application Default Credentials** when possible
- Rotate service account keys regularly
- Use **Workload Identity** for GKE
- Enable **VPC Service Controls** for enterprise security
- Use **Customer-Managed Encryption Keys** (CMEK) for sensitive data

### 2. Performance

- Use **compression** for faster uploads: `--compression 6`
- Enable **parallelism** for cluster backups: `--parallelism 4`
- Choose appropriate **GCS region** (close to source)
- Use **multi-region** buckets for high availability

### 3. Cost Optimization

- Use **Nearline** for backups older than 30 days
- Use **Archive** for long-term retention (>90 days)
- Enable **lifecycle management** for automatic transitions
- Monitor storage costs in GCP Billing Console
- Use **Coldline** for quarterly access patterns

### 4. Reliability

- Test **restore procedures** regularly
- Use **retention policies**: `--keep 30`
- Enable **object versioning** (30-day recovery)
- Use **multi-region** buckets for disaster recovery
- Monitor backup success with Cloud Monitoring

### 5. Organization

- Use **consistent naming**: `{database}/{date}/{backup}.sql`
- Use **bucket prefixes**: `prod-backups`, `dev-backups`
- Tag backups with **labels** (environment, version)
- Document restore procedures
- Use **separate buckets** per environment

## Troubleshooting

### Connection Issues

**Problem:** `failed to create GCS client`

**Solutions:**
- Check `GOOGLE_APPLICATION_CREDENTIALS` environment variable
- Verify service account JSON file exists and is valid
- Ensure gcloud CLI is authenticated: `gcloud auth list`
- For emulator, confirm `http://localhost:4443` is running

### Authentication Errors

**Problem:** `authentication failed` or `permission denied`

**Solutions:**
- Verify service account has required IAM roles
- Check if Application Default Credentials are set up
- Run `gcloud auth application-default login`
- Verify service account JSON is not corrupted
- Check GCP project ID is correct

### Upload Failures

**Problem:** `failed to upload object`

**Solutions:**
- Check bucket exists (or use `&create=true`)
- Verify service account has `storage.objects.create` permission
- Check network connectivity to GCS
- Try smaller files first (test connection)
- Check GCP quota limits

### Large File Issues

**Problem:** Upload timeout for large files

**Solutions:**
- dbbackup automatically uses chunked upload
- Increase compression: `--compression 9`
- Check network bandwidth
- Use **Transfer Appliance** for TB+ data

### List/Download Issues

**Problem:** `object not found`

**Solutions:**
- Verify object name (check GCS Console)
- Check bucket name is correct
- Ensure object hasn't been moved/deleted
- Check if object is in Archive class (requires restore)

### Performance Issues

**Problem:** Slow upload/download

**Solutions:**
- Use compression: `--compression 6`
- Choose closer GCS region
- Check network bandwidth
- Use **multi-region** bucket for better availability
- Enable parallelism for multiple files

### Debugging

Enable debug mode:

```bash
dbbackup backup postgres \
  --cloud "gs://bucket/backup.sql" \
  --debug
```

Check GCP logs:

```bash
# Cloud Logging
gcloud logging read "resource.type=gcs_bucket AND resource.labels.bucket_name=mybucket" \
  --limit 50 \
  --format json
```

View bucket details:

```bash
gsutil ls -L -b gs://mybucket/
```

## Monitoring and Alerting

### Cloud Monitoring

Create metrics and alerts:

```bash
# Monitor backup success rate
gcloud monitoring policies create \
  --notification-channels=CHANNEL_ID \
  --display-name="Backup Failure Alert" \
  --condition-display-name="No backups in 24h" \
  --condition-threshold-value=0 \
  --condition-threshold-duration=86400s
```

### Logging

Export logs to BigQuery for analysis:

```bash
gcloud logging sinks create backup-logs \
  bigquery.googleapis.com/projects/PROJECT_ID/datasets/backup_logs \
  --log-filter='resource.type="gcs_bucket" AND resource.labels.bucket_name="prod-backups"'
```

## Additional Resources

- [Google Cloud Storage Documentation](https://cloud.google.com/storage/docs)
- [fake-gcs-server](https://github.com/fsouza/fake-gcs-server)
- [gsutil Tool](https://cloud.google.com/storage/docs/gsutil)
- [GCS Client Libraries](https://cloud.google.com/storage/docs/reference/libraries)
- [dbbackup Cloud Storage Guide](CLOUD.md)

## Support

For issues specific to GCS integration:

1. Check [Troubleshooting](#troubleshooting) section
2. Run integration tests: `./scripts/test_gcs_storage.sh`
3. Enable debug mode: `--debug`
4. Check GCP Service Status
5. Open an issue on GitHub with debug logs

## See Also

- [Azure Blob Storage Guide](AZURE.md)
- [AWS S3 Guide](CLOUD.md#aws-s3)
- [Main Cloud Storage Documentation](CLOUD.md)
