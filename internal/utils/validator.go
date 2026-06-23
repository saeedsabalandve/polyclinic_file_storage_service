package utils

import (
    "fmt"
    "mime/multipart"
    "net/http"
    "path/filepath"
    "regexp"
    "strings"
)

// FileValidator handles file validation
type FileValidator struct {
    MaxSize          int64
    AllowedTypes     []string
    AllowedExtensions []string
    BlockedExtensions []string
}

// NewFileValidator creates a new FileValidator
func NewFileValidator(maxSize int64, allowedTypes, allowedExtensions []string) *FileValidator {
    return &FileValidator{
        MaxSize:          maxSize,
        AllowedTypes:     allowedTypes,
        AllowedExtensions: allowedExtensions,
        BlockedExtensions: []string{
            ".exe", ".bat", ".cmd", ".sh", ".bash",
            ".php", ".jsp", ".asp", ".aspx",
            ".dll", ".so", ".dylib",
        },
    }
}

// ValidateFile validates a file against configured rules
func (v *FileValidator) ValidateFile(file multipart.File, header *multipart.FileHeader) error {
    // Check file size
    if header.Size > v.MaxSize {
        return fmt.Errorf("file size %d exceeds maximum allowed size %d", header.Size, v.MaxSize)
    }

    // Check file extension
    ext := strings.ToLower(filepath.Ext(header.Filename))
    if v.isBlockedExtension(ext) {
        return fmt.Errorf("file extension %s is blocked for security reasons", ext)
    }

    if !v.isAllowedExtension(ext) {
        return fmt.Errorf("file extension %s is not allowed", ext)
    }

    // Check MIME type
    contentType := header.Header.Get("Content-Type")
    if !v.isAllowedType(contentType) {
        return fmt.Errorf("file type %s is not allowed", contentType)
    }

    // Validate filename
    if !v.isValidFilename(header.Filename) {
        return fmt.Errorf("filename contains invalid characters")
    }

    return nil
}

// isAllowedExtension checks if extension is in allowed list
func (v *FileValidator) isAllowedExtension(ext string) bool {
    if len(v.AllowedExtensions) == 0 {
        return true
    }

    ext = strings.TrimPrefix(ext, ".")
    for _, allowed := range v.AllowedExtensions {
        if strings.EqualFold(ext, strings.TrimPrefix(allowed, ".")) {
            return true
        }
    }
    return false
}

// isBlockedExtension checks if extension is blocked
func (v *FileValidator) isBlockedExtension(ext string) bool {
    for _, blocked := range v.BlockedExtensions {
        if strings.EqualFold(ext, blocked) {
            return true
        }
    }
    return false
}

// isAllowedType checks if MIME type is allowed
func (v *FileValidator) isAllowedType(contentType string) bool {
    if len(v.AllowedTypes) == 0 {
        return true
    }

    for _, allowed := range v.AllowedTypes {
        if strings.EqualFold(contentType, allowed) {
            return true
        }
        // Support wildcard types like "image/*"
        if strings.HasSuffix(allowed, "/*") {
            prefix := strings.TrimSuffix(allowed, "*")
            if strings.HasPrefix(contentType, prefix) {
                return true
            }
        }
    }
    return false
}

// isValidFilename validates filename format
func (v *FileValidator) isValidFilename(filename string) bool {
    // Filename should not be empty
    if filename == "" {
        return false
    }

    // Filename should not contain path separators
    if strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
        return false
    }

    // Check for null bytes
    if strings.Contains(filename, "\x00") {
        return false
    }

    // Check length (max 255 characters)
    if len(filename) > 255 {
        return false
    }

    // Check for valid characters (alphanumeric, dots, dashes, underscores, spaces)
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9\.\-_\(\)\[\] ]+$`, filename)
    return matched
}

// ValidatePatientID validates patient ID format
func ValidatePatientID(patientID string) bool {
    // Pattern: P followed by 6-10 digits (e.g., P123456)
    matched, _ := regexp.MatchString(`^P\d{6,10}$`, patientID)
    return matched
}

// ValidateDoctorID validates doctor ID format
func ValidateDoctorID(doctorID string) bool {
    // Pattern: D followed by 4-8 digits (e.g., D1234)
    matched, _ := regexp.MatchString(`^D\d{4,8}$`, doctorID)
    return matched
}

// ValidateStudyType validates medical study type
func ValidateStudyType(studyType string) bool {
    validTypes := []string{
        "X-RAY", "MRI", "CT", "ULTRASOUND", "MAMMOGRAPHY",
        "PET", "SPECT", "DEXA", "FLUOROSCOPY", "ANGIOGRAPHY",
    }

    for _, vt := range validTypes {
        if strings.EqualFold(studyType, vt) {
            return true
        }
    }
    return false
}

// SanitizeFilename sanitizes a filename by removing dangerous characters
func SanitizeFilename(filename string) string {
    // Remove path separators
    filename = strings.ReplaceAll(filename, "/", "_")
    filename = strings.ReplaceAll(filename, "\\", "_")
    
    // Remove null bytes
    filename = strings.ReplaceAll(filename, "\x00", "")
    
    // Replace spaces with underscores
    filename = strings.ReplaceAll(filename, " ", "_")
    
    // Remove any character that isn't alphanumeric, dot, dash, or underscore
    reg := regexp.MustCompile(`[^a-zA-Z0-9\.\-_]`)
    filename = reg.ReplaceAllString(filename, "")
    
    // Ensure filename isn't empty
    if filename == "" || filename == "." {
        filename = "unnamed_file"
    }
    
    return filename
}

// DetectContentType detects the content type of a file
func DetectContentType(data []byte) string {
    return http.DetectContentType(data)
}

// IsMedicalFile checks if file is a medical file type
func IsMedicalFile(contentType string) bool {
    medicalTypes := []string{
        "application/dicom",
        "image/dicom",
        "application/octet-stream", // DICOM often detected as this
    }

    for _, mt := range medicalTypes {
        if strings.EqualFold(contentType, mt) {
            return true
        }
    }
    return false
}
