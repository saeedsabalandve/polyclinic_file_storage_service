package utils

import (
    "encoding/json"
    "net/http"
)

// APIResponse represents a standard API response
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *APIError   `json:"error,omitempty"`
    Meta    *APIMeta    `json:"meta,omitempty"`
}

// APIError represents an API error
type APIError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}

// APIMeta represents metadata for paginated responses
type APIMeta struct {
    Page       int   `json:"page"`
    PageSize   int   `json:"page_size"`
    TotalCount int   `json:"total_count"`
    TotalPages int   `json:"total_pages"`
    HasNext    bool  `json:"has_next"`
    HasPrev    bool  `json:"has_prev"`
}

// RespondWithJSON sends a JSON response
func RespondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)

    response := APIResponse{
        Success: statusCode >= 200 && statusCode < 300,
        Data:    data,
    }

    json.NewEncoder(w).Encode(response)
}

// RespondWithError sends an error response
func RespondWithError(w http.ResponseWriter, statusCode int, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)

    response := APIResponse{
        Success: false,
        Error: &APIError{
            Code:    http.StatusText(statusCode),
            Message: message,
        },
    }

    json.NewEncoder(w).Encode(response)
}

// RespondWithDetailedError sends a detailed error response
func RespondWithDetailedError(w http.ResponseWriter, statusCode int, code, message, details string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)

    response := APIResponse{
        Success: false,
        Error: &APIError{
            Code:    code,
            Message: message,
            Details: details,
        },
    }

    json.NewEncoder(w).Encode(response)
}

// RespondWithPaginatedJSON sends a paginated JSON response
func RespondWithPaginatedJSON(w http.ResponseWriter, statusCode int, data interface{}, meta *APIMeta) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)

    response := APIResponse{
        Success: true,
        Data:    data,
        Meta:    meta,
    }

    json.NewEncoder(w).Encode(response)
}

// RespondWithFile sends a file response
func RespondWithFile(w http.ResponseWriter, filename string, contentType string, data []byte) {
    w.Header().Set("Content-Type", contentType)
    w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
    w.Header().Set("Content-Length", string(rune(len(data))))
    w.WriteHeader(http.StatusOK)
    w.Write(data)
}

// DecodeJSONBody decodes JSON request body
func DecodeJSONBody(r *http.Request, v interface{}) error {
    if r.Body == nil {
        return ErrEmptyBody
    }
    
    decoder := json.NewDecoder(r.Body)
    decoder.DisallowUnknownFields()
    
    return decoder.Decode(v)
}

// Error types
var (
    ErrEmptyBody        = &AppError{Code: "EMPTY_BODY", Message: "Request body is empty"}
    ErrInvalidJSON      = &AppError{Code: "INVALID_JSON", Message: "Invalid JSON format"}
    ErrUnauthorized     = &AppError{Code: "UNAUTHORIZED", Message: "Authentication required"}
    ErrForbidden        = &AppError{Code: "FORBIDDEN", Message: "Access denied"}
    ErrNotFound         = &AppError{Code: "NOT_FOUND", Message: "Resource not found"}
    ErrInternalServer   = &AppError{Code: "INTERNAL_ERROR", Message: "Internal server error"}
    ErrTooManyRequests  = &AppError{Code: "TOO_MANY_REQUESTS", Message: "Rate limit exceeded"}
    ErrValidationFailed = &AppError{Code: "VALIDATION_FAILED", Message: "Validation failed"}
)

// AppError represents an application error
type AppError struct {
    Code    string
    Message string
}

func (e *AppError) Error() string {
    return e.Message
}

// NewAppError creates a new application error
func NewAppError(code, message string) *AppError {
    return &AppError{
        Code:    code,
        Message: message,
    }
}

// SuccessResponse creates a success response
func SuccessResponse(data interface{}) APIResponse {
    return APIResponse{
        Success: true,
        Data:    data,
    }
}

// ErrorResponse creates an error response
func ErrorResponse(code, message string) APIResponse {
    return APIResponse{
        Success: false,
        Error: &APIError{
            Code:    code,
            Message: message,
        },
    }
}

// CreatePaginationMeta creates pagination metadata
func CreatePaginationMeta(page, pageSize, totalCount int) *APIMeta {
    totalPages := (totalCount + pageSize - 1) / pageSize
    
    return &APIMeta{
        Page:       page,
        PageSize:   pageSize,
        TotalCount: totalCount,
        TotalPages: totalPages,
        HasNext:    page < totalPages,
        HasPrev:    page > 1,
    }
}
