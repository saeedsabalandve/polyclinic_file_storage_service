package model

import (
    "time"
)

// Tenant represents a multi-tenant configuration
type Tenant struct {
    ID              string    `json:"id" db:"id"`
    Name            string    `json:"name" db:"name"`
    Slug            string    `json:"slug" db:"slug"`
    Status          string    `json:"status" db:"status"`
    
    // Storage configuration
    BucketName      string    `json:"bucket_name" db:"bucket_name"`
    StorageQuota    int64     `json:"storage_quota" db:"storage_quota"` // in bytes
    StorageUsed     int64     `json:"storage_used" db:"storage_used"`   // in bytes
    MaxFileSize     int64     `json:"max_file_size" db:"max_file_size"` // in bytes
    
    // Features
    AllowedTypes    []string  `json:"allowed_types" db:"allowed_types"`
    EnableEncryption bool     `json:"enable_encryption" db:"enable_encryption"`
    EncryptionType  string    `json:"encryption_type" db:"encryption_type"`
    KMSKeyARN       string    `json:"kms_key_arn,omitempty" db:"kms_key_arn"`
    
    // Retention policy
    RetentionDays   int       `json:"retention_days" db:"retention_days"`
    AutoArchiveDays int       `json:"auto_archive_days" db:"auto_archive_days"`
    
    // Backup configuration
    BackupEnabled   bool      `json:"backup_enabled" db:"backup_enabled"`
    BackupFrequency string    `json:"backup_frequency" db:"backup_frequency"`
    BackupRetention int       `json:"backup_retention" db:"backup_retention"` // days
    
    // CDN configuration
    CDNEnabled      bool      `json:"cdn_enabled" db:"cdn_enabled"`
    CDNDomain       string    `json:"cdn_domain,omitempty" db:"cdn_domain"`
    
    // CORS configuration
    CORSOrigins     []string  `json:"cors_origins" db:"cors_origins"`
    
    // Lifecycle rules
    LifecycleRules  JSONB     `json:"lifecycle_rules" db:"lifecycle_rules"`
    
    // Metadata
    CreatedBy       string    `json:"created_by" db:"created_by"`
    UpdatedBy       string    `json:"updated_by" db:"updated_by"`
    CreatedAt       time.Time `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// TenantConfig represents tenant-specific configuration
type TenantConfig struct {
    TenantID        string            `json:"tenant_id"`
    Features        map[string]bool   `json:"features"`
    Limits          TenantLimits      `json:"limits"`
    Preferences     TenantPreferences `json:"preferences"`
}

// TenantLimits represents tenant storage limits
type TenantLimits struct {
    MaxStorageGB      int `json:"max_storage_gb"`
    MaxFileSizeMB     int `json:"max_file_size_mb"`
    MaxFilesPerUpload int `json:"max_files_per_upload"`
    MaxConcurrentUploads int `json:"max_concurrent_uploads"`
    RateLimitRPM      int `json:"rate_limit_rpm"`
}

// TenantPreferences represents tenant-specific preferences
type TenantPreferences struct {
    DefaultEncryption   bool   `json:"default_encryption"`
    CompressionEnabled  bool   `json:"compression_enabled"`
    AutoArchiveDays     int    `json:"auto_archive_days"`
    NotificationEmail   string `json:"notification_email"`
    PreferredRegion     string `json:"preferred_region"`
}

// TenantUsage represents storage usage statistics
type TenantUsage struct {
    TenantID         string    `json:"tenant_id"`
    TotalFiles       int       `json:"total_files"`
    TotalSize        int64     `json:"total_size"`
    QuotaSize        int64     `json:"quota_size"`
    UsagePercent     float64   `json:"usage_percent"`
    ActiveUploads    int       `json:"active_uploads"`
    DailyUploads     int       `json:"daily_uploads"`
    DailyDownloads   int       `json:"daily_downloads"`
    LastActivity     time.Time `json:"last_activity"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
    ID           string    `json:"id" db:"id"`
    TenantID     string    `json:"tenant_id" db:"tenant_id"`
    UserID       string    `json:"user_id" db:"user_id"`
    Action       string    `json:"action" db:"action"`
    ResourceType string    `json:"resource_type" db:"resource_type"`
    ResourceID   string    `json:"resource_id" db:"resource_id"`
    Details      JSONB     `json:"details" db:"details"`
    IPAddress    string    `json:"ip_address" db:"ip_address"`
    UserAgent    string    `json:"user_agent" db:"user_agent"`
    Status       string    `json:"status" db:"status"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// Tenant status constants
const (
    TenantStatusActive     = "active"
    TenantStatusInactive   = "inactive"
    TenantStatusSuspended  = "suspended"
    TenantStatusDeleted    = "deleted"
)
