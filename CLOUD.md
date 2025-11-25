# Cloud Storage Guide for dbbackup

## Overview

dbbackup v2.0 includes comprehensive cloud storage integration, allowing you to backup directly to S3-compatible storage providers and restore from cloud URIs.

**Supported Providers:**
- AWS S3
- MinIO (self-hosted S3-compatible)
- Backblaze B2
- Google Cloud Storage (via S3 compatibility)
- Any S3-compatible storage

**Key Features:**
- ✅ Direct backup to cloud with `--cloud` URI flag
- ✅ Restore from cloud URIs
- ✅ Verify cloud backup integrity
- ✅ Apply retention policies to cloud storage
- ✅ Multipart upload for large files (>100MB)
- ✅ Progress tracking for uploads/downloads
- ✅ Automatic metadata synchronization
- ✅ Streaming transfers (memory efficient)

---

## Quick Start

### 1. Set Up Credentials

```bash
# For AWS S3
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_REGION="us-east-1"

# For MinIO
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin123"
export AWS_ENDPOINT_URL="http://localhost:9000"

# For Backblaze B2
export AWS_ACCESS_KEY_ID="your-b2-key-id"
export AWS_SECRET_ACCESS_KEY="your-b2-application-key"
export AWS_ENDPOINT_URL="https://s3.us-west-002.backblazeb2.com"
```

### 2. Backup with Cloud URI

```bash
# Backup to S3
dbbackup backup single mydb --cloud s3://my-bucket/backups/

# Backup to MinIO
dbbackup backup single mydb --cloud minio://my-bucket/backups/

# Backup to Backblaze B2
dbbackup backup single mydb --cloud b2://my-bucket/backups/
```

### 3. Restore from Cloud

```bash
# Restore from cloud URI
dbbackup restore single s3://my-bucket/backups/mydb_20260115_120000.dump --confirm

# Restore to different database
dbbackup restore single s3://my-bucket/backups/mydb.dump \
    --target mydb_restored \
    --confirm
```

---

## URI Syntax

Cloud URIs follow this format:

```
<provider>://<bucket>/<path>/<filename>
```

**Supported Providers:**
- `s3://` - AWS S3 or S3-compatible storage
- `minio://` - MinIO (auto-enables path-style addressing)
- `b2://` - Backblaze B2
- `gs://` or `gcs://` - Google Cloud Storage
- `azure://` - Azure Blob Storage (coming soon)

**Examples:**
```bash
s3://production-backups/databases/postgres/
minio://local-backups/dev/mydb/
b2://offsite-backups/daily/
gs://gcp-backups/prod/
```

---

## Configuration Methods

### Method 1: Cloud URIs (Recommended)

```bash
dbbackup backup single mydb --cloud s3://my-bucket/backups/
```

### Method 2: Individual Flags

```bash
dbbackup backup single mydb \
    --cloud-auto-upload \
    --cloud-provider s3 \
    --cloud-bucket my-bucket \
    --cloud-prefix backups/
```

### Method 3: Environment Variables

```bash
export CLOUD_ENABLED=true
export CLOUD_AUTO_UPLOAD=true
export CLOUD_PROVIDER=s3
export CLOUD_BUCKET=my-bucket
export CLOUD_PREFIX=backups/
export CLOUD_REGION=us-east-1

dbbackup backup single mydb
```

### Method 4: Config File

```toml
# ~/.dbbackup.conf
[cloud]
enabled = true
auto_upload = true
provider = "s3"
bucket = "my-bucket"
prefix = "backups/"
region = "us-east-1"
```

---

## Commands

### Cloud Upload

Upload existing backup files to cloud storage:

```bash
# Upload single file
dbbackup cloud upload /backups/mydb.dump \
    --cloud-provider s3 \
    --cloud-bucket my-bucket

# Upload with cloud URI flags
dbbackup cloud upload /backups/mydb.dump \
    --cloud-provider minio \
    --cloud-bucket local-backups \
    --cloud-endpoint http://localhost:9000

# Upload multiple files
dbbackup cloud upload /backups/*.dump \
    --cloud-provider s3 \
    --cloud-bucket my-bucket \
    --verbose
```

### Cloud Download

Download backups from cloud storage:

```bash
# Download to current directory
dbbackup cloud download mydb.dump . \
    --cloud-provider s3 \
    --cloud-bucket my-bucket

# Download to specific directory
dbbackup cloud download backups/mydb.dump /restore/ \
    --cloud-provider s3 \
    --cloud-bucket my-bucket \
    --verbose
```

### Cloud List

List backups in cloud storage:

```bash
# List all backups
dbbackup cloud list \
    --cloud-provider s3 \
    --cloud-bucket my-bucket

# List with prefix filter
dbbackup cloud list \
    --cloud-provider s3 \
    --cloud-bucket my-bucket \
    --cloud-prefix postgres/

# Verbose output with details
dbbackup cloud list \
    --cloud-provider s3 \
    --cloud-bucket my-bucket \
    --verbose
```

### Cloud Delete

Delete backups from cloud storage:

```bash
# Delete specific backup (with confirmation prompt)
dbbackup cloud delete mydb_old.dump \
    --cloud-provider s3 \
    --cloud-bucket my-bucket

# Delete without confirmation
dbbackup cloud delete mydb_old.dump \
    --cloud-provider s3 \
    --cloud-bucket my-bucket \
    --confirm
```

### Backup with Auto-Upload

```bash
# Backup and automatically upload
dbbackup backup single mydb --cloud s3://my-bucket/backups/

# With individual flags
dbbackup backup single mydb \
    --cloud-auto-upload \
    --cloud-provider s3 \
    --cloud-bucket my-bucket \
    --cloud-prefix backups/
```

### Restore from Cloud

```bash
# Restore from cloud URI (auto-download)
dbbackup restore single s3://my-bucket/backups/mydb.dump --confirm

# Restore to different database
dbbackup restore single s3://my-bucket/backups/mydb.dump \
    --target mydb_restored \
    --confirm

# Restore with database creation
dbbackup restore single s3://my-bucket/backups/mydb.dump \
    --create \
    --confirm
```

### Verify Cloud Backups

```bash
# Verify single cloud backup
dbbackup verify-backup s3://my-bucket/backups/mydb.dump

# Quick verification (size check only)
dbbackup verify-backup s3://my-bucket/backups/mydb.dump --quick

# Verbose output
dbbackup verify-backup s3://my-bucket/backups/mydb.dump --verbose
```

### Cloud Cleanup

Apply retention policies to cloud storage:

```bash
# Cleanup old backups (dry-run)
dbbackup cleanup s3://my-bucket/backups/ \
    --retention-days 30 \
    --min-backups 5 \
    --dry-run

# Actual cleanup
dbbackup cleanup s3://my-bucket/backups/ \
    --retention-days 30 \
    --min-backups 5

# Pattern-based cleanup
dbbackup cleanup s3://my-bucket/backups/ \
    --retention-days 7 \
    --min-backups 3 \
    --pattern "mydb_*.dump"
```

---

## Provider-Specific Setup

### AWS S3

**Prerequisites:**
- AWS account
- S3 bucket created
- IAM user with S3 permissions

**IAM Policy:**
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::my-bucket/*",
        "arn:aws:s3:::my-bucket"
      ]
    }
  ]
}
```

**Configuration:**
```bash
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_REGION="us-east-1"

dbbackup backup single mydb --cloud s3://my-bucket/backups/
```

### MinIO (Self-Hosted)

**Setup with Docker:**
```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e "MINIO_ROOT_USER=minioadmin" \
  -e "MINIO_ROOT_PASSWORD=minioadmin123" \
  --name minio \
  minio/minio server /data --console-address ":9001"

# Create bucket
docker exec minio mc alias set local http://localhost:9000 minioadmin minioadmin123
docker exec minio mc mb local/backups
```

**Configuration:**
```bash
export AWS_ACCESS_KEY_ID="minioadmin"
export AWS_SECRET_ACCESS_KEY="minioadmin123"
export AWS_ENDPOINT_URL="http://localhost:9000"

dbbackup backup single mydb --cloud minio://backups/db/
```

**Or use docker-compose:**
```bash
docker-compose -f docker-compose.minio.yml up -d
```

### Backblaze B2

**Prerequisites:**
- Backblaze account
- B2 bucket created
- Application key generated

**Configuration:**
```bash
export AWS_ACCESS_KEY_ID="<your-b2-key-id>"
export AWS_SECRET_ACCESS_KEY="<your-b2-application-key>"
export AWS_ENDPOINT_URL="https://s3.us-west-002.backblazeb2.com"
export AWS_REGION="us-west-002"

dbbackup backup single mydb --cloud b2://my-bucket/backups/
```

### Google Cloud Storage

**Prerequisites:**
- GCP account
- GCS bucket with S3 compatibility enabled
- HMAC keys generated

**Enable S3 Compatibility:**
1. Go to Cloud Storage > Settings > Interoperability
2. Create HMAC keys

**Configuration:**
```bash
export AWS_ACCESS_KEY_ID="<your-hmac-access-id>"
export AWS_SECRET_ACCESS_KEY="<your-hmac-secret>"
export AWS_ENDPOINT_URL="https://storage.googleapis.com"

dbbackup backup single mydb --cloud gs://my-bucket/backups/
```

---

## Features

### Multipart Upload

Files larger than 100MB automatically use multipart upload for:
- Faster transfers with parallel parts
- Resume capability on failure
- Better reliability for large files

**Configuration:**
- Part size: 10MB
- Concurrency: 10 parallel parts
- Automatic based on file size

### Progress Tracking

Real-time progress for uploads and downloads:

```bash
Uploading backup to cloud...
Progress: 10%
Progress: 20%
Progress: 30%
...
Upload completed: /backups/mydb.dump (1.2 GB)
```

### Metadata Synchronization

Automatically uploads `.meta.json` with each backup containing:
- SHA-256 checksum
- Database name and type
- Backup timestamp
- File size
- Compression info

### Automatic Verification

Downloads from cloud include automatic checksum verification:

```bash
Downloading backup from cloud...
Download completed
Verifying checksum...
Checksum verified successfully: sha256=abc123...
```

---

## Testing

### Local Testing with MinIO

**1. Start MinIO:**
```bash
docker-compose -f docker-compose.minio.yml up -d
```

**2. Run Integration Tests:**
```bash
./scripts/test_cloud_storage.sh
```

**3. Manual Testing:**
```bash
# Set credentials
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin123
export AWS_ENDPOINT_URL=http://localhost:9000

# Test backup
dbbackup backup single mydb --cloud minio://test-backups/test/

# Test restore
dbbackup restore single minio://test-backups/test/mydb.dump --confirm

# Test verify
dbbackup verify-backup minio://test-backups/test/mydb.dump

# Test cleanup
dbbackup cleanup minio://test-backups/test/ --retention-days 7 --dry-run
```

**4. Access MinIO Console:**
- URL: http://localhost:9001
- Username: `minioadmin`
- Password: `minioadmin123`

---

## Best Practices

### Security

1. **Never commit credentials:**
   ```bash
   # Use environment variables or config files
   export AWS_ACCESS_KEY_ID="..."
   ```

2. **Use IAM roles when possible:**
   ```bash
   # On EC2/ECS, credentials are automatic
   dbbackup backup single mydb --cloud s3://bucket/
   ```

3. **Restrict bucket permissions:**
   - Minimum required: GetObject, PutObject, DeleteObject, ListBucket
   - Use bucket policies to limit access

4. **Enable encryption:**
   - S3: Server-side encryption enabled by default
   - MinIO: Configure encryption at rest

### Performance

1. **Use multipart for large backups:**
   - Automatic for files >100MB
   - Configure concurrency based on bandwidth

2. **Choose nearby regions:**
   ```bash
   --cloud-region us-west-2  # Closest to your servers
   ```

3. **Use compression:**
   ```bash
   --compression gzip  # Reduces upload size
   ```

### Reliability

1. **Test restores regularly:**
   ```bash
   # Monthly restore test
   dbbackup restore single s3://bucket/latest.dump --target test_restore
   ```

2. **Verify backups:**
   ```bash
   # Daily verification
   dbbackup verify-backup s3://bucket/backups/*.dump
   ```

3. **Monitor retention:**
   ```bash
   # Weekly cleanup check
   dbbackup cleanup s3://bucket/ --retention-days 30 --dry-run
   ```

### Cost Optimization

1. **Use lifecycle policies:**
   - S3: Transition old backups to Glacier
   - Configure in AWS Console or bucket policy

2. **Cleanup old backups:**
   ```bash
   dbbackup cleanup s3://bucket/ --retention-days 30 --min-backups 10
   ```

3. **Choose appropriate storage class:**
   - Standard: Frequent access
   - Infrequent Access: Monthly restores
   - Glacier: Long-term archive

---

## Troubleshooting

### Connection Issues

**Problem:** Cannot connect to S3/MinIO

```bash
Error: failed to create cloud backend: failed to load AWS config
```

**Solution:**
1. Check credentials:
   ```bash
   echo $AWS_ACCESS_KEY_ID
   echo $AWS_SECRET_ACCESS_KEY
   ```

2. Test connectivity:
   ```bash
   curl $AWS_ENDPOINT_URL
   ```

3. Verify endpoint URL for MinIO/B2

### Permission Errors

**Problem:** Access denied

```bash
Error: failed to upload to S3: AccessDenied
```

**Solution:**
1. Check IAM policy includes required permissions
2. Verify bucket name is correct
3. Check bucket policy allows your IAM user

### Upload Failures

**Problem:** Large file upload fails

```bash
Error: multipart upload failed: connection timeout
```

**Solution:**
1. Check network stability
2. Retry - multipart uploads resume automatically
3. Increase timeout in config
4. Check firewall allows outbound HTTPS

### Verification Failures

**Problem:** Checksum mismatch

```bash
Error: checksum mismatch: expected abc123, got def456
```

**Solution:**
1. Re-download the backup
2. Check if file was corrupted during upload
3. Verify original backup integrity locally
4. Re-upload if necessary

---

## Examples

### Full Backup Workflow

```bash
#!/bin/bash
# Daily backup to S3 with retention

# Backup all databases
for db in db1 db2 db3; do
    dbbackup backup single $db \
        --cloud s3://production-backups/daily/$db/ \
        --compression gzip
done

# Cleanup old backups (keep 30 days, min 10 backups)
dbbackup cleanup s3://production-backups/daily/ \
    --retention-days 30 \
    --min-backups 10

# Verify today's backups
dbbackup verify-backup s3://production-backups/daily/*/$(date +%Y%m%d)*.dump
```

### Disaster Recovery

```bash
#!/bin/bash
# Restore from cloud backup

# List available backups
dbbackup cloud list \
    --cloud-provider s3 \
    --cloud-bucket disaster-recovery \
    --verbose

# Restore latest backup
LATEST=$(dbbackup cloud list \
    --cloud-provider s3 \
    --cloud-bucket disaster-recovery | tail -1)

dbbackup restore single "s3://disaster-recovery/$LATEST" \
    --target restored_db \
    --create \
    --confirm
```

### Multi-Cloud Strategy

```bash
#!/bin/bash
# Backup to both AWS S3 and Backblaze B2

# Backup to S3
dbbackup backup single production_db \
    --cloud s3://aws-backups/prod/ \
    --output-dir /tmp/backups

# Also upload to B2
BACKUP_FILE=$(ls -t /tmp/backups/*.dump | head -1)
dbbackup cloud upload "$BACKUP_FILE" \
    --cloud-provider b2 \
    --cloud-bucket b2-offsite-backups \
    --cloud-endpoint https://s3.us-west-002.backblazeb2.com

# Verify both locations
dbbackup verify-backup s3://aws-backups/prod/$(basename $BACKUP_FILE)
dbbackup verify-backup b2://b2-offsite-backups/$(basename $BACKUP_FILE)
```

---

## FAQ

**Q: Can I use dbbackup with my existing S3 buckets?**  
A: Yes! Just specify your bucket name and credentials.

**Q: Do I need to keep local backups?**  
A: No, use `--cloud` flag to upload directly without keeping local copies.

**Q: What happens if upload fails?**  
A: Backup succeeds locally. Upload failure is logged but doesn't fail the backup.

**Q: Can I restore without downloading?**  
A: No, backups are downloaded to temp directory, then restored and cleaned up.

**Q: How much does cloud storage cost?**  
A: Varies by provider:
- AWS S3: ~$0.023/GB/month + transfer
- Backblaze B2: ~$0.005/GB/month + transfer
- MinIO: Self-hosted, hardware costs only

**Q: Can I use multiple cloud providers?**  
A: Yes! Use different URIs or upload to multiple destinations.

**Q: Is multipart upload automatic?**  
A: Yes, automatically used for files >100MB.

**Q: Can I use S3 Glacier?**  
A: Yes, but restore requires thawing. Use lifecycle policies for automatic archival.

---

## Related Documentation

- [README.md](README.md) - Main documentation
- [ROADMAP.md](ROADMAP.md) - Feature roadmap
- [docker-compose.minio.yml](docker-compose.minio.yml) - MinIO test setup
- [scripts/test_cloud_storage.sh](scripts/test_cloud_storage.sh) - Integration tests

---

## Support

For issues or questions:
- GitHub Issues: [Create an issue](https://github.com/yourusername/dbbackup/issues)
- Documentation: Check README.md and inline help
- Examples: See `scripts/test_cloud_storage.sh`
