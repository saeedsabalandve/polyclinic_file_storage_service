package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "github.com/joho/godotenv"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/config"
    internalMiddleware "github.com/polyclinic/file-storage-service/internal/middleware"
    "github.com/polyclinic/file-storage-service/internal/handler"
    "github.com/polyclinic/file-storage-service/internal/service"
    "github.com/polyclinic/file-storage-service/internal/repository"
)

func main() {
    // Load environment variables from .env file if exists
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, using system environment variables")
    }

    // Initialize logger
    logger, err := zap.NewProduction()
    if err != nil {
        log.Fatalf("Failed to initialize logger: %v", err)
    }
    defer logger.Sync()

    // Load configuration
    cfg, err := config.LoadConfig()
    if err != nil {
        logger.Fatal("Failed to load configuration", zap.Error(err))
    }

    // Initialize database connection
    dbPool, err := repository.NewDBPool(context.Background(), cfg.DatabaseURL)
    if err != nil {
        logger.Fatal("Failed to connect to database", zap.Error(err))
    }
    defer dbPool.Close()

    // Initialize Redis client if configured
    var redisClient *repository.RedisClient
    if cfg.RedisURL != "" {
        redisClient, err = repository.NewRedisClient(cfg.RedisURL)
        if err != nil {
            logger.Warn("Failed to connect to Redis, continuing without cache", zap.Error(err))
        }
        defer redisClient.Close()
    }

    // Initialize S3 client
    s3Client, err := repository.NewS3Client(cfg)
    if err != nil {
        logger.Fatal("Failed to initialize S3 client", zap.Error(err))
    }

    // Initialize repositories
    metadataRepo := repository.NewMetadataRepository(dbPool)
    tenantRepo := repository.NewTenantRepository(dbPool)

    // Initialize services
    storageService := service.NewStorageService(s3Client, cfg, logger)
    metadataService := service.NewMetadataService(metadataRepo, redisClient, logger)
    tenantService := service.NewTenantService(tenantRepo, cfg, logger)

    // Initialize handlers
    uploadHandler := handler.NewUploadHandler(storageService, metadataService, logger)
    downloadHandler := handler.NewDownloadHandler(storageService, metadataService, logger)
    deleteHandler := handler.NewDeleteHandler(storageService, metadataService, logger)
    metadataHandler := handler.NewMetadataHandler(metadataService, logger)
    healthHandler := handler.NewHealthHandler(dbPool, s3Client, redisClient)
    adminHandler := handler.NewAdminHandler(tenantService, storageService, logger)

    // Initialize router
    r := chi.NewRouter()

    // Global middleware
    r.Use(middleware.RequestID)
    r.Use(middleware.RealIP)
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(middleware.Timeout(60 * time.Second))
    
    // CORS configuration
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   cfg.AllowedOrigins,
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Tenant-ID"},
        ExposedHeaders:   []string{"Link"},
        AllowCredentials: true,
        MaxAge:           300,
    }))

    // Health check routes (no auth required)
    r.Get("/health", healthHandler.Health)
    r.Get("/health/ready", healthHandler.Ready)
    r.Get("/metrics", healthHandler.Metrics)

    // API v1 routes
    r.Route("/api/v1", func(r chi.Router) {
        // Auth middleware for all API routes
        r.Use(internalMiddleware.JWTAuth(cfg.JWTSecret))
        r.Use(internalMiddleware.TenantResolver)
        
        // Rate limiting
        if cfg.RateLimitEnabled {
            r.Use(internalMiddleware.RateLimiter(redisClient, cfg.RateLimit))
        }

        // File routes
        r.Route("/files", func(r chi.Router) {
            // Upload routes
            r.Post("/upload", uploadHandler.Upload)
            r.Post("/upload/initiate", uploadHandler.InitiateMultipartUpload)
            r.Put("/upload/{uploadId}/part", uploadHandler.UploadPart)
            r.Post("/upload/{uploadId}/complete", uploadHandler.CompleteMultipartUpload)
            r.Delete("/upload/{uploadId}", uploadHandler.AbortMultipartUpload)

            // Download routes
            r.Get("/{fileId}/download", downloadHandler.Download)
            r.Get("/{fileId}/presigned", downloadHandler.GeneratePresignedURL)

            // Metadata routes
            r.Get("/{fileId}", metadataHandler.GetFileMetadata)
            r.Put("/{fileId}/metadata", metadataHandler.UpdateMetadata)
            r.Get("/list", metadataHandler.ListFiles)
            r.Get("/search", metadataHandler.SearchFiles)

            // Delete route
            r.Delete("/{fileId}", deleteHandler.Delete)
            r.Post("/batch-delete", deleteHandler.BatchDelete)
        })

        // Admin routes (requires admin role)
        r.Route("/admin", func(r chi.Router) {
            r.Use(internalMiddleware.RequireRole("admin"))
            
            r.Get("/stats", adminHandler.GetStorageStats)
            r.Get("/tenants", adminHandler.ListTenants)
            r.Post("/tenants", adminHandler.CreateTenant)
            r.Put("/tenants/{tenantId}", adminHandler.UpdateTenant)
            r.Delete("/tenants/{tenantId}", adminHandler.DeleteTenant)
            r.Get("/tenants/{tenantId}/usage", adminHandler.GetTenantUsage)
            r.Get("/audit-logs", adminHandler.GetAuditLogs)
        })
    })

    // Configure server
    srv := &http.Server{
        Addr:         fmt.Sprintf(":%s", cfg.ServerPort),
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Start server in goroutine
    go func() {
        logger.Info("Starting server", zap.String("port", cfg.ServerPort))
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatal("Server failed to start", zap.Error(err))
        }
    }()

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    logger.Info("Server is shutting down...")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        logger.Fatal("Server forced to shutdown", zap.Error(err))
    }

    logger.Info("Server exited properly")
}
