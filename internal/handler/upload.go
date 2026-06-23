package handler

import (
    "bytes"
    "crypto/md5"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "net/http"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/middleware"
    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/service"
    "github.com/polyclinic/file-storage-service/internal/utils"
)

// UploadHandler handles file upload requests
type UploadHandler struct {
    storageService  *service.StorageService
    metadataService *service.MetadataService
    logger          *zap.Logger
}

// NewUploadHandler creates a new UploadHandler
func NewUploadHandler(storageSvc *service.StorageService, metadataSvc *service.MetadataService, logger *zap.Logger) *UploadHandler {
    return &UploadHandler{
        storageService:  storageSvc,
        metadataService: metadataSvc,
        logger:          logger,
    }
}

// Upload handles single file upload
func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
    // Get tenant from context
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    // Get user from context
    claims, err := middleware.GetUserFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusUnauthorized, "User not authenticated")
        return
    }

    // Parse multipart form (max 32MB in memory, rest on disk)
    if err := r.ParseMultipartForm(32 << 20); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Failed to parse multipart form")
        return
    }
    defer r.MultipartForm.RemoveAll()

    // Get file from form
    file, header, err := r.FormFile("file")
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "File is required")
        return
    }
    defer file.Close()

    // Validate file
    if err := h.validateFile(header, tenantID); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, err.Error())
        return
    }

    // Read file into buffer
    buf := bytes.NewBuffer(nil)
    if _, err := io.Copy(buf, file); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to read file")
        return
    }

    // Calculate file hashes
    fileBytes := buf.Bytes()
    md5Hash := md5.Sum(fileBytes)
    sha256Hash := sha256.Sum256(fileBytes)
    
    md5String := hex.EncodeToString(md5Hash[:])
    sha256String := hex.EncodeToString(sha256Hash[:])

    // Generate file ID
    fileID := uuid.New().String()
    
    // Generate object key
    ext := filepath.Ext(header.Filename)
    objectKey := fmt.Sprintf("%s/%s/%s%s", tenantID, time.Now().Format("2006/01/02"), fileID, ext)

    // Upload to S3
    fileReader := bytes.NewReader(fileBytes)
    uploadResult, err := h.storageService.UploadFile(r.Context(), tenantID, objectKey, fileReader, header.Header.Get("Content-Type"), nil)
    if err != nil {
        h.logger.Error("Failed to upload file", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to upload file")
        return
    }

    // Extract metadata from form
    fileMetadata := h.extractMetadata(r)

    // Create file record
    fileRecord := &model.File{
        ID:             fileID,
        TenantID:       tenantID,
        Filename:       fmt.Sprintf("%s%s", fileID, ext),
        OriginalName:   header.Filename,
        BucketName:     h.storageService.GetTenantBucket(tenantID),
        ObjectKey:      objectKey,
        Size:           header.Size,
        ContentType:    header.Header.Get("Content-Type"),
        MD5Hash:        md5String,
        SHA256Hash:     sha256String,
        EncryptionType: "SSE-S3",
        VersionID:      *uploadResult.VersionId,
        Status:         model.FileStatusActive,
        Tags:           model.JSONB{},
        Metadata:       model.JSONB{},
        UploadedBy:     claims.UserID,
        CreatedAt:      time.Now(),
        UpdatedAt:      time.Now(),
    }

    // Save to database
    if err := h.metadataService.CreateFileRecord(r.Context(), fileRecord); err != nil {
        h.logger.Error("Failed to save file metadata", zap.Error(err))
        // Try to delete uploaded file
        h.storageService.DeleteFile(r.Context(), tenantID, objectKey)
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save file metadata")
        return
    }

    // Generate response
    response := model.UploadResponse{
        ID:          fileID,
        Filename:    header.Filename,
        Size:        header.Size,
        ContentType: header.Header.Get("Content-Type"),
        UploadedAt:  time.Now(),
        Metadata:    fileMetadata,
        Status:      model.FileStatusActive,
    }

    utils.RespondWithJSON(w, http.StatusCreated, response)
}

// InitiateMultipartUpload initiates a multipart upload
func (h *UploadHandler) InitiateMultipartUpload(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    // Parse request body
    var request struct {
        Filename    string `json:"filename"`
        ContentType string `json:"content_type"`
        TotalSize   int64  `json:"total_size"`
        PartCount   int    `json:"part_count"`
    }

    if err := utils.DecodeJSONBody(r, &request); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    // Validate total size
    if request.TotalSize > h.storageService.GetMaxFileSize() {
        utils.RespondWithError(w, http.StatusBadRequest, "File size exceeds maximum allowed")
        return
    }

    // Generate file ID
    fileID := uuid.New().String()
    ext := filepath.Ext(request.Filename)
    objectKey := fmt.Sprintf("%s/multipart/%s/%s%s", tenantID, time.Now().Format("2006/01/02"), fileID, ext)

    // Initiate multipart upload
    uploadResult, err := h.storageService.InitiateMultipartUpload(r.Context(), tenantID, objectKey, request.ContentType)
    if err != nil {
        h.logger.Error("Failed to initiate multipart upload", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to initiate upload")
        return
    }

    // Save upload session to database
    upload := &model.MultipartUpload{
        UploadID:    *uploadResult.UploadId,
        FileID:      fileID,
        Filename:    request.Filename,
        Bucket:      h.storageService.GetTenantBucket(tenantID),
        Key:         objectKey,
        PartCount:   request.PartCount,
        TotalSize:   request.TotalSize,
        Parts:       []model.PartInfo{},
        Status:      "in_progress",
        InitiatedAt: time.Now(),
    }

    if err := h.metadataService.SaveMultipartUpload(r.Context(), upload); err != nil {
        h.logger.Error("Failed to save upload session", zap.Error(err))
        h.storageService.AbortMultipartUpload(r.Context(), tenantID, objectKey, *uploadResult.UploadId)
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save upload session")
        return
    }

    // Calculate part size
    partSize := h.storageService.GetPartSize()
    
    response := map[string]interface{}{
        "upload_id": *uploadResult.UploadId,
        "file_id":   fileID,
        "part_size": partSize,
        "part_count": request.PartCount,
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// UploadPart handles uploading a part of a multipart upload
func (h *UploadHandler) UploadPart(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    uploadID := chi.URLParam(r, "uploadId")
    if uploadID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Upload ID is required")
        return
    }

    partNumber, err := strconv.Atoi(r.URL.Query().Get("partNumber"))
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid part number")
        return
    }

    // Get upload session from database
    upload, err := h.metadataService.GetMultipartUpload(r.Context(), uploadID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "Upload session not found")
        return
    }

    // Read part data
    buf := bytes.NewBuffer(nil)
    if _, err := io.Copy(buf, r.Body); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to read part data")
        return
    }

    // Upload part to S3
    reader := bytes.NewReader(buf.Bytes())
    partResult, err := h.storageService.UploadPart(r.Context(), tenantID, upload.Key, uploadID, int32(partNumber), reader, int64(buf.Len()))
    if err != nil {
        h.logger.Error("Failed to upload part", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to upload part")
        return
    }

    // Update upload session
    part := model.PartInfo{
        PartNumber: partNumber,
        ETag:       *partResult.ETag,
        Size:       int64(buf.Len()),
    }
    upload.Parts = append(upload.Parts, part)

    if err := h.metadataService.UpdateMultipartUpload(r.Context(), upload); err != nil {
        h.logger.Error("Failed to update upload session", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update upload session")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
        "etag":        *partResult.ETag,
        "part_number": partNumber,
        "size":        buf.Len(),
    })
}

// CompleteMultipartUpload completes a multipart upload
func (h *UploadHandler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
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

    uploadID := chi.URLParam(r, "uploadId")
    if uploadID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Upload ID is required")
        return
    }

    // Get upload session
    upload, err := h.metadataService.GetMultipartUpload(r.Context(), uploadID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "Upload session not found")
        return
    }

    // Check if all parts are uploaded
    if len(upload.Parts) != upload.PartCount {
        utils.RespondWithError(w, http.StatusBadRequest, "Not all parts have been uploaded")
        return
    }

    // Complete multipart upload in S3
    if err := h.storageService.CompleteMultipartUpload(r.Context(), tenantID, upload.Key, uploadID, upload.Parts); err != nil {
        h.logger.Error("Failed to complete multipart upload", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to complete upload")
        return
    }

    // Create file record in database
    fileRecord := &model.File{
        ID:             upload.FileID,
        TenantID:       tenantID,
        Filename:       filepath.Base(upload.Key),
        OriginalName:   upload.Filename,
        BucketName:     upload.Bucket,
        ObjectKey:      upload.Key,
        Size:           upload.TotalSize,
        Status:         model.FileStatusActive,
        UploadedBy:     claims.UserID,
        CreatedAt:      time.Now(),
        UpdatedAt:      time.Now(),
    }

    if err := h.metadataService.CreateFileRecord(r.Context(), fileRecord); err != nil {
        h.logger.Error("Failed to save file metadata", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to save file metadata")
        return
    }

    // Mark upload as completed
    upload.Status = "completed"
    now := time.Now()
    upload.CompletedAt = &now
    h.metadataService.UpdateMultipartUpload(r.Context(), upload)

    response := model.UploadResponse{
        ID:        upload.FileID,
        Filename:  upload.Filename,
        Size:      upload.TotalSize,
        Status:    model.FileStatusActive,
        UploadedAt: time.Now(),
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// AbortMultipartUpload aborts a multipart upload
func (h *UploadHandler) AbortMultipartUpload(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    uploadID := chi.URLParam(r, "uploadId")
    if uploadID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Upload ID is required")
        return
    }

    // Get upload session
    upload, err := h.metadataService.GetMultipartUpload(r.Context(), uploadID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "Upload session not found")
        return
    }

    // Abort in S3
    if err := h.storageService.AbortMultipartUpload(r.Context(), tenantID, upload.Key, uploadID); err != nil {
        h.logger.Error("Failed to abort multipart upload", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to abort upload")
        return
    }

    // Update status
    upload.Status = "aborted"
    h.metadataService.UpdateMultipartUpload(r.Context(), upload)

    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "status":  "aborted",
        "message": "Upload successfully aborted",
    })
}

// Helper functions
func (h *UploadHandler) validateFile(header interface{ GetFilename() string; GetSize() int64 }, tenantID string) error {
    // Check file size
    maxSize := h.storageService.GetMaxFileSize()
    if header.GetSize() > maxSize {
        return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", maxSize)
    }

    // Check file extension
    ext := strings.ToLower(filepath.Ext(header.GetFilename()))
    if !h.storageService.IsExtensionAllowed(ext) {
        return fmt.Errorf("file extension %s is not allowed", ext)
    }

    // Check tenant storage quota
    if !h.storageService.CheckTenantQuota(tenantID, header.GetSize()) {
        return fmt.Errorf("tenant storage quota exceeded")
    }

    return nil
}

func (h *UploadHandler) extractMetadata(r *http.Request) model.FileMetadata {
    metadata := model.FileMetadata{
        PatientID:   r.FormValue("patient_id"),
        PatientName: r.FormValue("patient_name"),
        StudyType:   r.FormValue("study_type"),
        BodyPart:    r.FormValue("body_part"),
        Modality:    r.FormValue("modality"),
        StudyDate:   r.FormValue("study_date"),
        DoctorID:    r.FormValue("doctor_id"),
        DoctorName:  r.FormValue("doctor_name"),
        Department:  r.FormValue("department"),
        Priority:    r.FormValue("priority"),
        Diagnosis:   r.FormValue("diagnosis"),
        Notes:       r.FormValue("notes"),
        AccessLevel: r.FormValue("access_level"),
    }

    // Set defaults
    if metadata.Priority == "" {
        metadata.Priority = "normal"
    }
    if metadata.AccessLevel == "" {
        metadata.AccessLevel = "restricted"
    }

    return metadata
}
