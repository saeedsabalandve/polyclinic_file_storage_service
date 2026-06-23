-- migrations/001_initial_schema.sql
-- Initial database schema for Polyclinic File Storage Service

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create tenants table
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    
    -- Storage configuration
    bucket_name VARCHAR(255) UNIQUE NOT NULL,
    storage_quota BIGINT NOT NULL DEFAULT 1099511627776, -- 1TB in bytes
    storage_used BIGINT NOT NULL DEFAULT 0,
    max_file_size BIGINT NOT NULL DEFAULT 2147483648, -- 2GB in bytes
    
    -- Features
    allowed_types TEXT[] DEFAULT ARRAY['dicom', 'pdf', 'jpg', 'png'],
    enable_encryption BOOLEAN DEFAULT true,
    encryption_type VARCHAR(50) DEFAULT 'SSE-S3',
    kms_key_arn VARCHAR(500),
    
    -- Retention
    retention_days INTEGER DEFAULT 365,
    auto_archive_days INTEGER DEFAULT 90,
    
    -- Backup
    backup_enabled BOOLEAN DEFAULT false,
    backup_frequency VARCHAR(50) DEFAULT 'daily',
    backup_retention INTEGER DEFAULT 30,
    
    -- CDN
    cdn_enabled BOOLEAN DEFAULT false,
    cdn_domain VARCHAR(255),
    
    -- CORS
    cors_origins TEXT[],
    
    -- Lifecycle rules (JSON)
    lifecycle_rules JSONB,
    
    -- Metadata
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_status CHECK (status IN ('active', 'inactive', 'suspended', 'deleted')),
    CONSTRAINT valid_encryption_type CHECK (encryption_type IN ('SSE-S3', 'SSE-KMS')),
    CONSTRAINT positive_quota CHECK (storage_quota > 0),
    CONSTRAINT positive_max_file_size CHECK (max_file_size > 0)
);

-- Create files table
CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    filename VARCHAR(500) NOT NULL,
    original_name VARCHAR(500) NOT NULL,
    bucket_name VARCHAR(255) NOT NULL,
    object_key VARCHAR(1000) NOT NULL,
    size BIGINT NOT NULL,
    content_type VARCHAR(255),
    md5_hash VARCHAR(32),
    sha256_hash VARCHAR(64),
    encryption_type VARCHAR(50) DEFAULT 'SSE-S3',
    version_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    tags JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    uploaded_by VARCHAR(255),
    updated_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT valid_file_status CHECK (status IN ('active', 'archived', 'deleted', 'expired')),
    CONSTRAINT unique_tenant_object UNIQUE (tenant_id, object_key, version_id)
);

-- Create multipart_uploads table
CREATE TABLE IF NOT EXISTS multipart_uploads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    upload_id VARCHAR(255) NOT NULL,
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    filename VARCHAR(500) NOT NULL,
    bucket_name VARCHAR(255) NOT NULL,
    object_key VARCHAR(1000) NOT NULL,
    part_count INTEGER DEFAULT 0,
    total_size BIGINT DEFAULT 0,
    parts JSONB DEFAULT '[]',
    status VARCHAR(50) NOT NULL DEFAULT 'in_progress',
    initiated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT valid_upload_status CHECK (status IN ('in_progress', 'completed', 'aborted'))
);

-- Create audit_logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    details JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'success',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_audit_status CHECK (status IN ('success', 'failure', 'denied'))
);

-- Create file_versions table for version history
CREATE TABLE IF NOT EXISTS file_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    version_id VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL,
    content_type VARCHAR(255),
    md5_hash VARCHAR(32),
    sha256_hash VARCHAR(64),
    created_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes
CREATE INDEX idx_files_tenant_id ON files(tenant_id);
CREATE INDEX idx_files_status ON files(status);
CREATE INDEX idx_files_created_at ON files(created_at DESC);
CREATE INDEX idx_files_metadata ON files USING GIN (metadata);
CREATE INDEX idx_files_tags ON files USING GIN (tags);
CREATE INDEX idx_files_patient_id ON files((metadata->>'patient_id'));
CREATE INDEX idx_files_study_type ON files((metadata->>'study_type'));

CREATE INDEX idx_audit_logs_tenant_id ON audit_logs(tenant_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);

CREATE INDEX idx_multipart_uploads_upload_id ON multipart_uploads(upload_id);
CREATE INDEX idx_multipart_uploads_status ON multipart_uploads(status);

CREATE INDEX idx_file_versions_file_id ON file_versions(file_id);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_tenants_updated_at
    BEFORE UPDATE ON tenants
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_files_updated_at
    BEFORE UPDATE ON files
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Create function to update storage_used in tenants
CREATE OR REPLACE FUNCTION update_tenant_storage_used()
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'INSERT') THEN
        UPDATE tenants
        SET storage_used = storage_used + NEW.size
        WHERE id = NEW.tenant_id;
    ELSIF (TG_OP = 'DELETE') THEN
        UPDATE tenants
        SET storage_used = storage_used - OLD.size
        WHERE id = OLD.tenant_id;
    ELSIF (TG_OP = 'UPDATE') THEN
        UPDATE tenants
        SET storage_used = storage_used - OLD.size + NEW.size
        WHERE id = NEW.tenant_id;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger for storage tracking
CREATE TRIGGER update_storage_usage
    AFTER INSERT OR DELETE OR UPDATE OF size
    ON files
    FOR EACH ROW
    EXECUTE FUNCTION update_tenant_storage_used();

-- Insert default tenant
INSERT INTO tenants (name, slug, bucket_name) 
VALUES ('Default Clinic', 'default-clinic', 'polyclinic-default-clinic-storage')
ON CONFLICT (slug) DO NOTHING;
