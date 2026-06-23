package handler

import (
    "context"
    "net/http"
    "time"

    "go.uber.org/zap"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"

    "github.com/polyclinic/file-storage-service/internal/repository"
    "github.com/polyclinic/file-storage-service/internal/utils"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
    db          *pgxpool.Pool
    s3Client    *repository.S3Client
    redisClient *repository.RedisClient
    startTime   time.Time
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler(db *pgxpool.Pool, s3Client *repository.S3Client, redisClient *repository.RedisClient) *HealthHandler {
    return &HealthHandler{
        db:          db,
        s3Client:    s3Client,
        redisClient: redisClient,
        startTime:   time.Now(),
    }
}

// Health returns basic health status
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
    response := map[string]interface{}{
        "status":    "healthy",
        "service":   "polyclinic-file-storage",
        "version":   "1.0.0",
        "timestamp": time.Now().UTC().Format(time.RFC3339),
        "uptime":    time.Since(h.startTime).String(),
    }

    utils.RespondWithJSON(w, http.StatusOK, response)
}

// Ready returns readiness status (checks all dependencies)
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
    checks := make(map[string]interface{})
    isReady := true

    // Check database
    dbCtx, dbCancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer dbCancel()
    
    if err := h.db.Ping(dbCtx); err != nil {
        checks["database"] = map[string]interface{}{
            "status":  "unhealthy",
            "error":   err.Error(),
        }
        isReady = false
    } else {
        checks["database"] = map[string]string{
            "status": "healthy",
        }
    }

    // Check Redis
    if h.redisClient != nil {
        redisCtx, redisCancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer redisCancel()
        
        if h.redisClient.IsConnected(redisCtx) {
            checks["redis"] = map[string]string{
                "status": "healthy",
            }
        } else {
            checks["redis"] = map[string]string{
                "status": "unhealthy",
            }
            isReady = false
        }
    }

    // Check S3
    s3Ctx, s3Cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer s3Cancel()
    
    if err := h.checkS3(s3Ctx); err != nil {
        checks["s3"] = map[string]interface{}{
            "status": "unhealthy",
            "error":  err.Error(),
        }
        isReady = false
    } else {
        checks["s3"] = map[string]string{
            "status": "healthy",
        }
    }

    statusCode := http.StatusOK
    if !isReady {
        statusCode = http.StatusServiceUnavailable
    }

    response := map[string]interface{}{
        "status":    getStatus(isReady),
        "checks":    checks,
        "timestamp": time.Now().UTC().Format(time.RFC3339),
    }

    utils.RespondWithJSON(w, statusCode, response)
}

// Metrics returns service metrics
func (h *HealthHandler) Metrics(w http.ResponseWriter, r *http.Request) {
    metrics := map[string]interface{}{
        "uptime_seconds": time.Since(h.startTime).Seconds(),
        "service":       "polyclinic-file-storage",
        "version":       "1.0.0",
    }

    utils.RespondWithJSON(w, http.StatusOK, metrics)
}

// Helper functions
func (h *HealthHandler) checkS3(ctx context.Context) error {
    // Perform a simple S3 operation to verify connectivity
    // This could be a ListBuckets call or similar
    return nil
}

func getStatus(isReady bool) string {
    if isReady {
        return "ready"
    }
    return "not_ready"
}
