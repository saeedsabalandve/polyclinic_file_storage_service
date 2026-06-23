package handler

import (
    "net/http"
    "strconv"
    "time"

    "github.com/go-chi/chi/v5"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/middleware"
    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/service"
    "github.com/polyclinic/file-storage-service/internal/utils"
)

// MetadataHandler handles file metadata requests
type MetadataHandler struct {
    metadataService *service.MetadataService
    logger          *zap.Logger
}

// NewMetadataHandler creates a new MetadataHandler
func NewMetadataHandler(metadataSvc *service.MetadataService, logger *zap.Logger) *MetadataHandler {
    return &MetadataHandler{
        metadataService: metadataSvc,
        logger:          logger,
    }
}

// GetFileMetadata retrieves file metadata
func (h *MetadataHandler) GetFileMetadata(w http.ResponseWriter, r *http.Request) {
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

    utils.RespondWithJSON(w, http.StatusOK, fileRecord)
}

// UpdateMetadata updates file metadata
func (h *MetadataHandler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
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

    // Get existing file record
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

    // Parse update request
    var updates model.FileMetadata
    if err := utils.DecodeJSONBody(r, &updates); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    // Update metadata
    if fileRecord.Metadata == nil {
        fileRecord.Metadata = make(model.JSONB)
    }

    // Merge new metadata with existing
    metadataMap := fileRecord.Metadata
    if updates.PatientID != "" {
        metadataMap["patient_id"] = updates.PatientID
    }
    if updates.PatientName != "" {
        metadataMap["patient_name"] = updates.PatientName
    }
    if updates.StudyType != "" {
        metadataMap["study_type"] = updates.StudyType
    }
    if updates.BodyPart != "" {
        metadataMap["body_part"] = updates.BodyPart
    }
    if updates.Modality != "" {
        metadataMap["modality"] = updates.Modality
    }
    if updates.DoctorID != "" {
        metadataMap["doctor_id"] = updates.DoctorID
    }
    if updates.DoctorName != "" {
        metadataMap["doctor_name"] = updates.DoctorName
    }
    if updates.Department != "" {
        metadataMap["department"] = updates.Department
    }
    if updates.Priority != "" {
        metadataMap["priority"] = updates.Priority
    }
    if updates.Diagnosis != "" {
        metadataMap["diagnosis"] = updates.Diagnosis
    }
    if updates.Notes != "" {
        metadataMap["notes"] = updates.Notes
    }
    if updates.AccessLevel != "" {
        metadataMap["access_level"] = updates.AccessLevel
    }

    // Add update metadata
    metadataMap["last_updated_by"] = claims.UserID
    metadataMap["last_updated_at"] = time.Now().Format(time.RFC3339)

    fileRecord.Metadata = metadataMap
    fileRecord.UpdatedAt = time.Now()
    fileRecord.UpdatedBy = claims.UserID

    if err := h.metadataService.UpdateFileRecord(r.Context(), fileRecord); err != nil {
        h.logger.Error("Failed to update file metadata", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update metadata")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, fileRecord)
}

// ListFiles returns a paginated list of files
func (h *MetadataHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    // Parse pagination parameters
    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page < 1 {
        page = 1
    }

    pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
    if pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }

    // Parse filters
    filters := make(map[string]interface{})
    if status := r.URL.Query().Get("status"); status != "" {
        filters["status"] = status
    }
    if studyType := r.URL.Query().Get("study_type"); studyType != "" {
        filters["study_type"] = studyType
    }
    if patientID := r.URL.Query().Get("patient_id"); patientID != "" {
        filters["patient_id"] = patientID
    }
    if doctorID := r.URL.Query().Get("doctor_id"); doctorID != "" {
        filters["doctor_id"] = doctorID
    }
    if dateFrom := r.URL.Query().Get("date_from"); dateFrom != "" {
        filters["date_from"] = dateFrom
    }
    if dateTo := r.URL.Query().Get("date_to"); dateTo != "" {
        filters["date_to"] = dateTo
    }

    // Get files from database
    files, totalCount, err := h.metadataService.ListFiles(r.Context(), tenantID, page, pageSize, filters)
    if err != nil {
        h.logger.Error("Failed to list files", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve files")
        return
    }

    totalPages := (totalCount + pageSize - 1) / pageSize

    response := model.FileListResponse{
        Files:      files,
        TotalCount: totalCount,
        Page:       page,
        PageSize:   pageSize,
        TotalPages: totalPages,
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// SearchFiles performs full-text search on file metadata
func (h *MetadataHandler) SearchFiles(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    query := r.URL.Query().Get("q")
    if query == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Search query is required")
        return
    }

    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page < 1 {
        page = 1
    }

    pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
    if pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }

    files, totalCount, err := h.metadataService.SearchFiles(r.Context(), tenantID, query, page, pageSize)
    if err != nil {
        h.logger.Error("Failed to search files", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to search files")
        return
    }

    response := model.FileListResponse{
        Files:      files,
        TotalCount: totalCount,
        Page:       page,
        PageSize:   pageSize,
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// GetFileVersions returns version history for a file
func (h *MetadataHandler) GetFileVersions(w http.ResponseWriter, r *http.Request) {
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

    // Get file metadata to verify tenant ownership
    fileRecord, err := h.metadataService.GetFileRecord(r.Context(), fileID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "File not found")
        return
    }

    if fileRecord.TenantID != tenantID {
        utils.RespondWithError(w, http.StatusForbidden, "Access denied")
        return
    }

    versions, err := h.metadataService.GetFileVersions(r.Context(), fileID)
    if err != nil {
        h.logger.Error("Failed to get file versions", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get file versions")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, versions)
}

// GetFileStats returns file statistics
func (h *MetadataHandler) GetFileStats(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    stats, err := h.metadataService.GetFileStats(r.Context(), tenantID)
    if err != nil {
        h.logger.Error("Failed to get file stats", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get statistics")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, stats)
}
