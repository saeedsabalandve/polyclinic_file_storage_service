package service

import (
    "context"
    "fmt"
    "io"
    "strings"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/s3/types"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/config"
    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/repository"
)

// StorageService handles file storage operations
type StorageService struct {
    s3Client *repository.S3Client
    config   *config.Config
    logger   *zap.Logger
}

// NewStorageService creates a new StorageService
func NewStorageService(s3Client *repository.S3Client, cfg *config.Config, logger *zap.Logger) *StorageService {
    return &StorageService{
        s3Client: s3Client,
        config:   cfg,
        logger:   logger,
    }
}

// UploadFile uploads a file to S3
func (s *StorageService) UploadFile(ctx context.Context, tenantID, objectKey string, reader io.Reader, contentType string, metadata map[string]string) (*s3.PutObjectOutput, error) {
    // Add service metadata
    if metadata == nil {
        metadata = make(map[string]string)
    }
    metadata["uploaded-by-service"] = "polyclinic-file-storage"
    metadata["upload-timestamp"] = time.Now().Format(time.RFC3339)

    result, err := s.s3Client.UploadFile(ctx, tenantID, objectKey, reader, contentType, metadata)
    if err != nil {
        s.logger.Error("Failed to upload file to S3",
            zap.String("tenant_id", tenantID),
            zap.String("object_key", objectKey),
            zap.Error(err))
        return nil, err
    }

    s.logger.Info("File uploaded successfully",
        zap.String("tenant_id", tenantID),
        zap.String("object_key", objectKey),
        zap.String("version_id", *result.VersionId))

    return result, nil
}

// DownloadFile downloads a file from S3
func (s *StorageService) DownloadFile(ctx context.Context, tenantID, objectKey string) (*s3.GetObjectOutput, error) {
    result, err := s.s3Client.DownloadFile(ctx, tenantID, objectKey)
    if err != nil {
        s.logger.Error("Failed to download file from S3",
            zap.String("tenant_id", tenantID),
            zap.String("object_key", objectKey),
            zap.Error(err))
        return nil, err
    }

    return result, nil
}

// DeleteFile deletes a file from S3
func (s *StorageService) DeleteFile(ctx context.Context, tenantID, objectKey string) error {
    err := s.s3Client.DeleteFile(ctx, tenantID, objectKey)
    if err != nil {
        s.logger.Error("Failed to delete file from S3",
            zap.String("tenant_id", tenantID),
            zap.String("object_key", objectKey),
            zap.Error(err))
        return err
    }

    s.logger.Info("File deleted successfully",
        zap.String("tenant_id", tenantID),
        zap.String("object_key", objectKey))

    return nil
}

// GeneratePresignedURL generates a pre-signed URL for download
func (s *StorageService) GeneratePresignedURL(ctx context.Context, tenantID, objectKey string, expiry time.Duration) (string, error) {
    url, err := s.s3Client.GeneratePresignedURL(ctx, tenantID, objectKey, expiry, "GET")
    if err != nil {
        s.logger.Error("Failed to generate presigned URL",
            zap.String("tenant_id", tenantID),
            zap.String("object_key", objectKey),
            zap.Error(err))
        return "", err
    }

    return url, nil
}

// GeneratePresignedUploadURL generates a pre-signed URL for upload
func (s *StorageService) GeneratePresignedUploadURL(ctx context.Context, tenantID, objectKey string, expiry time.Duration) (string, error) {
    url, err := s.s3Client.GeneratePresignedURL(ctx, tenantID, objectKey, expiry, "PUT")
    if err != nil {
        s.logger.Error("Failed to generate presigned upload URL",
            zap.String("tenant_id", tenantID),
            zap.String("object_key", objectKey),
            zap.Error(err))
        return "", err
    }

    return url, nil
}

// InitiateMultipartUpload initiates a multipart upload
func (s *StorageService) InitiateMultipartUpload(ctx context.Context, tenantID, objectKey, contentType string) (*s3.CreateMultipartUploadOutput, error) {
    result, err := s.s3Client.InitiateMultipartUpload(ctx, tenantID, objectKey, contentType)
    if err != nil {
        s.logger.Error("Failed to initiate multipart upload",
            zap.String("tenant_id", tenantID),
            zap.String("object_key", objectKey),
            zap.Error(err))
        return nil, err
    }

    s.logger.Info("Multipart upload initiated",
        zap.String("tenant_id", tenantID),
        zap.String("object_key", objectKey),
        zap.String("upload_id", *result.UploadId))

    return result, nil
}

// UploadPart uploads a part of a multipart upload
func (s *StorageService) UploadPart(ctx context.Context, tenantID, objectKey, uploadID string, partNumber int32, reader io.ReadSeeker, size int64) (*s3.UploadPartOutput, error) {
    result, err := s.s3Client.UploadPart(ctx, tenantID, objectKey, uploadID, partNumber, reader, size)
    if err != nil {
        s.logger.Error("Failed to upload part",
            zap.String("upload_id", uploadID),
            zap.Int32("part_number", partNumber),
            zap.Error(err))
        return nil, err
    }

    return result, nil
}

// CompleteMultipartUpload completes a multipart upload
func (s *StorageService) CompleteMultipartUpload(ctx context.Context, tenantID, objectKey, uploadID string, parts []model.PartInfo) error {
    // Convert parts to S3 format
    s3Parts := make([]types.CompletedPart, len(parts))
    for i, part := range parts {
        s3Parts[i] = types.CompletedPart{
            PartNumber: aws.Int32(int32(part.PartNumber)),
            ETag:       aws.String(part.ETag),
        }
    }

    err := s.s3Client.CompleteMultipartUpload(ctx, tenantID, objectKey, uploadID, s3Parts)
    if err != nil {
        s.logger.Error("Failed to complete multipart upload",
            zap.String("upload_id", uploadID),
            zap.Error(err))
        return err
    }

    s.logger.Info("Multipart upload completed",
        zap.String("upload_id", uploadID),
        zap.String("object_key", objectKey))

    return nil
}

// AbortMultipartUpload aborts a multipart upload
func (s *StorageService) AbortMultipartUpload(ctx context.Context, tenantID, objectKey, uploadID string) error {
    err := s.s3Client.AbortMultipartUpload(ctx, tenantID, objectKey, uploadID)
    if err != nil {
        s.logger.Error("Failed to abort multipart upload",
            zap.String("upload_id", uploadID),
            zap.Error(err))
        return err
    }

    return nil
}

// CreateTenantBucket creates a new bucket for a tenant
func (s *StorageService) CreateTenantBucket(ctx context.Context, tenantID string) error {
    return s.s3Client.CreateTenantBucket(ctx, tenantID)
}

// GetTenantBucket returns the bucket name for a tenant
func (s *StorageService) GetTenantBucket(tenantID string) string {
    return fmt.Sprintf("%s-%s-storage", s.config.S3BucketPrefix, tenantID)
}

// CheckTenantQuota checks if tenant has enough storage quota
func (s *StorageService) CheckTenantQuota(tenantID string, fileSize int64) bool {
    // This would typically check against database records
    // For now, return true if under max file size
    return fileSize <= s.config.MaxFileSize
}

// GetMaxFileSize returns the maximum allowed file size
func (s *StorageService) GetMaxFileSize() int64 {
    return s.config.MaxFileSize
}

// GetPartSize returns the part size for multipart uploads
func (s *StorageService) GetPartSize() int64 {
    return s.config.UploadPartSize
}

// IsExtensionAllowed checks if file extension is allowed
func (s *StorageService) IsExtensionAllowed(ext string) bool {
    ext = strings.TrimPrefix(ext, ".")
    for _, allowed := range s.config.AllowedExtensions {
        if strings.EqualFold(ext, allowed) {
            return true
        }
    }
    return false
}

// IsMimeTypeAllowed checks if MIME type is allowed
func (s *StorageService) IsMimeTypeAllowed(mimeType string) bool {
    for _, allowed := range s.config.AllowedMimeTypes {
        if strings.EqualFold(mimeType, allowed) {
            return true
        }
    }
    return false
}

// GetBucketStats returns bucket statistics
func (s *StorageService) GetBucketStats(ctx context.Context, tenantID string) (map[string]interface{}, error) {
    bucketName := s.GetTenantBucket(tenantID)
    
    // Get bucket metrics
    stats := map[string]interface{}{
        "bucket_name": bucketName,
        "tenant_id":   tenantID,
    }

    // Calculate total size and object count
    var totalSize int64
    var objectCount int

    paginator := s3.NewListObjectsV2Paginator(s.s3Client.GetClient(), &s3.ListObjectsV2Input{
        Bucket: aws.String(bucketName),
    })

    for paginator.HasMorePages() {
        page, err := paginator.NextPage(ctx)
        if err != nil {
            return nil, fmt.Errorf("failed to list objects: %w", err)
        }

        for _, obj := range page.Contents {
            totalSize += *obj.Size
            objectCount++
        }
    }

    stats["total_size_bytes"] = totalSize
    stats["total_size_gb"] = float64(totalSize) / (1024 * 1024 * 1024)
    stats["object_count"] = objectCount

    return stats, nil
}
