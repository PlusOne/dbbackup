# Azure Blob Storage Integration

This guide covers using **Azure Blob Storage** with `dbbackup` for secure, scalable cloud backup storage.

## Table of Contents

- [Quick Start](#quick-start)
- [URI Syntax](#uri-syntax)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [Usage Examples](#usage-examples)
- [Advanced Features](#advanced-features)
- [Testing with Azurite](#testing-with-azurite)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Quick Start

### 1. Azure Portal Setup

1. Create a storage account in Azure Portal
2. Create a container for backups
3. Get your account credentials:
   - **Account Name**: Your storage account name
   - **Account Key**: Primary or secondary access key (from Access Keys section)

### 2. Basic Backup

```bash
# Backup PostgreSQL to Azure
dbbackup backup postgres \
  --host localhost \
  --database mydb \
  --output backup.sql \
  --cloud "azure://mycontainer/backups/db.sql?account=myaccount&key=ACCOUNT_KEY"
```

### 3. Restore from Azure

```bash
# Restore from Azure backup
dbbackup restore postgres \
  --source "azure://mycontainer/backups/db.sql?account=myaccount&key=ACCOUNT_KEY" \
  --host localhost \
  --database mydb_restored
```

## URI Syntax

### Basic Format

```
azure://container/path/to/backup.sql?account=ACCOUNT_NAME&key=ACCOUNT_KEY
```

### URI Components

| Component | Required | Description | Example |
|-----------|----------|-------------|---------|
| `container` | Yes | Azure container name | `mycontainer` |
| `path` | Yes | Object path within container | `backups/db.sql` |
| `account` | Yes | Storage account name | `mystorageaccount` |
| `key` | Yes | Storage account key | `base64-encoded-key` |
| `endpoint` | No | Custom endpoint (Azurite) | `http://localhost:10000` |

### URI Examples

**Production Azure:**
```
azure://prod-backups/postgres/db.sql?account=prodaccount&key=YOUR_KEY_HERE
```

**Azurite Emulator:**
```
azure://test-backups/postgres/db.sql?endpoint=http://localhost:10000&account=devstoreaccount1&key=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==
```

**With Path Prefix:**
```
azure://backups/production/postgres/2024/db.sql?account=myaccount&key=KEY
```

## Authentication

### Method 1: URI Parameters (Recommended for CLI)

Pass credentials directly in the URI:

```bash
azure://container/path?account=myaccount&key=YOUR_ACCOUNT_KEY
```

### Method 2: Environment Variables

Set credentials via environment:

```bash
export AZURE_STORAGE_ACCOUNT="myaccount"
export AZURE_STORAGE_KEY="YOUR_ACCOUNT_KEY"

# Use simplified URI (credentials from environment)
dbbackup backup postgres --cloud "azure://container/path/backup.sql"
```

### Method 3: Connection String

Use Azure connection string:

```bash
export AZURE_STORAGE_CONNECTION_STRING="DefaultEndpointsProtocol=https;AccountName=myaccount;AccountKey=YOUR_KEY;EndpointSuffix=core.windows.net"

dbbackup backup postgres --cloud "azure://container/path/backup.sql"
```

### Getting Your Account Key

1. Go to Azure Portal → Storage Accounts
2. Select your storage account
3. Navigate to **Security + networking** → **Access keys**
4. Copy **key1** or **key2**

**Important:** Keep your account keys secure. Use Azure Key Vault for production.

## Configuration

### Container Setup

Create a container before first use:

```bash
# Azure CLI
az storage container create \
  --name backups \
  --account-name myaccount \
  --account-key YOUR_KEY

# Or let dbbackup create it automatically
dbbackup cloud upload file.sql "azure://backups/file.sql?account=myaccount&key=KEY&create=true"
```

### Access Tiers

Azure Blob Storage offers multiple access tiers:

- **Hot**: Frequent access (default)
- **Cool**: Infrequent access (lower storage cost)
- **Archive**: Long-term retention (lowest cost, retrieval delay)

Set the tier in Azure Portal or using Azure CLI:

```bash
az storage blob set-tier \
  --container-name backups \
  --name backup.sql \
  --tier Cool \
  --account-name myaccount
```

### Lifecycle Management

Configure automatic tier transitions:

```json
{
  "rules": [
    {
      "name": "moveToArchive",
      "type": "Lifecycle",
      "definition": {
        "filters": {
          "blobTypes": ["blockBlob"],
          "prefixMatch": ["backups/"]
        },
        "actions": {
          "baseBlob": {
            "tierToCool": {
              "daysAfterModificationGreaterThan": 30
            },
            "tierToArchive": {
              "daysAfterModificationGreaterThan": 90
            },
            "delete": {
              "daysAfterModificationGreaterThan": 365
            }
          }
        }
      }
    }
  ]
}
```

## Usage Examples

### Backup with Auto-Upload

```bash
# PostgreSQL backup with automatic Azure upload
dbbackup backup postgres \
  --host localhost \
  --database production_db \
  --output /backups/db.sql \
  --cloud "azure://prod-backups/postgres/$(date +%Y%m%d_%H%M%S).sql?account=myaccount&key=KEY" \
  --compression 6
```

### Backup All Databases

```bash
# Backup entire PostgreSQL cluster to Azure
dbbackup backup postgres \
  --host localhost \
  --all-databases \
  --output-dir /backups \
  --cloud "azure://prod-backups/postgres/cluster/?account=myaccount&key=KEY"
```

### Verify Backup

```bash
# Verify backup integrity
dbbackup verify "azure://prod-backups/postgres/backup.sql?account=myaccount&key=KEY"
```

### List Backups

```bash
# List all backups in container
dbbackup cloud list "azure://prod-backups/postgres/?account=myaccount&key=KEY"

# List with pattern
dbbackup cloud list "azure://prod-backups/postgres/2024/?account=myaccount&key=KEY"
```

### Download Backup

```bash
# Download from Azure to local
dbbackup cloud download \
  "azure://prod-backups/postgres/backup.sql?account=myaccount&key=KEY" \
  /local/path/backup.sql
```

### Delete Old Backups

```bash
# Manual delete
dbbackup cloud delete "azure://prod-backups/postgres/old_backup.sql?account=myaccount&key=KEY"

# Automatic cleanup (keep last 7 backups)
dbbackup cleanup "azure://prod-backups/postgres/?account=myaccount&key=KEY" --keep 7
```

### Scheduled Backups

```bash
#!/bin/bash
# Azure backup script (run via cron)

DATE=$(date +%Y%m%d_%H%M%S)
AZURE_URI="azure://prod-backups/postgres/${DATE}.sql?account=myaccount&key=${AZURE_STORAGE_KEY}"

dbbackup backup postgres \
  --host localhost \
  --database production_db \
  --output /tmp/backup.sql \
  --cloud "${AZURE_URI}" \
  --compression 9

# Cleanup old backups
dbbackup cleanup "azure://prod-backups/postgres/?account=myaccount&key=${AZURE_STORAGE_KEY}" --keep 30
```

**Crontab:**
```cron
# Daily at 2 AM
0 2 * * * /usr/local/bin/azure-backup.sh >> /var/log/azure-backup.log 2>&1
```

## Advanced Features

### Block Blob Upload

For large files (>256MB), dbbackup automatically uses Azure Block Blob staging:

- **Block Size**: 100MB per block
- **Parallel Upload**: Multiple blocks uploaded concurrently
- **Checksum**: SHA-256 integrity verification

```bash
# Large database backup (automatically uses block blob)
dbbackup backup postgres \
  --host localhost \
  --database huge_db \
  --output /backups/huge.sql \
  --cloud "azure://backups/huge.sql?account=myaccount&key=KEY"
```

### Progress Tracking

```bash
# Backup with progress display
dbbackup backup postgres \
  --host localhost \
  --database mydb \
  --output backup.sql \
  --cloud "azure://backups/backup.sql?account=myaccount&key=KEY" \
  --progress
```

### Concurrent Operations

```bash
# Backup multiple databases in parallel
dbbackup backup postgres \
  --host localhost \
  --all-databases \
  --output-dir /backups \
  --cloud "azure://backups/cluster/?account=myaccount&key=KEY" \
  --parallelism 4
```

### Custom Metadata

Backups include SHA-256 checksums as blob metadata:

```bash
# Verify metadata using Azure CLI
az storage blob metadata show \
  --container-name backups \
  --name backup.sql \
  --account-name myaccount
```

## Testing with Azurite

### Setup Azurite Emulator

**Docker Compose:**
```yaml
services:
  azurite:
    image: mcr.microsoft.com/azure-storage/azurite:latest
    ports:
      - "10000:10000"
      - "10001:10001"
      - "10002:10002"
    command: azurite --blobHost 0.0.0.0 --loose
```

**Start:**
```bash
docker-compose -f docker-compose.azurite.yml up -d
```

### Default Azurite Credentials

```
Account Name: devstoreaccount1
Account Key: Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==
Endpoint: http://localhost:10000/devstoreaccount1
```

### Test Backup

```bash
# Backup to Azurite
dbbackup backup postgres \
  --host localhost \
  --database testdb \
  --output test.sql \
  --cloud "azure://test-backups/test.sql?endpoint=http://localhost:10000&account=devstoreaccount1&key=Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
```

### Run Integration Tests

```bash
# Run comprehensive test suite
./scripts/test_azure_storage.sh
```

Tests include:
- PostgreSQL and MySQL backups
- Upload/download operations
- Large file handling (300MB+)
- Verification and cleanup
- Restore operations

## Best Practices

### 1. Security

- **Never commit credentials** to version control
- Use **Azure Key Vault** for production keys
- Rotate account keys regularly
- Use **Shared Access Signatures (SAS)** for limited access
- Enable **Azure AD authentication** when possible

### 2. Performance

- Use **compression** for faster uploads: `--compression 6`
- Enable **parallelism** for cluster backups: `--parallelism 4`
- Choose appropriate **Azure region** (close to source)
- Use **Premium Storage** for high throughput

### 3. Cost Optimization

- Use **Cool tier** for backups older than 30 days
- Use **Archive tier** for long-term retention (>90 days)
- Enable **lifecycle management** for automatic transitions
- Monitor storage costs in Azure Cost Management

### 4. Reliability

- Test **restore procedures** regularly
- Use **retention policies**: `--keep 30`
- Enable **soft delete** in Azure (30-day recovery)
- Monitor backup success with Azure Monitor

### 5. Organization

- Use **consistent naming**: `{database}/{date}/{backup}.sql`
- Use **container prefixes**: `prod-backups`, `dev-backups`
- Tag backups with **metadata** (version, environment)
- Document restore procedures

## Troubleshooting

### Connection Issues

**Problem:** `failed to create Azure client`

**Solutions:**
- Verify account name is correct
- Check account key (copy from Azure Portal)
- Ensure endpoint is accessible (firewall rules)
- For Azurite, confirm `http://localhost:10000` is running

### Authentication Errors

**Problem:** `authentication failed`

**Solutions:**
- Check for spaces/special characters in key
- Verify account key hasn't been rotated
- Try using connection string method
- Check Azure firewall rules (allow your IP)

### Upload Failures

**Problem:** `failed to upload blob`

**Solutions:**
- Check container exists (or use `&create=true`)
- Verify sufficient storage quota
- Check network connectivity
- Try smaller files first (test connection)

### Large File Issues

**Problem:** Upload timeout for large files

**Solutions:**
- dbbackup automatically uses block blob for files >256MB
- Increase compression: `--compression 9`
- Check network bandwidth
- Use Azure Premium Storage for better throughput

### List/Download Issues

**Problem:** `blob not found`

**Solutions:**
- Verify blob name (check Azure Portal)
- Check container name is correct
- Ensure blob hasn't been moved/deleted
- Check if blob is in Archive tier (requires rehydration)

### Performance Issues

**Problem:** Slow upload/download

**Solutions:**
- Use compression: `--compression 6`
- Choose closer Azure region
- Check network bandwidth
- Use Azure Premium Storage
- Enable parallelism for multiple files

### Debugging

Enable debug mode:

```bash
dbbackup backup postgres \
  --cloud "azure://container/backup.sql?account=myaccount&key=KEY" \
  --debug
```

Check Azure logs:

```bash
# Azure CLI
az monitor activity-log list \
  --resource-group mygroup \
  --namespace Microsoft.Storage
```

## Additional Resources

- [Azure Blob Storage Documentation](https://docs.microsoft.com/azure/storage/blobs/)
- [Azurite Emulator](https://github.com/Azure/Azurite)
- [Azure Storage Explorer](https://azure.microsoft.com/features/storage-explorer/)
- [Azure CLI](https://docs.microsoft.com/cli/azure/storage)
- [dbbackup Cloud Storage Guide](CLOUD.md)

## Support

For issues specific to Azure integration:

1. Check [Troubleshooting](#troubleshooting) section
2. Run integration tests: `./scripts/test_azure_storage.sh`
3. Enable debug mode: `--debug`
4. Check Azure Service Health
5. Open an issue on GitHub with debug logs

## See Also

- [Google Cloud Storage Guide](GCS.md)
- [AWS S3 Guide](CLOUD.md#aws-s3)
- [Main Cloud Storage Documentation](CLOUD.md)
