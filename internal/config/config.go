package config

import (
    "fmt"
    "os"
    "strconv"
    "strings"
    "time"
)

// Config holds all configuration for the service
type Config struct {
    // Server
    ServerPort     string
    Environment    string
    AllowedOrigins []string

    // AWS
    AWSRegion          string
    AWSAccessKeyID     string
    AWSSecretAccessKey string
    S3BucketPrefix     string
    S3Endpoint         string // For custom endpoints like MinIO
    S3UsePathStyle     bool
    S3PresignedExpiry  time.Duration

    // Database
    DatabaseURL string
    DBMaxConns  int
    DBMinConns  int

    // Redis
    RedisURL       string
    RedisPassword  string
    RedisDB        int

    // JWT
    JWTSecret          string
    JWTAccessTokenExp  time.Duration
    JWTRefreshTokenExp time.Duration

    // File limits
    MaxFileSize        int64
    AllowedExtensions  []string
    AllowedMimeTypes   []string
    MaxFilesPerRequest int

    // Upload
    UploadPartSize    int64
    UploadConcurrency int
    UploadTimeout     time.Duration

    // Rate Limiting
    RateLimitEnabled bool
    RateLimit        RateLimitConfig

    // Features
    EnableAuditLog    bool
    EnableVersioning  bool
    EnableCompression bool

    // Encryption
    EncryptionType   string // "SSE-S3", "SSE-KMS"
    KMSKeyID         string
    EnableEncryption bool

    // CDN
    CDNDomain    string
    EnableCDN    bool

    // Lifecycle
    LifecycleRules LifecycleConfig
}

type RateLimitConfig struct {
    RequestsPerMinute int
    BurstSize        int
    CleanupInterval  time.Duration
}

type LifecycleConfig struct {
    TransitionToIA      int // days
    TransitionToGlacier int // days
    ExpirationDays      int // days
    EnableLifecycle    bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
    cfg := &Config{
        ServerPort:         getEnv("SERVER_PORT", "8080"),
        Environment:        getEnv("ENVIRONMENT", "development"),
        AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
        AWSAccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
        AWSSecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
        S3BucketPrefix:     getEnv("S3_BUCKET_PREFIX", "polyclinic"),
        DatabaseURL:        getEnv("DATABASE_URL", ""),
        RedisURL:           os.Getenv("REDIS_URL"),
        JWTSecret:          getEnv("JWT_SECRET", ""),
        EncryptionType:     getEnv("ENCRYPTION_TYPE", "SSE-S3"),
        KMSKeyID:           os.Getenv("KMS_KEY_ID"),
    }

    // Parse durations
    presignedExpiry, err := time.ParseDuration(getEnv("S3_PRESIGNED_EXPIRY", "60m"))
    if err != nil {
        return nil, fmt.Errorf("invalid S3 presigned expiry: %w", err)
    }
    cfg.S3PresignedExpiry = presignedExpiry

    // Parse file size
    maxFileSizeMB, err := strconv.ParseInt(getEnv("MAX_FILE_SIZE", "5120"), 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid MAX_FILE_SIZE: %w", err)
    }
    cfg.MaxFileSize = maxFileSizeMB * 1024 * 1024 // Convert MB to bytes

    // Parse allowed extensions
    extensions := getEnv("ALLOWED_EXTENSIONS", "dcm,dicom,jpg,jpeg,png,pdf,doc,docx,zip")
    cfg.AllowedExtensions = strings.Split(extensions, ",")

    // Parse allowed MIME types
    mimeTypes := getEnv("ALLOWED_MIME_TYPES", "application/dicom,image/jpeg,image/png,application/pdf")
    cfg.AllowedMimeTypes = strings.Split(mimeTypes, ",")

    // Parse max files per request
    maxFiles, err := strconv.Atoi(getEnv("MAX_FILES_PER_REQUEST", "10"))
    if err != nil {
        return nil, fmt.Errorf("invalid MAX_FILES_PER_REQUEST: %w", err)
    }
    cfg.MaxFilesPerRequest = maxFiles

    // Parse upload part size (5MB default)
    partSizeMB, err := strconv.ParseInt(getEnv("UPLOAD_PART_SIZE", "5"), 10, 64)
    if err != nil {
        return nil, fmt.Errorf("invalid UPLOAD_PART_SIZE: %w", err)
    }
    cfg.UploadPartSize = partSizeMB * 1024 * 1024

    // Parse upload concurrency
    concurrency, err := strconv.Atoi(getEnv("UPLOAD_CONCURRENCY", "5"))
    if err != nil {
        return nil, fmt.Errorf("invalid UPLOAD_CONCURRENCY: %w", err)
    }
    cfg.UploadConcurrency = concurrency

    // Parse upload timeout
    uploadTimeout, err := time.ParseDuration(getEnv("UPLOAD_TIMEOUT", "30m"))
    if err != nil {
        return nil, fmt.Errorf("invalid UPLOAD_TIMEOUT: %w", err)
    }
    cfg.UploadTimeout = uploadTimeout

    // Parse rate limit
    rpm, err := strconv.Atoi(getEnv("RATE_LIMIT_RPM", "100"))
    if err != nil {
        return nil, fmt.Errorf("invalid RATE_LIMIT_RPM: %w", err)
    }
    cfg.RateLimit = RateLimitConfig{
        RequestsPerMinute: rpm,
        BurstSize:        5,
        CleanupInterval:  time.Minute,
    }

    // Feature flags
    cfg.EnableAuditLog = getEnv("ENABLE_AUDIT_LOG", "true") == "true"
    cfg.EnableVersioning = getEnv("ENABLE_VERSIONING", "true") == "true"
    cfg.EnableCompression = getEnv("ENABLE_COMPRESSION", "false") == "true"
    cfg.EnableEncryption = getEnv("ENABLE_ENCRYPTION", "true") == "true"
    cfg.EnableCDN = getEnv("ENABLE_CDN", "false") == "true"

    // Parse lifecycle configuration
    iaDays, _ := strconv.Atoi(getEnv("LIFECYCLE_IA_DAYS", "30"))
    glacierDays, _ := strconv.Atoi(getEnv("LIFECYCLE_GLACIER_DAYS", "90"))
    expirationDays, _ := strconv.Atoi(getEnv("LIFECYCLE_EXPIRATION_DAYS", "365"))
    
    cfg.LifecycleRules = LifecycleConfig{
        TransitionToIA:      iaDays,
        TransitionToGlacier: glacierDays,
        ExpirationDays:      expirationDays,
        EnableLifecycle:     getEnv("ENABLE_LIFECYCLE", "true") == "true",
    }

    // Validate required configurations
    if cfg.Environment == "production" {
        if cfg.JWTSecret == "" {
            return nil, fmt.Errorf("JWT_SECRET is required in production")
        }
        if cfg.DatabaseURL == "" {
            return nil, fmt.Errorf("DATABASE_URL is required in production")
        }
    }

    return cfg, nil
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
