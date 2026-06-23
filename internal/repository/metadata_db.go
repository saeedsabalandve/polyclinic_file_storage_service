package repository

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/polyclinic/file-storage-service/internal/model"
)

// MetadataRepository handles database operations for file metadata
type MetadataRepository struct {
    pool *pgxpool.Pool
}

// NewMetadataRepository creates a new MetadataRepository
func NewMetadataRepository(pool *pgxpool.Pool) *MetadataRepository {
    return &MetadataRepository{pool: pool}
}

// CreateFile creates a new file record
func (r *MetadataRepository) CreateFile(ctx context.Context, file *model.File) error {
    query := `
        INSERT INTO files (
            id, tenant_id, filename, original_name, bucket_name, object_key,
            size, content_type, md5_hash, sha256_hash, encryption_type, version_id,
            status, tags, metadata, uploaded_by, created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
        )
        RETURNING id`

    tagsJSON, _ := json.Marshal(file.Tags)
    metadataJSON, _ := json.Marshal(file.Metadata)

    err := r.pool.QueryRow(ctx, query,
        file.ID, file.TenantID, file.Filename, file.OriginalName,
        file.BucketName, file.ObjectKey, file.Size, file.ContentType,
        file.MD5Hash, file.SHA256Hash, file.EncryptionType, file.VersionID,
        file.Status, tagsJSON, metadataJSON, file.UploadedBy,
        file.CreatedAt, file.UpdatedAt,
    ).Scan(&file.ID)

    return err
}

// GetFileByID retrieves a file by ID
func (r *MetadataRepository) GetFileByID(ctx context.Context, fileID string) (*model.File, error) {
    query := `
        SELECT id, tenant_id, filename, original_name, bucket_name, object_key,
               size, content_type, md5_hash, sha256_hash, encryption_type, version_id,
               status, tags, metadata, uploaded_by, updated_by, created_at, updated_at, expires_at
        FROM files
        WHERE id = $1 AND status != 'deleted'`

    file := &model.File{}
    var tagsJSON, metadataJSON []byte
    var expiresAt sql.NullTime

    err := r.pool.QueryRow(ctx, query, fileID).Scan(
        &file.ID, &file.TenantID, &file.Filename, &file.OriginalName,
        &file.BucketName, &file.ObjectKey, &file.Size, &file.ContentType,
        &file.MD5Hash, &file.SHA256Hash, &file.EncryptionType, &file.VersionID,
        &file.Status, &tagsJSON, &metadataJSON, &file.UploadedBy, &file.UpdatedBy,
        &file.CreatedAt, &file.UpdatedAt, &expiresAt,
    )

    if err != nil {
        return nil, fmt.Errorf("file not found: %w", err)
    }

    // Parse JSON fields
    json.Unmarshal(tagsJSON, &file.Tags)
    json.Unmarshal(metadataJSON, &file.Metadata)

    if expiresAt.Valid {
        file.ExpiresAt = &expiresAt.Time
    }

    return file, nil
}

// UpdateFile updates a file record
func (r *MetadataRepository) UpdateFile(ctx context.Context, file *model.File) error {
    query := `
        UPDATE files
        SET filename = $1, original_name = $2, status = $3,
            tags = $4, metadata = $5, updated_by = $6, updated_at = $7, expires_at = $8
        WHERE id = $9 AND tenant_id = $10`

    tagsJSON, _ := json.Marshal(file.Tags)
    metadataJSON, _ := json.Marshal(file.Metadata)

    _, err := r.pool.Exec(ctx, query,
        file.Filename, file.OriginalName, file.Status,
        tagsJSON, metadataJSON, file.UpdatedBy, file.UpdatedAt, file.ExpiresAt,
        file.ID, file.TenantID,
    )

    return err
}

// DeleteFile deletes a file record (soft delete)
func (r *MetadataRepository) DeleteFile(ctx context.Context, fileID string) error {
    query := `
        UPDATE files
        SET status = 'deleted', updated_at = $1, expires_at = $2
        WHERE id = $3`

    _, err := r.pool.Exec(ctx, query, time.Now(), time.Now().AddDate(0, 0, 30), fileID)
    return err
}

// ListFiles returns a paginated list of files for a tenant
func (r *MetadataRepository) ListFiles(ctx context.Context, tenantID string, limit, offset int, filters map[string]interface{}) ([]model.File, int, error) {
    baseQuery := `FROM files WHERE tenant_id = $1 AND status = 'active'`
    args := []interface{}{tenantID}
    argPos := 2

    // Add filters
    if status, ok := filters["status"]; ok {
        baseQuery += fmt.Sprintf(` AND status = $%d`, argPos)
        args = append(args, status)
        argPos++
    }

    if studyType, ok := filters["study_type"]; ok {
        baseQuery += fmt.Sprintf(` AND metadata->>'study_type' = $%d`, argPos)
        args = append(args, studyType)
        argPos++
    }

    if patientID, ok := filters["patient_id"]; ok {
        baseQuery += fmt.Sprintf(` AND metadata->>'patient_id' = $%d`, argPos)
        args = append(args, patientID)
        argPos++
    }

    // Get total count
    var totalCount int
    countQuery := fmt.Sprintf("SELECT COUNT(*) %s", baseQuery)
    err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
    if err != nil {
        return nil, 0, err
    }

    // Get files
    dataQuery := fmt.Sprintf(`
        SELECT id, tenant_id, filename, original_name, bucket_name, object_key,
               size, content_type, status, tags, metadata, uploaded_by,
               created_at, updated_at
        %s
        ORDER BY created_at DESC
        LIMIT $%d OFFSET $%d`, baseQuery, argPos, argPos+1)

    args = append(args, limit, offset)

    rows, err := r.pool.Query(ctx, dataQuery, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var files []model.File
    for rows.Next() {
        var file model.File
        var tagsJSON, metadataJSON []byte

        err := rows.Scan(
            &file.ID, &file.TenantID, &file.Filename, &file.OriginalName,
            &file.BucketName, &file.ObjectKey, &file.Size, &file.ContentType,
            &file.Status, &tagsJSON, &metadataJSON, &file.UploadedBy,
            &file.CreatedAt, &file.UpdatedAt,
        )
        if err != nil {
            return nil, 0, err
        }

        json.Unmarshal(tagsJSON, &file.Tags)
        json.Unmarshal(metadataJSON, &file.Metadata)
        files = append(files, file)
    }

    return files, totalCount, nil
}

// SearchFiles performs full-text search on files
func (r *MetadataRepository) SearchFiles(ctx context.Context, tenantID, query string, limit, offset int) ([]model.File, int, error) {
    searchQuery := `
        SELECT id, tenant_id, filename, original_name, bucket_name, object_key,
               size, content_type, status, tags, metadata, uploaded_by,
               created_at, updated_at,
               COUNT(*) OVER() as total_count
        FROM files
        WHERE tenant_id = $1
          AND status = 'active'
          AND (
              original_name ILIKE $2
              OR filename ILIKE $2
              OR metadata::text ILIKE $2
              OR tags::text ILIKE $2
          )
        ORDER BY created_at DESC
        LIMIT $3 OFFSET $4`

    searchPattern := fmt.Sprintf("%%%s%%", query)

    rows, err := r.pool.Query(ctx, searchQuery, tenantID, searchPattern, limit, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var files []model.File
    var totalCount int

    for rows.Next() {
        var file model.File
        var tagsJSON, metadataJSON []byte

        err := rows.Scan(
            &file.ID, &file.TenantID, &file.Filename, &file.OriginalName,
            &file.BucketName, &file.ObjectKey, &file.Size, &file.ContentType,
            &file.Status, &tagsJSON, &metadataJSON, &file.UploadedBy,
            &file.CreatedAt, &file.UpdatedAt, &totalCount,
        )
        if err != nil {
            return nil, 0, err
        }

        json.Unmarshal(tagsJSON, &file.Tags)
        json.Unmarshal(metadataJSON, &file.Metadata)
        files = append(files, file)
    }

    return files, totalCount, nil
}

// SaveMultipartUpload saves a multipart upload session
func (r *MetadataRepository) SaveMultipartUpload(ctx context.Context, upload *model.MultipartUpload) error {
    query := `
        INSERT INTO multipart_uploads (
            id, upload_id, file_id, tenant_id, filename, bucket_name,
            object_key, part_count, total_size, parts, status, initiated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

    partsJSON, _ := json.Marshal(upload.Parts)

    _, err := r.pool.Exec(ctx, query,
        upload.UploadID, upload.UploadID, upload.FileID, upload.TenantID,
        upload.Filename, upload.Bucket, upload.Key, upload.PartCount,
        upload.TotalSize, partsJSON, upload.Status, upload.InitiatedAt,
    )

    return err
}

// GetMultipartUpload retrieves a multipart upload session
func (r *MetadataRepository) GetMultipartUpload(ctx context.Context, uploadID string) (*model.MultipartUpload, error) {
    query := `
        SELECT upload_id, file_id, tenant_id, filename, bucket_name,
               object_key, part_count, total_size, parts, status, initiated_at, completed_at
        FROM multipart_uploads
        WHERE upload_id = $1 AND status = 'in_progress'`

    upload := &model.MultipartUpload{}
    var partsJSON []byte
    var completedAt sql.NullTime

    err := r.pool.QueryRow(ctx, query, uploadID).Scan(
        &upload.UploadID, &upload.FileID, &upload.TenantID, &upload.Filename,
        &upload.Bucket, &upload.Key, &upload.PartCount, &upload.TotalSize,
        &partsJSON, &upload.Status, &upload.InitiatedAt, &completedAt,
    )

    if err != nil {
        return nil, err
    }

    json.Unmarshal(partsJSON, &upload.Parts)

    if completedAt.Valid {
        upload.CompletedAt = &completedAt.Time
    }

    return upload, nil
}

// UpdateMultipartUpload updates a multipart upload session
func (r *MetadataRepository) UpdateMultipartUpload(ctx context.Context, upload *model.MultipartUpload) error {
    query := `
        UPDATE multipart_uploads
        SET parts = $1, status = $2, completed_at = $3
        WHERE upload_id = $4`

    partsJSON, _ := json.Marshal(upload.Parts)

    _, err := r.pool.Exec(ctx, query, partsJSON, upload.Status, upload.CompletedAt, upload.UploadID)
    return err
}

// GetFileVersions returns version history for a file
func (r *MetadataRepository) GetFileVersions(ctx context.Context, fileID string) ([]model.File, error) {
    query := `
        SELECT fv.version_id, fv.size, fv.content_type, fv.md5_hash,
               fv.sha256_hash, fv.created_by, fv.created_at
        FROM file_versions fv
        WHERE fv.file_id = $1
        ORDER BY fv.created_at DESC`

    rows, err := r.pool.Query(ctx, query, fileID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var files []model.File
    for rows.Next() {
        var file model.File
        err := rows.Scan(
            &file.VersionID, &file.Size, &file.ContentType,
            &file.MD5Hash, &file.SHA256Hash, &file.UploadedBy,
            &file.CreatedAt,
        )
        if err != nil {
            return nil, err
        }
        files = append(files, file)
    }

    return files, nil
}

// GetFileStats returns file statistics for a tenant
func (r *MetadataRepository) GetFileStats(ctx context.Context, tenantID string) (map[string]interface{}, error) {
    query := `
        SELECT
            COUNT(*) as total_files,
            COALESCE(SUM(size), 0) as total_size,
            COUNT(CASE WHEN created_at > NOW() - INTERVAL '24 hours' THEN 1 END) as daily_uploads
        FROM files
        WHERE tenant_id = $1 AND status = 'active'`

    stats := make(map[string]interface{})
    var totalFiles int
    var totalSize int64
    var dailyUploads int

    err := r.pool.QueryRow(ctx, query, tenantID).Scan(&totalFiles, &totalSize, &dailyUploads)
    if err != nil {
        return nil, err
    }

    stats["total_files"] = totalFiles
    stats["total_size_bytes"] = totalSize
    stats["total_size_gb"] = float64(totalSize) / (1024 * 1024 * 1024)
    stats["daily_uploads"] = dailyUploads

    return stats, nil
}

// GetExpiredFiles returns files marked for deletion that have expired
func (r *MetadataRepository) GetExpiredFiles(ctx context.Context) ([]model.File, error) {
    query := `
        SELECT id, tenant_id, filename, original_name, bucket_name, object_key,
               size, content_type, status, metadata, uploaded_by, created_at, updated_at
        FROM files
        WHERE status = 'deleted'
          AND expires_at IS NOT NULL
          AND expires_at <= NOW()
        LIMIT 100`

    rows, err := r.pool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var files []model.File
    for rows.Next() {
        var file model.File
        var metadataJSON []byte

        err := rows.Scan(
            &file.ID, &file.TenantID, &file.Filename, &file.OriginalName,
            &file.BucketName, &file.ObjectKey, &file.Size, &file.ContentType,
            &file.Status, &metadataJSON, &file.UploadedBy,
            &file.CreatedAt, &file.UpdatedAt,
        )
        if err != nil {
            return nil, err
        }

        json.Unmarshal(metadataJSON, &file.Metadata)
        files = append(files, file)
    }

    return files, nil
}

// CreateAuditLog creates an audit log entry
func (r *MetadataRepository) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
    query := `
        INSERT INTO audit_logs (
            tenant_id, user_id, action, resource_type, resource_id,
            details, ip_address, user_agent, status, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

    detailsJSON, _ := json.Marshal(log.Details)

    _, err := r.pool.Exec(ctx, query,
        log.TenantID, log.UserID, log.Action, log.ResourceType,
        log.ResourceID, detailsJSON, log.IPAddress, log.UserAgent,
        log.Status, time.Now(),
    )

    return err
}
