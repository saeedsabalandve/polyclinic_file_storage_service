package handler

import (
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "go.uber.org/zap"

    "github.com/polyclinic/file-storage-service/internal/middleware"
    "github.com/polyclinic/file-storage-service/internal/model"
    "github.com/polyclinic/file-storage-service/internal/service"
    "github.com/polyclinic/file-storage-service/internal/utils"
)

// AdminHandler handles admin endpoints
type AdminHandler struct {
    tenantService  *service.TenantService
    storageService *service.StorageService
    logger         *zap.Logger
}

// NewAdminHandler creates a new AdminHandler
func NewAdminHandler(tenantSvc *service.TenantService, storageSvc *service.StorageService, logger *zap.Logger) *AdminHandler {
    return &AdminHandler{
        tenantService:  tenantSvc,
        storageService: storageSvc,
        logger:         logger,
    }
}

// GetStorageStats returns storage statistics
func (h *AdminHandler) GetStorageStats(w http.ResponseWriter, r *http.Request) {
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Storage stats endpoint",
    })
}

// ListTenants returns a list of all tenants
func (h *AdminHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page < 1 {
        page = 1
    }

    pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
    if pageSize < 1 || pageSize > 100 {
        pageSize = 20
    }

    tenants, totalCount, err := h.tenantService.ListTenants(r.Context(), page, pageSize)
    if err != nil {
        h.logger.Error("Failed to list tenants", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to list tenants")
        return
    }

    meta := utils.CreatePaginationMeta(page, pageSize, totalCount)
    utils.RespondWithPaginatedJSON(w, http.StatusOK, tenants, meta)
}

// CreateTenant creates a new tenant
func (h *AdminHandler) CreateTenant(w http.ResponseWriter, r *http.Request) {
    claims, err := middleware.GetUserFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusUnauthorized, "User not authenticated")
        return
    }

    var tenant model.Tenant
    if err := utils.DecodeJSONBody(r, &tenant); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    if tenant.Name == "" || tenant.Slug == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Name and slug are required")
        return
    }

    tenant.CreatedBy = claims.UserID

    if err := h.tenantService.CreateTenant(r.Context(), &tenant); err != nil {
        h.logger.Error("Failed to create tenant", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create tenant")
        return
    }

    // Create S3 bucket for the tenant
    if err := h.storageService.CreateTenantBucket(r.Context(), tenant.ID); err != nil {
        h.logger.Error("Failed to create tenant bucket", zap.Error(err))
        // Don't fail the request, but log the error
    }

    utils.RespondWithJSON(w, http.StatusCreated, tenant)
}

// UpdateTenant updates a tenant
func (h *AdminHandler) UpdateTenant(w http.ResponseWriter, r *http.Request) {
    claims, err := middleware.GetUserFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusUnauthorized, "User not authenticated")
        return
    }

    tenantID := chi.URLParam(r, "tenantId")
    if tenantID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant ID is required")
        return
    }

    var updates model.Tenant
    if err := utils.DecodeJSONBody(r, &updates); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    existingTenant, err := h.tenantService.GetTenant(r.Context(), tenantID)
    if err != nil {
        utils.RespondWithError(w, http.StatusNotFound, "Tenant not found")
        return
    }

    // Apply updates
    if updates.Name != "" {
        existingTenant.Name = updates.Name
    }
    if updates.StorageQuota > 0 {
        existingTenant.StorageQuota = updates.StorageQuota
    }
    if updates.MaxFileSize > 0 {
        existingTenant.MaxFileSize = updates.MaxFileSize
    }
    if updates.RetentionDays > 0 {
        existingTenant.RetentionDays = updates.RetentionDays
    }
    if updates.AutoArchiveDays > 0 {
        existingTenant.AutoArchiveDays = updates.AutoArchiveDays
    }

    existingTenant.UpdatedBy = claims.UserID

    if err := h.tenantService.UpdateTenant(r.Context(), existingTenant); err != nil {
        h.logger.Error("Failed to update tenant", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update tenant")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, existingTenant)
}

// DeleteTenant deletes a tenant
func (h *AdminHandler) DeleteTenant(w http.ResponseWriter, r *http.Request) {
    tenantID := chi.URLParam(r, "tenantId")
    if tenantID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant ID is required")
        return
    }

    if err := h.tenantService.DeleteTenant(r.Context(), tenantID); err != nil {
        h.logger.Error("Failed to delete tenant", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete tenant")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Tenant deleted successfully",
        "tenant_id": tenantID,
    })
}

// GetTenantUsage returns tenant storage usage
func (h *AdminHandler) GetTenantUsage(w http.ResponseWriter, r *http.Request) {
    tenantID := chi.URLParam(r, "tenantId")
    if tenantID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant ID is required")
        return
    }

    usage, err := h.tenantService.GetTenantUsage(r.Context(), tenantID)
    if err != nil {
        h.logger.Error("Failed to get tenant usage", zap.Error(err))
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get tenant usage")
        return
    }

    utils.RespondWithJSON(w, http.StatusOK, usage)
}

// GetAuditLogs returns audit logs
func (h *AdminHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
    tenantID, err := middleware.GetTenantFromContext(r.Context())
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Tenant not found")
        return
    }

    page, _ := strconv.Atoi(r.URL.Query().Get("page"))
    if page < 1 {
        page = 1
    }

    pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
    if pageSize < 1 || pageSize > 100 {
        pageSize = 50
    }

    utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
        "message":   "Audit logs endpoint",
        "tenant_id": tenantID,
        "page":      page,
        "page_size": pageSize,
    })
}
