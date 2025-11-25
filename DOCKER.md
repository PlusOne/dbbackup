# Docker Usage Guide

## Quick Start

### Build Image

```bash
docker build -t dbbackup:latest .
```

### Run Container

**PostgreSQL Backup:**
```bash
docker run --rm \
  -v $(pwd)/backups:/backups \
  -e PGHOST=your-postgres-host \
  -e PGUSER=postgres \
  -e PGPASSWORD=secret \
  dbbackup:latest backup single mydb
```

**MySQL Backup:**
```bash
docker run --rm \
  -v $(pwd)/backups:/backups \
  -e MYSQL_HOST=your-mysql-host \
  -e MYSQL_USER=root \
  -e MYSQL_PWD=secret \
  dbbackup:latest backup single mydb --db-type mysql
```

**Interactive Mode:**
```bash
docker run --rm -it \
  -v $(pwd)/backups:/backups \
  -e PGHOST=your-postgres-host \
  -e PGUSER=postgres \
  -e PGPASSWORD=secret \
  dbbackup:latest interactive
```

## Docker Compose

### Start Test Environment

```bash
# Start test databases
docker-compose up -d postgres mysql

# Wait for databases to be ready
sleep 10

# Run backup
docker-compose run --rm postgres-backup
```

### Interactive Mode

```bash
docker-compose run --rm dbbackup-interactive
```

### Scheduled Backups with Cron

Create `docker-cron`:
```bash
#!/bin/bash
# Daily PostgreSQL backup at 2 AM
0 2 * * * docker run --rm -v /backups:/backups -e PGHOST=postgres -e PGUSER=postgres -e PGPASSWORD=secret dbbackup:latest backup single production_db
```

## Environment Variables

**PostgreSQL:**
- `PGHOST` - Database host
- `PGPORT` - Database port (default: 5432)
- `PGUSER` - Database user
- `PGPASSWORD` - Database password
- `PGDATABASE` - Database name

**MySQL/MariaDB:**
- `MYSQL_HOST` - Database host
- `MYSQL_PORT` - Database port (default: 3306)
- `MYSQL_USER` - Database user
- `MYSQL_PWD` - Database password
- `MYSQL_DATABASE` - Database name

**General:**
- `BACKUP_DIR` - Backup directory (default: /backups)
- `COMPRESS_LEVEL` - Compression level 0-9 (default: 6)

## Volume Mounts

```bash
docker run --rm \
  -v /host/backups:/backups \              # Backup storage
  -v /host/config/.dbbackup.conf:/home/dbbackup/.dbbackup.conf:ro \  # Config file
  dbbackup:latest backup single mydb
```

## Docker Hub

Pull pre-built image (when published):
```bash
docker pull uuxo/dbbackup:latest
docker pull uuxo/dbbackup:1.0
```

## Kubernetes Deployment

**CronJob Example:**
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: postgres-backup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: dbbackup
            image: dbbackup:latest
            args: ["backup", "single", "production_db"]
            env:
            - name: PGHOST
              value: "postgres.default.svc.cluster.local"
            - name: PGUSER
              value: "postgres"
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-secret
                  key: password
            volumeMounts:
            - name: backups
              mountPath: /backups
          volumes:
          - name: backups
            persistentVolumeClaim:
              claimName: backup-storage
          restartPolicy: OnFailure
```

## Docker Secrets

**Using Docker Secrets:**
```bash
# Create secrets
echo "mypassword" | docker secret create db_password -

# Use in stack
docker stack deploy -c docker-stack.yml dbbackup
```

**docker-stack.yml:**
```yaml
version: '3.8'
services:
  backup:
    image: dbbackup:latest
    secrets:
      - db_password
    environment:
      - PGHOST=postgres
      - PGUSER=postgres
      - PGPASSWORD_FILE=/run/secrets/db_password
    command: backup single mydb
    volumes:
      - backups:/backups

secrets:
  db_password:
    external: true

volumes:
  backups:
```

## Image Size

**Multi-stage build results:**
- Builder stage: ~500MB (Go + dependencies)
- Final image: ~100MB (Alpine + clients)
- Binary only: ~15MB

## Security

**Non-root user:**
- Runs as UID 1000 (dbbackup user)
- No privileged operations needed
- Read-only config mount recommended

**Network:**
```bash
# Use custom network
docker network create dbnet

docker run --rm \
  --network dbnet \
  -v $(pwd)/backups:/backups \
  dbbackup:latest backup single mydb
```

## Troubleshooting

**Check logs:**
```bash
docker logs dbbackup-postgres
```

**Debug mode:**
```bash
docker run --rm -it \
  -v $(pwd)/backups:/backups \
  dbbackup:latest backup single mydb --debug
```

**Shell access:**
```bash
docker run --rm -it --entrypoint /bin/sh dbbackup:latest
```

## Building for Multiple Platforms

```bash
# Enable buildx
docker buildx create --use

# Build multi-arch
docker buildx build \
  --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t uuxo/dbbackup:latest \
  --push .
```

## Registry Push

```bash
# Tag for registry
docker tag dbbackup:latest git.uuxo.net/uuxo/dbbackup:latest
docker tag dbbackup:latest git.uuxo.net/uuxo/dbbackup:1.0

# Push to private registry
docker push git.uuxo.net/uuxo/dbbackup:latest
docker push git.uuxo.net/uuxo/dbbackup:1.0
```
