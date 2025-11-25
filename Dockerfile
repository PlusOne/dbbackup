# Multi-stage build for minimal image size
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o dbbackup .

# Final stage - minimal runtime image
FROM alpine:3.19

# Install database client tools
RUN apk add --no-cache \
    postgresql-client \
    mysql-client \
    mariadb-client \
    pigz \
    pv \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 dbbackup && \
    adduser -D -u 1000 -G dbbackup dbbackup

# Copy binary from builder
COPY --from=builder /build/dbbackup /usr/local/bin/dbbackup
RUN chmod +x /usr/local/bin/dbbackup

# Create backup directory
RUN mkdir -p /backups && chown dbbackup:dbbackup /backups

# Set working directory
WORKDIR /backups

# Switch to non-root user
USER dbbackup

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/dbbackup"]

# Default command shows help
CMD ["--help"]

# Labels
LABEL maintainer="UUXO"
LABEL version="1.0"
LABEL description="Professional database backup tool for PostgreSQL, MySQL, and MariaDB"
