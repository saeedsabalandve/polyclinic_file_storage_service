package service

import (
    "context"
    "time"

    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/config"
    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/repository"
)

// TenantService handles tenant management operations
type TenantService struct {
    tenantRepo *repository.TenantRepository
    config     *config.Config
    logger     *zap.Logger
}

// NewTenantService creates a new TenantService
func NewTenantService(tenantRepo *repository.TenantRepository, cfg *config.Config, logger *zap.Logger) *TenantService {
    return &TenantService{
        tenantRepo: tenantRepo,
        config:     cfg,
        logger:     logger,
    }
}

// CreateTenant creates a new tenant
func (s *TenantService) CreateTenant(ctx context.Context, tenant *model.Tenant) error {
    // Set default values
    if tenant.StorageQuota == 0 {
        tenant.StorageQuota = 1099511627776 // 1TB default
    }
    if tenant.MaxFileSize == 0 {
        tenant.MaxFileSize = 2147483648 // 2GB default
    }
    if tenant.RetentionDays == 0 {
        tenant.RetentionDays = 365
    }
    if tenant.AutoArchiveDays == 0 {
        tenant.AutoArchiveDays = 90
    }
    if tenant.EncryptionType == "" {
        tenant.EncryptionType = "SSE-S3"
    }

    tenant.Status = model.TenantStatusActive
    tenant.CreatedAt = time.Now()
    tenant.UpdatedAt = time.Now()

    if err := s.tenantRepo.CreateTenant(ctx, tenant); err != nil {
        s.logger.Error("Failed to create tenant", zap.Error(err))
        return err
    }

    s.logger.Info("Tenant created successfully",
        zap.String("tenant_id", tenant.ID),
        zap.String("tenant_name", tenant.Name))

    return nil
}

// GetTenant retrieves tenant information
func (s *TenantService) GetTenant(ctx context.Context, tenantID string) (*model.Tenant, error) {
    tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
    if err != nil {
        s.logger.Error("Failed to get tenant", zap.Error(err))
        return nil, err
    }
    return tenant, nil
}

// UpdateTenant updates tenant information
func (s *TenantService) UpdateTenant(ctx context.Context, tenant *model.Tenant) error {
    tenant.UpdatedAt = time.Now()
    
    if err := s.tenantRepo.UpdateTenant(ctx, tenant); err != nil {
        s.logger.Error("Failed to update tenant", zap.Error(err))
        return err
    }

    return nil
}

// DeleteTenant soft deletes a tenant
func (s *TenantService) DeleteTenant(ctx context.Context, tenantID string) error {
    tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
    if err != nil {
        return err
    }

    tenant.Status = model.TenantStatusDeleted
    tenant.UpdatedAt = time.Now()

    if err := s.tenantRepo.UpdateTenant(ctx, tenant); err != nil {
        s.logger.Error("Failed to delete tenant", zap.Error(err))
        return err
    }

    return nil
}

// ListTenants returns a list of all tenants
func (s *TenantService) ListTenants(ctx context.Context, page, pageSize int) ([]model.Tenant, int, error) {
    offset := (page - 1) * pageSize
    tenants, totalCount, err := s.tenantRepo.ListTenants(ctx, pageSize, offset)
    if err != nil {
        s.logger.Error("Failed to list tenants", zap.Error(err))
        return nil, 0, err
    }
    return tenants, totalCount, nil
}

// GetTenantUsage returns storage usage statistics for a tenant
func (s *TenantService) GetTenantUsage(ctx context.Context, tenantID string) (*model.TenantUsage, error) {
    usage, err := s.tenantRepo.GetTenantUsage(ctx, tenantID)
    if err != nil {
        s.logger.Error("Failed to get tenant usage", zap.Error(err))
        return nil, err
    }

    // Calculate usage percentage
    if usage.QuotaSize > 0 {
        usage.UsagePercent = float64(usage.TotalSize) / float64(usage.QuotaSize) * 100
    }

    return usage, nil
}

// CheckQuota checks if tenant has enough storage quota
func (s *TenantService) CheckQuota(ctx context.Context, tenantID string, additionalSize int64) (bool, error) {
    usage, err := s.GetTenantUsage(ctx, tenantID)
    if err != nil {
        return false, err
    }

    return (usage.TotalSize + additionalSize) <= usage.QuotaSize, nil
}

// SuspendTenant suspends a tenant
func (s *TenantService) SuspendTenant(ctx context.Context, tenantID string) error {
    tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
    if err != nil {
        return err
    }

    tenant.Status = model.TenantStatusSuspended
    tenant.UpdatedAt = time.Now()

    return s.tenantRepo.UpdateTenant(ctx, tenant)
}

// ActivateTenant activates a suspended tenant
func (s *TenantService) ActivateTenant(ctx context.Context, tenantID string) error {
    tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
    if err != nil {
        return err
    }

    tenant.Status = model.TenantStatusActive
    tenant.UpdatedAt = time.Now()

    return s.tenantRepo.UpdateTenant(ctx, tenant)
}

// GetTenantConfig returns tenant-specific configuration
func (s *TenantService) GetTenantConfig(ctx context.Context, tenantID string) (*model.TenantConfig, error) {
    tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
    if err != nil {
        return nil, err
    }

    config := &model.TenantConfig{
        TenantID: tenantID,
        Features: map[string]bool{
            "encryption":   tenant.EnableEncryption,
            "backup":       tenant.BackupEnabled,
            "cdn":          tenant.CDNEnabled,
            "versioning":   true,
            "compression":  false,
        },
        Limits: model.TenantLimits{
            MaxStorageGB:         int(tenant.StorageQuota / (1024 * 1024 * 1024)),
            MaxFileSizeMB:        int(tenant.MaxFileSize / (1024 * 1024)),
            MaxFilesPerUpload:    10,
            MaxConcurrentUploads: 5,
            RateLimitRPM:         100,
        },
        Preferences: model.TenantPreferences{
            DefaultEncryption:   tenant.EnableEncryption,
            CompressionEnabled:  false,
            AutoArchiveDays:     tenant.AutoArchiveDays,
            PreferredRegion:     s.config.AWSRegion,
        },
    }

    return config, nil
}
