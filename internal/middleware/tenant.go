package middleware

import (
    "context"
    "net/http"
    "sync"
    "time"
)

// TenantConfig holds tenant-specific configuration
type TenantConfig struct {
    mu               sync.RWMutex
    tenants          map[string]*TenantInfo
    refreshInterval  time.Duration
}

// TenantInfo holds cached tenant information
type TenantInfo struct {
    ID           string
    Name         string
    Status       string
    BucketName   string
    StorageQuota int64
    MaxFileSize  int64
    Features     map[string]bool
}

// NewTenantConfig creates a new tenant configuration manager
func NewTenantConfig(refreshInterval time.Duration) *TenantConfig {
    tc := &TenantConfig{
        tenants:         make(map[string]*TenantInfo),
        refreshInterval: refreshInterval,
    }
    
    // Start background refresh
    go tc.startBackgroundRefresh()
    
    return tc
}

// GetTenant retrieves tenant information
func (tc *TenantConfig) GetTenant(tenantID string) (*TenantInfo, bool) {
    tc.mu.RLock()
    defer tc.mu.RUnlock()
    
    tenant, exists := tc.tenants[tenantID]
    return tenant, exists
}

// SetTenant sets tenant information in cache
func (tc *TenantConfig) SetTenant(tenant *TenantInfo) {
    tc.mu.Lock()
    defer tc.mu.Unlock()
    
    tc.tenants[tenant.ID] = tenant
}

// RemoveTenant removes tenant from cache
func (tc *TenantConfig) RemoveTenant(tenantID string) {
    tc.mu.Lock()
    defer tc.mu.Unlock()
    
    delete(tc.tenants, tenantID)
}

// IsTenantActive checks if tenant is active
func (tc *TenantConfig) IsTenantActive(tenantID string) bool {
    tenant, exists := tc.GetTenant(tenantID)
    if !exists {
        return false
    }
    return tenant.Status == "active"
}

// HasFeature checks if tenant has a specific feature enabled
func (tc *TenantConfig) HasFeature(tenantID, feature string) bool {
    tenant, exists := tc.GetTenant(tenantID)
    if !exists {
        return false
    }
    enabled, ok := tenant.Features[feature]
    return ok && enabled
}

// CheckStorageQuota checks if tenant has enough storage quota
func (tc *TenantConfig) CheckStorageQuota(tenantID string, additionalSize int64) bool {
    tenant, exists := tc.GetTenant(tenantID)
    if !exists {
        return false
    }
    // This would need to check actual usage from database
    // Simplified for example
    return tenant.StorageQuota >= additionalSize
}

// CheckFileSizeLimit checks if file size is within tenant limit
func (tc *TenantConfig) CheckFileSizeLimit(tenantID string, fileSize int64) bool {
    tenant, exists := tc.GetTenant(tenantID)
    if !exists {
        return false
    }
    return fileSize <= tenant.MaxFileSize
}

// TenantValidationMiddleware validates tenant and checks permissions
func (tc *TenantConfig) TenantValidationMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tenantID := r.Context().Value(TenantContextKey)
            if tenantID == nil {
                http.Error(w, "Tenant not found in context", http.StatusInternalServerError)
                return
            }

            // Check if tenant exists and is active
            if !tc.IsTenantActive(tenantID.(string)) {
                http.Error(w, "Tenant is not active or does not exist", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func (tc *TenantConfig) startBackgroundRefresh() {
    ticker := time.NewTicker(tc.refreshInterval)
    defer ticker.Stop()

    for range ticker.C {
        // In production, this would fetch tenant configs from database
        // and update the cache
        tc.mu.Lock()
        // Refresh logic here
        tc.mu.Unlock()
    }
}

// TenantContextKey is used to store tenant in context
var TenantContextKey = struct{ string }{"tenant"}
