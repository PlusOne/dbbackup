package restore

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"

	"dbbackup/internal/database"
)

// VersionInfo holds PostgreSQL version information
type VersionInfo struct {
	Major int
	Minor int
	Full  string
}

// ParsePostgreSQLVersion extracts major and minor version from version string
// Example: "PostgreSQL 17.7 on x86_64-redhat-linux-gnu..." -> Major: 17, Minor: 7
func ParsePostgreSQLVersion(versionStr string) (*VersionInfo, error) {
	// Match patterns like "PostgreSQL 17.7", "PostgreSQL 13.11", "PostgreSQL 10.23"
	re := regexp.MustCompile(`PostgreSQL\s+(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(versionStr)
	
	if len(matches) < 3 {
		return nil, fmt.Errorf("could not parse PostgreSQL version from: %s", versionStr)
	}
	
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", matches[1])
	}
	
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", matches[2])
	}
	
	return &VersionInfo{
		Major: major,
		Minor: minor,
		Full:  versionStr,
	}, nil
}

// GetDumpFileVersion extracts the PostgreSQL version from a dump file
// Uses pg_restore -l to read the dump metadata
func GetDumpFileVersion(dumpPath string) (*VersionInfo, error) {
	cmd := exec.Command("pg_restore", "-l", dumpPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to read dump file metadata: %w (output: %s)", err, string(output))
	}
	
	// Look for "Dumped from database version: X.Y.Z" in output
	re := regexp.MustCompile(`Dumped from database version:\s+(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(string(output))
	
	if len(matches) < 3 {
		// Try alternate format in some dumps
		re = regexp.MustCompile(`PostgreSQL database dump.*(\d+)\.(\d+)`)
		matches = re.FindStringSubmatch(string(output))
	}
	
	if len(matches) < 3 {
		return nil, fmt.Errorf("could not find version information in dump file")
	}
	
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	
	return &VersionInfo{
		Major: major,
		Minor: minor,
		Full:  fmt.Sprintf("PostgreSQL %d.%d", major, minor),
	}, nil
}

// CheckVersionCompatibility checks if restoring from source version to target version is safe
func CheckVersionCompatibility(sourceVer, targetVer *VersionInfo) *VersionCompatibilityResult {
	result := &VersionCompatibilityResult{
		Compatible: true,
		SourceVersion: sourceVer,
		TargetVersion: targetVer,
	}
	
	// Same major version - always compatible
	if sourceVer.Major == targetVer.Major {
		result.Level = CompatibilityLevelSafe
		result.Message = "Same major version - fully compatible"
		return result
	}
	
	// Downgrade - not supported
	if sourceVer.Major > targetVer.Major {
		result.Compatible = false
		result.Level = CompatibilityLevelUnsupported
		result.Message = fmt.Sprintf("Downgrade from PostgreSQL %d to %d is not supported", sourceVer.Major, targetVer.Major)
		result.Warnings = append(result.Warnings, "Database downgrades require pg_dump from the target version")
		return result
	}
	
	// Upgrade - check how many major versions
	versionDiff := targetVer.Major - sourceVer.Major
	
	if versionDiff == 1 {
		// One major version upgrade - generally safe
		result.Level = CompatibilityLevelSafe
		result.Message = fmt.Sprintf("Upgrading from PostgreSQL %d to %d - officially supported", sourceVer.Major, targetVer.Major)
	} else if versionDiff <= 3 {
		// 2-3 major versions - should work but review release notes
		result.Level = CompatibilityLevelWarning
		result.Message = fmt.Sprintf("Upgrading from PostgreSQL %d to %d - supported but review release notes", sourceVer.Major, targetVer.Major)
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("You are jumping %d major versions - some features may have changed", versionDiff))
		result.Warnings = append(result.Warnings,
			"Review release notes for deprecated features or behavior changes")
	} else {
		// 4+ major versions - high risk
		result.Level = CompatibilityLevelRisky
		result.Message = fmt.Sprintf("Upgrading from PostgreSQL %d to %d - large version jump", sourceVer.Major, targetVer.Major)
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("WARNING: Jumping %d major versions may encounter compatibility issues", versionDiff))
		result.Warnings = append(result.Warnings,
			"Deprecated features from PostgreSQL "+strconv.Itoa(sourceVer.Major)+" may not exist in "+strconv.Itoa(targetVer.Major))
		result.Warnings = append(result.Warnings,
			"Extensions may need updates or may be incompatible")
		result.Warnings = append(result.Warnings,
			"Test thoroughly in a non-production environment first")
		result.Recommendations = append(result.Recommendations,
			"Consider using --schema-only first to validate schema compatibility")
		result.Recommendations = append(result.Recommendations,
			"Review PostgreSQL release notes for versions "+strconv.Itoa(sourceVer.Major)+" through "+strconv.Itoa(targetVer.Major))
	}
	
	// Add general upgrade advice
	if versionDiff > 0 {
		result.Recommendations = append(result.Recommendations,
			"Run ANALYZE on all tables after restore for optimal query performance")
	}
	
	return result
}

// CompatibilityLevel indicates the risk level of version compatibility
type CompatibilityLevel int

const (
	CompatibilityLevelSafe CompatibilityLevel = iota
	CompatibilityLevelWarning
	CompatibilityLevelRisky
	CompatibilityLevelUnsupported
)

func (c CompatibilityLevel) String() string {
	switch c {
	case CompatibilityLevelSafe:
		return "SAFE"
	case CompatibilityLevelWarning:
		return "WARNING"
	case CompatibilityLevelRisky:
		return "RISKY"
	case CompatibilityLevelUnsupported:
		return "UNSUPPORTED"
	default:
		return "UNKNOWN"
	}
}

// VersionCompatibilityResult contains the result of version compatibility check
type VersionCompatibilityResult struct {
	Compatible      bool
	Level           CompatibilityLevel
	SourceVersion   *VersionInfo
	TargetVersion   *VersionInfo
	Message         string
	Warnings        []string
	Recommendations []string
}

// CheckRestoreVersionCompatibility performs version check for a restore operation
func (e *Engine) CheckRestoreVersionCompatibility(ctx context.Context, dumpPath string) (*VersionCompatibilityResult, error) {
	// Get dump file version
	dumpVer, err := GetDumpFileVersion(dumpPath)
	if err != nil {
		// Not critical if we can't read version - continue with warning
		e.log.Warn("Could not determine dump file version", "error", err)
		return nil, nil
	}
	
	// Get target database version
	targetVerStr, err := e.db.GetVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get target database version: %w", err)
	}
	
	targetVer, err := ParsePostgreSQLVersion(targetVerStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target version: %w", err)
	}
	
	// Check compatibility
	result := CheckVersionCompatibility(dumpVer, targetVer)
	
	// Log the results
	e.log.Info("Version compatibility check",
		"source", dumpVer.Full,
		"target", targetVer.Full,
		"level", result.Level.String())
	
	if len(result.Warnings) > 0 {
		for _, warning := range result.Warnings {
			e.log.Warn(warning)
		}
	}
	
	return result, nil
}

// ValidatePostgreSQLDatabase ensures we're working with a PostgreSQL database
func ValidatePostgreSQLDatabase(db database.Database) error {
	// Type assertion to check if it's PostgreSQL
	switch db.(type) {
	case *database.PostgreSQL:
		return nil
	default:
		return fmt.Errorf("version compatibility checks only supported for PostgreSQL")
	}
}
