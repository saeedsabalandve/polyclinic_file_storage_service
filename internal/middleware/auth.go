package middleware

import (
    "context"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/utils"
)

// Claims represents JWT claims
type Claims struct {
    UserID    string   `json:"user_id"`
    Email     string   `json:"email"`
    Roles     []string `json:"roles"`
    TenantID  string   `json:"tenant_id"`
    jwt.RegisteredClaims
}

type contextKey string

const (
    UserContextKey    contextKey = "user"
    TenantContextKey  contextKey = "tenant"
    RequestIDKey      contextKey = "request_id"
)

// JWTAuth middleware validates JWT tokens
func JWTAuth(jwtSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract token from Authorization header
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                utils.RespondWithError(w, http.StatusUnauthorized, "Authorization header is required")
                return
            }

            // Check Bearer prefix
            parts := strings.SplitN(authHeader, " ", 2)
            if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
                utils.RespondWithError(w, http.StatusUnauthorized, "Invalid authorization format. Use: Bearer <token>")
                return
            }

            tokenString := parts[1]

            // Parse and validate token
            claims := &Claims{}
            token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
                // Validate signing method
                if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
                    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
                }
                return []byte(jwtSecret), nil
            })

            if err != nil {
                utils.RespondWithError(w, http.StatusUnauthorized, fmt.Sprintf("Invalid token: %v", err))
                return
            }

            if !token.Valid {
                utils.RespondWithError(w, http.StatusUnauthorized, "Token is not valid")
                return
            }

            // Check token expiration
            if time.Now().After(claims.ExpiresAt.Time) {
                utils.RespondWithError(w, http.StatusUnauthorized, "Token has expired")
                return
            }

            // Add claims to context
            ctx := context.WithValue(r.Context(), UserContextKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// TenantResolver middleware extracts and validates tenant information
func TenantResolver(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get tenant ID from header
        tenantID := r.Header.Get("X-Tenant-ID")
        
        // If not in header, check if user has tenant in JWT claims
        if tenantID == "" {
            if claims, ok := r.Context().Value(UserContextKey).(*Claims); ok {
                tenantID = claims.TenantID
            }
        }

        if tenantID == "" {
            utils.RespondWithError(w, http.StatusBadRequest, "Tenant ID is required")
            return
        }

        // Add tenant ID to context
        ctx := context.WithValue(r.Context(), TenantContextKey, tenantID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// RequireRole middleware checks if user has required role
func RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, ok := r.Context().Value(UserContextKey).(*Claims)
            if !ok {
                utils.RespondWithError(w, http.StatusUnauthorized, "User not authenticated")
                return
            }

            // Check if user has required role
            hasRole := false
            for _, userRole := range claims.Roles {
                for _, requiredRole := range roles {
                    if userRole == requiredRole {
                        hasRole = true
                        break
                    }
                }
                if hasRole {
                    break
                }
            }

            if !hasRole {
                utils.RespondWithError(w, http.StatusForbidden, "Insufficient permissions")
                return
            }

            next.ServeHTTP(w, r.WithContext(r.Context()))
        })
    }
}

// GetUserFromContext extracts user claims from context
func GetUserFromContext(ctx context.Context) (*Claims, error) {
    claims, ok := ctx.Value(UserContextKey).(*Claims)
    if !ok {
        return nil, fmt.Errorf("user not found in context")
    }
    return claims, nil
}

// GetTenantFromContext extracts tenant ID from context
func GetTenantFromContext(ctx context.Context) (string, error) {
    tenantID, ok := ctx.Value(TenantContextKey).(string)
    if !ok {
        return "", fmt.Errorf("tenant not found in context")
    }
    return tenantID, nil
}

// AuditLog middleware logs all API requests
func AuditLog(logger *zap.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Create response writer wrapper to capture status code
            rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
            
            // Process request
            next.ServeHTTP(rw, r)
            
            // Log request
            duration := time.Since(start)
            
            logger.Info("API Request",
                zap.String("method", r.Method),
                zap.String("path", r.URL.Path),
                zap.Int("status", rw.statusCode),
                zap.Duration("duration", duration),
                zap.String("remote_addr", r.RemoteAddr),
                zap.String("user_agent", r.UserAgent()),
            )
        })
    }
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}
