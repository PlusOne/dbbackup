package cloud

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// CloudURI represents a parsed cloud storage URI
type CloudURI struct {
	Provider string // "s3", "minio", "azure", "gcs", "b2"
	Bucket   string // Bucket or container name
	Path     string // Path within bucket (without leading /)
	Region   string // Region (optional, extracted from host)
	Endpoint string // Custom endpoint (for MinIO, etc)
	FullURI  string // Original URI string
}

// ParseCloudURI parses a cloud storage URI like s3://bucket/path/file.dump
// Supported formats:
//   - s3://bucket/path/file.dump
//   - s3://bucket.s3.region.amazonaws.com/path/file.dump
//   - minio://bucket/path/file.dump
//   - azure://container/path/file.dump
//   - gs://bucket/path/file.dump (Google Cloud Storage)
//   - b2://bucket/path/file.dump (Backblaze B2)
func ParseCloudURI(uri string) (*CloudURI, error) {
	if uri == "" {
		return nil, fmt.Errorf("URI cannot be empty")
	}

	// Parse URL
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	// Extract provider from scheme
	provider := strings.ToLower(parsed.Scheme)
	if provider == "" {
		return nil, fmt.Errorf("URI must have a scheme (e.g., s3://)")
	}

	// Validate provider
	validProviders := map[string]bool{
		"s3":    true,
		"minio": true,
		"azure": true,
		"gs":    true,
		"gcs":   true,
		"b2":    true,
	}
	if !validProviders[provider] {
		return nil, fmt.Errorf("unsupported provider: %s (supported: s3, minio, azure, gs, gcs, b2)", provider)
	}

	// Normalize provider names
	if provider == "gcs" {
		provider = "gs"
	}

	// Extract bucket and path
	bucket := parsed.Host
	if bucket == "" {
		return nil, fmt.Errorf("URI must specify a bucket (e.g., s3://bucket/path)")
	}

	// Extract region from AWS S3 hostname if present
	// Format: bucket.s3.region.amazonaws.com or bucket.s3-region.amazonaws.com
	var region string
	var endpoint string

	if strings.Contains(bucket, ".amazonaws.com") {
		parts := strings.Split(bucket, ".")
		if len(parts) >= 3 {
			// Extract bucket name (first part)
			bucket = parts[0]
			
			// Extract region if present
			// bucket.s3.us-west-2.amazonaws.com -> us-west-2
			// bucket.s3-us-west-2.amazonaws.com -> us-west-2
			for i, part := range parts {
				if part == "s3" && i+1 < len(parts) && parts[i+1] != "amazonaws" {
					region = parts[i+1]
					break
				}
				if strings.HasPrefix(part, "s3-") {
					region = strings.TrimPrefix(part, "s3-")
					break
				}
			}
		}
	}

	// For MinIO and custom endpoints, preserve the host as endpoint
	if provider == "minio" || (provider == "s3" && !strings.Contains(bucket, "amazonaws.com")) {
		// If it looks like a custom endpoint (has dots), preserve it
		if strings.Contains(bucket, ".") && !strings.Contains(bucket, "amazonaws.com") {
			endpoint = bucket
			// Try to extract bucket from path
			trimmedPath := strings.TrimPrefix(parsed.Path, "/")
			pathParts := strings.SplitN(trimmedPath, "/", 2)
			if len(pathParts) > 0 && pathParts[0] != "" {
				bucket = pathParts[0]
				if len(pathParts) > 1 {
					parsed.Path = "/" + pathParts[1]
				} else {
					parsed.Path = "/"
				}
			}
		}
	}

	// Clean up path (remove leading slash)
	filepath := strings.TrimPrefix(parsed.Path, "/")

	return &CloudURI{
		Provider: provider,
		Bucket:   bucket,
		Path:     filepath,
		Region:   region,
		Endpoint: endpoint,
		FullURI:  uri,
	}, nil
}

// IsCloudURI checks if a string looks like a cloud storage URI
func IsCloudURI(s string) bool {
	s = strings.ToLower(s)
	return strings.HasPrefix(s, "s3://") ||
		strings.HasPrefix(s, "minio://") ||
		strings.HasPrefix(s, "azure://") ||
		strings.HasPrefix(s, "gs://") ||
		strings.HasPrefix(s, "gcs://") ||
		strings.HasPrefix(s, "b2://")
}

// String returns the string representation of the URI
func (u *CloudURI) String() string {
	return u.FullURI
}

// BaseName returns the filename without path
func (u *CloudURI) BaseName() string {
	return path.Base(u.Path)
}

// Dir returns the directory path without filename
func (u *CloudURI) Dir() string {
	return path.Dir(u.Path)
}

// Join appends path elements to the URI path
func (u *CloudURI) Join(elem ...string) string {
	newPath := u.Path
	for _, e := range elem {
		newPath = path.Join(newPath, e)
	}
	return fmt.Sprintf("%s://%s/%s", u.Provider, u.Bucket, newPath)
}

// ToConfig converts a CloudURI to a cloud.Config
func (u *CloudURI) ToConfig() *Config {
	cfg := &Config{
		Provider: u.Provider,
		Bucket:   u.Bucket,
		Prefix:   u.Dir(), // Use directory part as prefix
	}

	// Set region if available
	if u.Region != "" {
		cfg.Region = u.Region
	}

	// Set endpoint if available (for MinIO, etc)
	if u.Endpoint != "" {
		cfg.Endpoint = u.Endpoint
	}

	// Provider-specific settings
	switch u.Provider {
	case "minio":
		cfg.PathStyle = true
	case "b2":
		cfg.PathStyle = true
	}

	return cfg
}

// BuildRemotePath constructs the full remote path for a file
func (u *CloudURI) BuildRemotePath(filename string) string {
	if u.Path == "" || u.Path == "." {
		return filename
	}
	return path.Join(u.Path, filename)
}
