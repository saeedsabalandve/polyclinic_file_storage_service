package handler

import (
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"

    "github.com/go-chi/chi/v5"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/middleware"
    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/service"
    "github.com/polyclinic/file-storage-service/internal/utils"
)

// DownloadHandler handles file download requests
type DownloadHandler struct {
    storageService  *service.StorageService
    metadataService *service.MetadataService
    logger          *zap.Logger
}

// NewDownloadHandler creates a new DownloadHandler
func NewDownloadHandler(storageSvc *service.StorageService, metadataSvc *service.MetadataService, logger *zap.Logger) *DownloadHandler {
    return &DownloadHandler{
        storageService:  storageSvc,
        metadataService: metadataSvc,
        logger:          logger,
    }
}

// Download handles direct file download
func (h *DownloadHandler) Download(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    fileID := chi.URLParam(r, "fileId")
    if fileID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "File ID is required")
        return
    }

    // Get file metadata from database
    fileRecord, err := h.metadataService.GetFileRecord(r.Context(), fileID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "File not found")
        return
    }

    // Verify tenant ownership
    if fileRecord.TenantID != tenantID {
        utils.RespondWithError(w, http.StatusForbidden, "Access denied")
        return
    }

    // Check if file is active
    if fileRecord.Status != model.FileStatusActive {
        utils.RespondWithError(w, http.StatusGone, "File is no longer available")
        return
    }

    // Download from S3
    result, err := h.storageService.DownloadFile(r.Context(), tenantID, fileRecord.ObjectKey)
    if err != nil {
        h.logger.Error("Failed to download file", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to download file")
        return
    }
    defer result.Body.Close()

    // Set response headers
    w.Header().Set("Content-Type", fileRecord.ContentType)
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileRecord.OriginalName))
    w.Header().Set("Content-Length", fmt.Sprintf("%d", fileRecord.Size))
    w.Header().Set("Cache-Control", "private, max-age=3600")
    w.Header().Set("X-Content-Type-Options", "nosniff")

    // Stream file to response
    if _, err := io.Copy(w, result.Body); err != nil {
        h.logger.Error("Failed to stream file", zap.Error(err))
    }

    // Log download in audit trail
    h.logAuditDownload(r, fileRecord)
}

// GeneratePresignedURL generates a pre-signed URL for file download
func (h *DownloadHandler) GeneratePresignedURL(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    fileID := chi.URLParam(r, "fileId")
    if fileID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "File ID is required")
        return
    }

    // Get file metadata
    fileRecord, err := h.metadataService.GetFileRecord(r.Context(), fileID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "File not found")
        return
    }

    // Verify tenant ownership
    if fileRecord.TenantID != tenantID {
        utils.RespondWithError(w, http.StatusForbidden, "Access denied")
        return
    }

    // Parse optional expiry duration
    expiry := 15 * time.Minute // Default 15 minutes
    if expiryParam := r.URL.Query().Get("expiry"); expiryParam != "" {
        if parsedExpiry, err := time.ParseDuration(expiryParam); err == nil {
            maxExpiry := 7 * 24 * time.Hour // Maximum 7 days
            if parsedExpiry <= maxExpiry {
                expiry = parsedExpiry
            }
        }
    }

    // Generate presigned URL
    presignedURL, err := h.storageService.GeneratePresignedURL(r.Context(), tenantID, fileRecord.ObjectKey, expiry)
    if err != nil {
        h.logger.Error("Failed to generate presigned URL", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to generate download URL")
        return
    }

    response := model.PresignedURLResponse{
        URL:       presignedURL,
        ExpiresAt: time.Now().Add(expiry),
        Method:    "GET",
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// GeneratePresignedUploadURL generates a pre-signed URL for upload
func (h *DownloadHandler) GeneratePresignedUploadURL(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    // Parse request body
    var request struct {
        Filename    string `json:"filename"`
        ContentType string `json:"content_type"`
        Expiry      int    `json:"expiry_minutes,omitempty"`
    }

    if err := utils.DecodeJSONBody(r, &request); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if request.Filename == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Filename is required")
        return
    }

    // Generate unique object key
    fileID := generateFileID()
    objectKey := fmt.Sprintf("%s/uploads/%s/%s", tenantID, time.Now().Format("2006/01"), fileID)

    // Calculate expiry
    expiry := 15 * time.Minute
    if request.Expiry > 0 && request.Expiry <= 60 {
        expiry = time.Duration(request.Expiry) * time.Minute
    }

    // Generate presigned URL
    presignedURL, err := h.storageService.GeneratePresignedUploadURL(r.Context(), tenantID, objectKey, expiry)
    if err != nil {
        h.logger.Error("Failed to generate presigned upload URL", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to generate upload URL")
        return
    }

    response := map[string]interface{}{
        "upload_url":  presignedURL,
        "object_key":  objectKey,
        "expires_at":  time.Now().Add(expiry),
        "expiry_seconds": int(expiry.Seconds()),
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// StreamDownload handles streaming large files
func (h *DownloadHandler) StreamDownload(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    fileID := chi.URLParam(r, "fileId")
    if fileID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "File ID is required")
        return
    }

    // Get file metadata
    fileRecord, err := h.metadataService.GetFileRecord(r.Context(), fileID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "File not found")
        return
    }

    // Verify tenant ownership
    if fileRecord.TenantID != tenantID {
        utils.RespondWithError(w, http.StatusForbidden, "Access denied")
        return
    }

    // Support range requests
    rangeHeader := r.Header.Get("Range")
    
    // Get object from S3
    result, err := h.storageService.DownloadFile(r.Context(), tenantID, fileRecord.ObjectKey)
    if err != nil {
        h.logger.Error("Failed to download file", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to download file")
        return
    }
    defer result.Body.Close()

    // Handle range requests
    if rangeHeader != "" {
        // Parse and handle range request
        // This would implement HTTP range request handling
        w.Header().Set("Accept-Ranges", "bytes")
        w.WriteHeader(http.StatusPartialContent)
    }

    // Set headers
    w.Header().Set("Content-Type", fileRecord.ContentType)
    w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", url.QueryEscape(fileRecord.OriginalName)))
    
    if rangeHeader == "" {
        w.Header().Set("Content-Length", fmt.Sprintf("%d", fileRecord.Size))
    }

    // Stream file
    buf := make([]byte, 32*1024) // 32KB buffer
    io.CopyBuffer(w, result.Body, buf)
}

// Helper functions
func (h *DownloadHandler) logAuditDownload(r *http.Request, fileRecord *model.File) {
    claims, err := middleware.GetUserFromContext(r.Context())
    if err != nil {
        return
    }

    h.logger.Info("File downloaded",
        zap.String("file_id", fileRecord.ID),
        zap.String("tenant_id", fileRecord.TenantID),
        zap.String("user_id", claims.UserID),
        zap.String("filename", fileRecord.OriginalName),
        zap.Int64("size", fileRecord.Size),
    )
}

func generateFileID() string {
    return fmt.Sprintf("%d_%s", time.Now().UnixNano(), randomString(8))
}
