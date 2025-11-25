package security

import (
	"fmt"
	"os"
	"runtime"

	"dbbackup/internal/logger"
)

// PrivilegeChecker checks for elevated privileges
type PrivilegeChecker struct {
	log logger.Logger
}

// NewPrivilegeChecker creates a new privilege checker
func NewPrivilegeChecker(log logger.Logger) *PrivilegeChecker {
	return &PrivilegeChecker{
		log: log,
	}
}

// CheckAndWarn checks if running with elevated privileges and warns
func (pc *PrivilegeChecker) CheckAndWarn(allowRoot bool) error {
	isRoot, user := pc.isRunningAsRoot()
	
	if isRoot {
		pc.log.Warn("⚠️  Running with elevated privileges (root/Administrator)")
		pc.log.Warn("Security recommendation: Create a dedicated backup user with minimal privileges")
		
		if !allowRoot {
			return fmt.Errorf("running as root is not recommended, use --allow-root to override")
		}
		
		pc.log.Warn("Proceeding with root privileges (--allow-root specified)")
	} else {
		pc.log.Debug("Running as non-privileged user", "user", user)
	}
	
	return nil
}

// isRunningAsRoot checks if current process has root/admin privileges
func (pc *PrivilegeChecker) isRunningAsRoot() (bool, string) {
	if runtime.GOOS == "windows" {
		return pc.isWindowsAdmin()
	}
	return pc.isUnixRoot()
}

// isUnixRoot checks for root on Unix-like systems
func (pc *PrivilegeChecker) isUnixRoot() (bool, string) {
	uid := os.Getuid()
	user := GetCurrentUser()
	
	isRoot := uid == 0 || user == "root"
	return isRoot, user
}

// isWindowsAdmin checks for Administrator on Windows
func (pc *PrivilegeChecker) isWindowsAdmin() (bool, string) {
	// Check if running as Administrator on Windows
	// This is a simplified check - full implementation would use Windows API
	user := GetCurrentUser()
	
	// Common admin user patterns on Windows
	isAdmin := user == "Administrator" || user == "SYSTEM"
	
	return isAdmin, user
}

// GetRecommendedUser returns recommended non-privileged username
func (pc *PrivilegeChecker) GetRecommendedUser() string {
	if runtime.GOOS == "windows" {
		return "BackupUser"
	}
	return "dbbackup"
}

// GetSecurityRecommendations returns security best practices
func (pc *PrivilegeChecker) GetSecurityRecommendations() []string {
	recommendations := []string{
		"Create a dedicated backup user with minimal database privileges",
		"Grant only necessary permissions (SELECT, LOCK TABLES for MySQL)",
		"Use connection strings instead of environment variables in production",
		"Store credentials in secure credential management systems",
		"Enable SSL/TLS for database connections",
		"Restrict backup directory permissions (chmod 700)",
		"Regularly rotate database passwords",
		"Monitor audit logs for unauthorized access attempts",
	}
	
	if runtime.GOOS != "windows" {
		recommendations = append(recommendations,
			fmt.Sprintf("Run as non-root user: sudo -u %s dbbackup ...", pc.GetRecommendedUser()))
	}
	
	return recommendations
}
