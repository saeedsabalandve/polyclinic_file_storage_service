package service

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/repository"
)

// MetadataService handles file metadata operations
type MetadataService struct {
    metadataRepo *repository.MetadataRepository
    redisClient  *repository.RedisClient
    logger       *zap.Logger
}

// NewMetadataService creates a new MetadataService
func NewMetadataService(metadataRepo *repository.MetadataRepository, redisClient *repository.RedisClient, logger *zap.Logger) *MetadataService {
    return &MetadataService{
        metadataRepo: metadataRepo,
        redisClient:  redisClient,
        logger:       logger,
    }
}

// CreateFileRecord creates a new file record in database
func (s *MetadataService) CreateFileRecord(ctx context.Context, file *model.File) error {
    if err := s.metadataRepo.CreateFile(ctx, file); err != nil {
        s.logger.Error("Failed to create file record", zap.Error(err))
        return err
    }

    // Invalidate cache if needed
    if s.redisClient != nil {
        s.invalidateFileCache(ctx, file.TenantID)
    }

    return nil
}

// GetFileRecord retrieves file metadata from database
func (s *MetadataService) GetFileRecord(ctx context.Context, fileID string) (*model.File, error) {
    // Try cache first
    if s.redisClient != nil {
        cached, err := s.getCachedFile(ctx, fileID)
        if err == nil && cached != nil {
            return cached, nil
        }
    }

    // Get from database
    file, err := s.metadataRepo.GetFileByID(ctx, fileID)
    if err != nil {
        return nil, err
    }

    // Cache the result
    if s.redisClient != nil {
        s.cacheFile(ctx, file)
    }

    return file, nil
}

// UpdateFileRecord updates file metadata
func (s *MetadataService) UpdateFileRecord(ctx context.Context, file *model.File) error {
    if err := s.metadataRepo.UpdateFile(ctx, file); err != nil {
        s.logger.Error("Failed to update file record", zap.Error(err))
        return err
    }

    // Invalidate cache
    if s.redisClient != nil {
        s.invalidateFileCache(ctx, file.TenantID)
        s.deleteCachedFile(ctx, file.ID)
    }

    return nil
}

// DeleteFileRecord deletes file record from database
func (s *MetadataService) DeleteFileRecord(ctx context.Context, fileID string) error {
    if err := s.metadataRepo.DeleteFile(ctx, fileID); err != nil {
        s.logger.Error("Failed to delete file record", zap.Error(err))
        return err
    }

    // Invalidate cache
    if s.redisClient != nil {
        s.deleteCachedFile(ctx, fileID)
    }

    return nil
}

// ListFiles returns a paginated list of files for a tenant
func (s *MetadataService) ListFiles(ctx context.Context, tenantID string, page, pageSize int, filters map[string]interface{}) ([]model.File, int, error) {
    offset := (page - 1) * pageSize
    
    files, totalCount, err := s.metadataRepo.ListFiles(ctx, tenantID, pageSize, offset, filters)
    if err != nil {
        s.logger.Error("Failed to list files", zap.Error(err))
        return nil, 0, err
    }

    return files, totalCount, nil
}

// SearchFiles performs full-text search on files
func (s *MetadataService) SearchFiles(ctx context.Context, tenantID, query string, page, pageSize int) ([]model.File, int, error) {
    offset := (page - 1) * pageSize
    
    files, totalCount, err := s.metadataRepo.SearchFiles(ctx, tenantID, query, pageSize, offset)
    if err != nil {
        s.logger.Error("Failed to search files", zap.Error(err))
        return nil, 0, err
    }

    return files, totalCount, nil
}

// GetFileVersions returns version history for a file
func (s *MetadataService) GetFileVersions(ctx context.Context, fileID string) ([]model.File, error) {
    versions, err := s.metadataRepo.GetFileVersions(ctx, fileID)
    if err != nil {
        s.logger.Error("Failed to get file versions", zap.Error(err))
        return nil, err
    }

    return versions, nil
}

// GetFileStats returns file statistics for a tenant
func (s *MetadataService) GetFileStats(ctx context.Context, tenantID string) (map[string]interface{}, error) {
    stats, err := s.metadataRepo.GetFileStats(ctx, tenantID)
    if err != nil {
        s.logger.Error("Failed to get file stats", zap.Error(err))
        return nil, err
    }

    return stats, nil
}

// SaveMultipartUpload saves multipart upload session
func (s *MetadataService) SaveMultipartUpload(ctx context.Context, upload *model.MultipartUpload) error {
    return s.metadataRepo.SaveMultipartUpload(ctx, upload)
}

// GetMultipartUpload retrieves multipart upload session
func (s *MetadataService) GetMultipartUpload(ctx context.Context, uploadID string) (*model.MultipartUpload, error) {
    return s.metadataRepo.GetMultipartUpload(ctx, uploadID)
}

// UpdateMultipartUpload updates multipart upload session
func (s *MetadataService) UpdateMultipartUpload(ctx context.Context, upload *model.MultipartUpload) error {
    return s.metadataRepo.UpdateMultipartUpload(ctx, upload)
}

// GetExpiredFiles returns files marked for deletion that have expired
func (s *MetadataService) GetExpiredFiles(ctx context.Context) ([]model.File, error) {
    return s.metadataRepo.GetExpiredFiles(ctx)
}

// CreateAuditLog creates an audit log entry
func (s *MetadataService) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
    return s.metadataRepo.CreateAuditLog(ctx, log)
}

// Cache helper methods
func (s *MetadataService) cacheFile(ctx context.Context, file *model.File) error {
    key := fmt.Sprintf("file:%s", file.ID)
    data, err := json.Marshal(file)
    if err != nil {
        return err
    }
    return s.redisClient.Set(ctx, key, string(data), 5*time.Minute)
}

func (s *MetadataService) getCachedFile(ctx context.Context, fileID string) (*model.File, error) {
    key := fmt.Sprintf("file:%s", fileID)
    data, err := s.redisClient.Get(ctx, key)
    if err != nil {
        return nil, err
    }

    var file model.File
    if err := json.Unmarshal([]byte(data), &file); err != nil {
        return nil, err
    }

    return &file, nil
}

func (s *MetadataService) deleteCachedFile(ctx context.Context, fileID string) error {
    key := fmt.Sprintf("file:%s", fileID)
    return s.redisClient.Del(ctx, key)
}

func (s *MetadataService) invalidateFileCache(ctx context.Context, tenantID string) error {
    pattern := fmt.Sprintf("files:list:%s:*", tenantID)
    return s.redisClient.DelPattern(ctx, pattern)
}
