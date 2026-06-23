package handler

import (
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/middleware"
    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/service"
    "github.com/polyclinic/file-storage-service/internal/utils"
)

// DeleteHandler handles file deletion requests
type DeleteHandler struct {
    storageService  *service.StorageService
    metadataService *service.MetadataService
    logger          *zap.Logger
}

// NewDeleteHandler creates a new DeleteHandler
func NewDeleteHandler(storageSvc *service.StorageService, metadataSvc *service.MetadataService, logger *zap.Logger) *DeleteHandler {
    return &DeleteHandler{
        storageService:  storageSvc,
        metadataService: metadataSvc,
        logger:          logger,
    }
}

// Delete handles single file deletion
func (h *DeleteHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

    claims, err := middleware.GetUserFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusUnauthorized, "User not authenticated")
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

    // Check if user has permission to delete
    if !h.hasDeletePermission(claims, fileRecord) {
        utils.RespondWithError(w, http.StatusForbidden, "Insufficient permissions to delete this file")
        return
    }

    // Get deletion type (soft or hard delete)
    deleteType := r.URL.Query().Get("type")
    if deleteType == "" {
        deleteType = "soft" // Default to soft delete
    }

    var response model.DeleteResponse

    switch deleteType {
    case "hard":
        // Hard delete - permanently remove file
        err = h.performHardDelete(r, tenantID, fileRecord)
        if err != nil {
            h.logger.Error("Hard delete failed", zap.Error(err))
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete file")
            return
        }
        response = model.DeleteResponse{
            ID:       fileID,
            Status:   "deleted_permanently",
            DeletedAt: time.Now(),
        }

    case "soft":
        // Soft delete - mark as deleted but keep in storage
        err = h.performSoftDelete(r, fileRecord)
        if err != nil {
            h.logger.Error("Soft delete failed", zap.Error(err))
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete file")
            return
        }
        response = model.DeleteResponse{
            ID:       fileID,
            Status:   "marked_for_deletion",
            DeletedAt: time.Now(),
        }

    default:
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid delete type. Use 'soft' or 'hard'")
        return
    }

    // Log deletion
    h.logger.Info("File deleted",
        zap.String("file_id", fileID),
        zap.String("tenant_id", tenantID),
        zap.String("user_id", claims.UserID),
        zap.String("delete_type", deleteType),
    )

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// BatchDelete handles batch file deletion
func (h *DeleteHandler) BatchDelete(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    claims, err := middleware.GetUserFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusUnauthorized, "User not authenticated")
        return
    }

    // Parse request body
    var request model.BatchDeleteRequest
    if err := utils.DecodeJSONBody(r, &request); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if len(request.FileIDs) == 0 {
        utils.RespondWithError(w, http.StatusBadRequest, "No file IDs provided")
        return
    }

    if len(request.FileIDs) > 100 {
        utils.RespondWithError(w, http.StatusBadRequest, "Maximum 100 files can be deleted at once")
        return
    }

    var successCount int
    var failedIDs []string
    var errors []string

    for _, fileID := range request.FileIDs {
        // Get file metadata
        fileRecord, err := h.metadataService.GetFileRecord(r.Context(), fileID)
        if err != nil {
            failedIDs = append(failedIDs, fileID)
            errors = append(errors, "File not found: "+fileID)
            continue
        }

        // Verify tenant ownership
        if fileRecord.TenantID != tenantID {
            failedIDs = append(failedIDs, fileID)
            errors = append(errors, "Access denied: "+fileID)
            continue
        }

        // Check delete permission
        if !h.hasDeletePermission(claims, fileRecord) {
            failedIDs = append(failedIDs, fileID)
            errors = append(errors, "Insufficient permissions: "+fileID)
            continue
        }

        // Perform soft delete
        if err := h.performSoftDelete(r, fileRecord); err != nil {
            h.logger.Error("Batch delete failed for file", zap.String("file_id", fileID), zap.Error(err))
            failedIDs = append(failedIDs, fileID)
            errors = append(errors, "Delete failed: "+fileID)
            continue
        }

        successCount++
    }

    response := model.BatchDeleteResponse{
        SuccessCount: successCount,
        FailedCount:  len(failedIDs),
        FailedIDs:    failedIDs,
        Errors:       errors,
    }

    statusCode := http.StatusOK
    if successCount == 0 {
        statusCode = http.StatusBadRequest
    } else if len(failedIDs) > 0 {
        statusCode = http.StatusMultiStatus
    }

    utils.RespondWithJSON(w, statusCode, response)
}

// Restore handles file restoration (for soft-deleted files)
func (h *DeleteHandler) Restore(w http.ResponseWriter, r *http.Request) {
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

    // Check if file is deleted
    if fileRecord.Status != model.FileStatusDeleted {
        utils.RespondWithError(w, http.StatusBadRequest, "File is not in deleted state")
        return
    }

    // Restore file
    fileRecord.Status = model.FileStatusActive
    fileRecord.UpdatedAt = time.Now()
    
    if err := h.metadataService.UpdateFileRecord(r.Context(), fileRecord); err != nil {
        h.logger.Error("Failed to restore file", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to restore file")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "status":  "restored",
        "message": "File successfully restored",
        "file_id": fileID,
    })
}

// Helper functions
func (h *DeleteHandler) performSoftDelete(r *http.Request, fileRecord *model.File) error {
    fileRecord.Status = model.FileStatusDeleted
    fileRecord.UpdatedAt = time.Now()
    
    // Set expiration for permanent deletion (e.g., 30 days)
    expiry := time.Now().AddDate(0, 0, 30)
    fileRecord.ExpiresAt = &expiry

    return h.metadataService.UpdateFileRecord(r.Context(), fileRecord)
}

func (h *DeleteHandler) performHardDelete(r *http.Request, tenantID string, fileRecord *model.File) error {
    // Delete from S3
    if err := h.storageService.DeleteFile(r.Context(), tenantID, fileRecord.ObjectKey); err != nil {
        return err
    }

    // Delete from database
    return h.metadataService.DeleteFileRecord(r.Context(), fileRecord.ID)
}

func (h *DeleteHandler) hasDeletePermission(claims *middleware.Claims, fileRecord *model.File) bool {
    // Admin can delete any file
    for _, role := range claims.Roles {
        if role == "admin" || role == "super_admin" {
            return true
        }
    }

    // Doctor can delete their own files
    if claims.UserID == fileRecord.UploadedBy {
        return true
    }

    return false
}

// CleanupExpiredFiles handles cleanup of soft-deleted files
func (h *DeleteHandler) CleanupExpiredFiles(w http.ResponseWriter, r *http.Request) {
    // This would typically be called by a scheduled job
    expiredFiles, err := h.metadataService.GetExpiredFiles(r.Context())
    if err != nil {
        h.logger.Error("Failed to get expired files", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get expired files")
        return
    }

    var cleanedCount int
    var errors []string

    for _, file := range expiredFiles {
        if err := h.performHardDelete(r, file.TenantID, &file); err != nil {
            h.logger.Error("Failed to clean up expired file", zap.String("file_id", file.ID), zap.Error(err))
            errors = append(errors, file.ID)
            continue
        }
        cleanedCount++
    }

    response := map[string]interface{}{
        "cleaned_count": cleanedCount,
        "errors":        errors,
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}
