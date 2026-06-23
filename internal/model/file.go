package model

import (
    "time"
)

// File represents a stored file entity
type File struct {
    ID              string    `json:"id" db:"id"`
    TenantID        string    `json:"tenant_id" db:"tenant_id"`
    Filename        string    `json:"filename" db:"filename"`
    OriginalName    string    `json:"original_name" db:"original_name"`
    BucketName      string    `json:"bucket_name" db:"bucket_name"`
    ObjectKey       string    `json:"object_key" db:"object_key"`
    Size            int64     `json:"size" db:"size"`
    ContentType     string    `json:"content_type" db:"content_type"`
    MD5Hash         string    `json:"md5_hash" db:"md5_hash"`
    SHA256Hash      string    `json:"sha256_hash" db:"sha256_hash"`
    EncryptionType  string    `json:"encryption_type" db:"encryption_type"`
    VersionID       string    `json:"version_id" db:"version_id"`
    Status          string    `json:"status" db:"status"` // active, archived, deleted
    Tags            JSONB     `json:"tags" db:"tags"`
    Metadata        JSONB     `json:"metadata" db:"metadata"`
    UploadedBy      string    `json:"uploaded_by" db:"uploaded_by"`
    UpdatedBy       string    `json:"updated_by" db:"updated_by"`
    CreatedAt       time.Time `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
    ExpiresAt       *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// FileMetadata represents additional metadata for medical files
type FileMetadata struct {
    PatientID       string `json:"patient_id,omitempty"`
    PatientName     string `json:"patient_name,omitempty"`
    StudyType       string `json:"study_type,omitempty"`      // X-RAY, MRI, CT, etc.
    BodyPart        string `json:"body_part,omitempty"`
    Modality        string `json:"modality,omitempty"`
    StudyDate       string `json:"study_date,omitempty"`
    DoctorID        string `json:"doctor_id,omitempty"`
    DoctorName      string `json:"doctor_name,omitempty"`
    Department      string `json:"department,omitempty"`
    Priority        string `json:"priority,omitempty"`        // low, normal, high, urgent
    Diagnosis       string `json:"diagnosis,omitempty"`
    Notes           string `json:"notes,omitempty"`
    AccessLevel     string `json:"access_level,omitempty"`    // public, restricted, confidential
    RetentionPeriod int    `json:"retention_period,omitempty"` // days
}

// UploadRequest represents a file upload request
type UploadRequest struct {
    File        []byte   `json:"-"`
    Filename    string   `json:"filename"`
    ContentType string   `json:"content_type"`
    Metadata    FileMetadata `json:"metadata"`
    Tags        []string `json:"tags"`
}

// UploadResponse represents a file upload response
type UploadResponse struct {
    ID           string     `json:"id"`
    Filename     string     `json:"filename"`
    Size         int64      `json:"size"`
    ContentType  string     `json:"content_type"`
    UploadedAt   time.Time  `json:"uploaded_at"`
    PresignedURL string     `json:"presigned_url,omitempty"`
    Metadata     FileMetadata `json:"metadata"`
    Status       string     `json:"status"`
}

// MultipartUpload represents a multipart upload session
type MultipartUpload struct {
    UploadID    string    `json:"upload_id"`
    FileID      string    `json:"file_id"`
    Filename    string    `json:"filename"`
    Bucket      string    `json:"bucket"`
    Key         string    `json:"key"`
    PartCount   int       `json:"part_count"`
    TotalSize   int64     `json:"total_size"`
    Parts       []PartInfo `json:"parts"`
    Status      string    `json:"status"`
    InitiatedAt time.Time `json:"initiated_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// PartInfo represents a part of a multipart upload
type PartInfo struct {
    PartNumber int    `json:"part_number"`
    ETag       string `json:"etag"`
    Size       int64  `json:"size"`
}

// DownloadResponse represents a file download response
type DownloadResponse struct {
    File        []byte `json:"-"`
    Filename    string `json:"filename"`
    ContentType string `json:"content_type"`
    Size        int64  `json:"size"`
}

// PresignedURLResponse represents a presigned URL response
type PresignedURLResponse struct {
    URL       string    `json:"url"`
    ExpiresAt time.Time `json:"expires_at"`
    Method    string    `json:"method"` // GET or PUT
}

// FileListResponse represents a list of files with pagination
type FileListResponse struct {
    Files      []File `json:"files"`
    TotalCount int    `json:"total_count"`
    Page       int    `json:"page"`
    PageSize   int    `json:"page_size"`
    TotalPages int    `json:"total_pages"`
}

// DeleteResponse represents a file deletion response
type DeleteResponse struct {
    ID       string `json:"id"`
    Status   string `json:"status"`
    DeletedAt time.Time `json:"deleted_at"`
}

// BatchDeleteRequest represents a batch deletion request
type BatchDeleteRequest struct {
    FileIDs []string `json:"file_ids"`
    Reason  string   `json:"reason"`
}

// BatchDeleteResponse represents a batch deletion response
type BatchDeleteResponse struct {
    SuccessCount int      `json:"success_count"`
    FailedCount  int      `json:"failed_count"`
    FailedIDs    []string `json:"failed_ids,omitempty"`
    Errors       []string `json:"errors,omitempty"`
}

// FileStatus constants
const (
    FileStatusActive   = "active"
    FileStatusArchived = "archived"
    FileStatusDeleted  = "deleted"
    FileStatusExpired  = "expired"
)

// JSONB type for PostgreSQL JSONB fields
type JSONB map[string]interface{}
