package repository

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/polyclinic/file-storage-service/internal/model"
)

// TenantRepository handles database operations for tenants
type TenantRepository struct {
    pool *pgxpool.Pool
}

// NewTenantRepository creates a new TenantRepository
func NewTenantRepository(pool *pgxpool.Pool) *TenantRepository {
    return &TenantRepository{pool: pool}
}

// CreateTenant creates a new tenant
func (r *TenantRepository) CreateTenant(ctx context.Context, tenant *model.Tenant) error {
    query := `
        INSERT INTO tenants (
            id, name, slug, status, bucket_name, storage_quota,
            max_file_size, allowed_types, enable_encryption,
            encryption_type, kms_key_arn, retention_days,
            auto_archive_days, backup_enabled, backup_frequency,
            backup_retention, cdn_enabled, cdn_domain,
            cors_origins, lifecycle_rules, created_by, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
            $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23
        )`

    _, err := r.pool.Exec(ctx, query,
        tenant.ID, tenant.Name, tenant.Slug, tenant.Status,
        tenant.BucketName, tenant.StorageQuota, tenant.MaxFileSize,
        tenant.AllowedTypes, tenant.EnableEncryption,
        tenant.EncryptionType, tenant.KMSKeyARN, tenant.RetentionDays,
        tenant.AutoArchiveDays, tenant.BackupEnabled,
        tenant.BackupFrequency, tenant.BackupRetention,
        tenant.CDNEnabled, tenant.CDNDomain,
        tenant.CORSOrigins, tenant.LifecycleRules,
        tenant.CreatedBy, tenant.CreatedAt, tenant.UpdatedAt,
    )

    return err
}

// GetTenantByID retrieves a tenant by ID
func (r *TenantRepository) GetTenantByID(ctx context.Context, tenantID string) (*model.Tenant, error) {
    query := `
        SELECT id, name, slug, status, bucket_name, storage_quota,
               storage_used, max_file_size, allowed_types,
               enable_encryption, encryption_type, kms_key_arn,
               retention_days, auto_archive_days, backup_enabled,
               backup_frequency, backup_retention, cdn_enabled,
               cdn_domain, cors_origins, lifecycle_rules,
               created_by, updated_by, created_at, updated_at
        FROM tenants
        WHERE id = $1 AND status != 'deleted'`

    tenant := &model.Tenant{}
    err := r.pool.QueryRow(ctx, query, tenantID).Scan(
        &tenant.ID, &tenant.Name, &tenant.Slug, &tenant.Status,
        &tenant.BucketName, &tenant.StorageQuota, &tenant.StorageUsed,
        &tenant.MaxFileSize, &tenant.AllowedTypes,
        &tenant.EnableEncryption, &tenant.EncryptionType,
        &tenant.KMSKeyARN, &tenant.RetentionDays,
        &tenant.AutoArchiveDays, &tenant.BackupEnabled,
        &tenant.BackupFrequency, &tenant.BackupRetention,
        &tenant.CDNEnabled, &tenant.CDNDomain,
        &tenant.CORSOrigins, &tenant.LifecycleRules,
        &tenant.CreatedBy, &tenant.UpdatedBy,
        &tenant.CreatedAt, &tenant.UpdatedAt,
    )

    if err != nil {
        return nil, err
    }

    return tenant, nil
}

// UpdateTenant updates a tenant
func (r *TenantRepository) UpdateTenant(ctx context.Context, tenant *model.Tenant) error {
    query := `
        UPDATE tenants
        SET name = $1, status = $2, storage_quota = $3,
            max_file_size = $4, allowed_types = $5,
            enable_encryption = $6, encryption_type = $7,
            retention_days = $8, auto_archive_days = $9,
            backup_enabled = $10, backup_frequency = $11,
            cdn_enabled = $12, cors_origins = $13,
            lifecycle_rules = $14, updated_by = $15, updated_at = $16
        WHERE id = $17`

    _, err := r.pool.Exec(ctx, query,
        tenant.Name, tenant.Status, tenant.StorageQuota,
        tenant.MaxFileSize, tenant.AllowedTypes,
        tenant.EnableEncryption, tenant.EncryptionType,
        tenant.RetentionDays, tenant.AutoArchiveDays,
        tenant.BackupEnabled, tenant.BackupFrequency,
        tenant.CDNEnabled, tenant.CORSOrigins,
        tenant.LifecycleRules, tenant.UpdatedBy,
        tenant.UpdatedAt, tenant.ID,
    )

    return err
}

// ListTenants returns a paginated list of tenants
func (r *TenantRepository) ListTenants(ctx context.Context, limit, offset int) ([]model.Tenant, int, error) {
    countQuery := `SELECT COUNT(*) FROM tenants WHERE status != 'deleted'`
    var totalCount int
    if err := r.pool.QueryRow(ctx, countQuery).Scan(&totalCount); err != nil {
        return nil, 0, err
    }

    query := `
        SELECT id, name, slug, status, bucket_name, storage_quota,
               storage_used, max_file_size, created_at, updated_at
        FROM tenants
        WHERE status != 'deleted'
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2`

    rows, err := r.pool.Query(ctx, query, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var tenants []model.Tenant
    for rows.Next() {
        var tenant model.Tenant
        err := rows.Scan(
            &tenant.ID, &tenant.Name, &tenant.Slug,
            &tenant.Status, &tenant.BucketName,
            &tenant.StorageQuota, &tenant.StorageUsed,
            &tenant.MaxFileSize, &tenant.CreatedAt,
            &tenant.UpdatedAt,
        )
        if err != nil {
            return nil, 0, err
        }
        tenants = append(tenants, tenant)
    }

    return tenants, totalCount, nil
}

// GetTenantUsage returns storage usage for a tenant
func (r *TenantRepository) GetTenantUsage(ctx context.Context, tenantID string) (*model.TenantUsage, error) {
    query := `
        SELECT
            t.id as tenant_id,
            COUNT(f.id) as total_files,
            COALESCE(SUM(f.size), 0) as total_size,
            t.storage_quota as quota_size,
            COUNT(CASE WHEN f.created_at > NOW() - INTERVAL '24 hours' THEN 1 END) as daily_uploads,
            MAX(f.created_at) as last_activity
        FROM tenants t
        LEFT JOIN files f ON t.id = f.tenant_id AND f.status = 'active'
        WHERE t.id = $1
        GROUP BY t.id, t.storage_quota`

    usage := &model.TenantUsage{}
    var lastActivity *time.Time

    err := r.pool.QueryRow(ctx, query, tenantID).Scan(
        &usage.TenantID, &usage.TotalFiles,
        &usage.TotalSize, &usage.QuotaSize,
        &usage.DailyUploads, &lastActivity,
    )

    if err != nil {
        return nil, err
    }

    if lastActivity != nil {
        usage.LastActivity = *lastActivity
    }

    return usage, nil
}
