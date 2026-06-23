# Polyclinic File Storage Microservice 🏥

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![AWS S3](https://img.shields.io/badge/AWS-S3-FF9900?style=flat&logo=amazon-aws)](https://aws.amazon.com/s3/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com/)

A high-performance, multi-tenant microservice for storing and managing large medical files in a polyclinic environment, built on AWS S3 with tenant-based authorization.

## 📋 Table of Contents
- [Overview](#overview)
- [Architecture](#architecture)
- [Features](#features)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Documentation](#api-documentation)
- [Multi-Tenancy](#multi-tenancy)
- [Security](#security)
- [Deployment](#deployment)
- [Monitoring](#monitoring)
- [Contributing](#contributing)
- [License](#license)

## 🎯 Overview

This microservice provides a scalable solution for polyclinics to store and manage large medical files (X-rays, MRI scans, CT scans, lab reports, etc.) with strict tenant isolation. Each clinic (tenant) has its own dedicated storage space with role-based access control.

### Use Cases
- Medical imaging storage (DICOM files)
- Lab report management
- Patient documentation archive
- Telemedicine file sharing
- Medical record backup

## 🏗 Architecture

┌─────────────────────────────────────────────────────────────┐
│                      API Gateway/Load Balancer               │
└────────────────────┬────────────────────────────────────────┘
│
┌────────────────────▼────────────────────────────────────────┐
│                    Auth Middleware                           │
│            (JWT Validation + Tenant Resolution)              │
└────────────────────┬────────────────────────────────────────┘
│
┌────────────────────▼────────────────────────────────────────┐
│                  File Storage Service                        │
├──────────────┬──────────────┬──────────────┬────────────────┤
│  Upload      │  Download    │  Delete      │  Metadata      │
│  Handler     │  Handler     │  Handler     │  Handler       │
└──────┬───────┴──────┬───────┴──────┬───────┴───────┬────────┘
│              │              │               │
┌──────▼──────────────▼──────────────▼───────────────▼────────┐
│                    Service Layer                            │
│  ┌────────────────┐              ┌──────────────────┐      │
│  │  S3 Service    │              │ Metadata Service │      │
│  │  (AWS SDK)     │              │   (PostgreSQL)   │      │
│  └────────────────┘              └──────────────────┘      │
└─────────────────────┬──────────────────┬───────────────────┘
│                  │
┌─────────────────────▼──┐   ┌──────────▼──────────────────┐
│    AWS S3 Buckets      │   │   PostgreSQL Database        │
│  ┌──────────────────┐  │   │  ┌──────────────────────┐   │
│  │ Tenant A Bucket  │  │   │  │ file_metadata        │   │
│  ├──────────────────┤  │   │  ├──────────────────────┤   │
│  │ Tenant B Bucket  │  │   │  │ tenant_config        │   │
│  ├──────────────────┤  │   │  ├──────────────────────┤   │
│  │ Tenant C Bucket  │  │   │  │ audit_logs           │   │
│  └──────────────────┘  │   │  └──────────────────────┘   │
└────────────────────────┘   └─────────────────────────────┘

## ✨ Features

### Core Features
- **Multi-Tenant Architecture**: Complete data isolation per clinic
- **Large File Support**: Handles files up to 5GB with multipart upload
- **Resumable Uploads**: Support for paused/resumed uploads
- **Pre-signed URLs**: Secure temporary access links
- **Metadata Management**: Rich file metadata with search capabilities
- **Version Control**: File versioning and history tracking
- **CORS Support**: Cross-origin resource sharing configured
- **CDN Integration**: CloudFront support for faster delivery

### Security Features
- **JWT-based Authentication**: Token validation for all endpoints
- **RBAC (Role-Based Access Control)**:
  - Admin: Full access
  - Doctor: Read/Write for assigned patients
  - Nurse: Read-only for assigned patients
  - Patient: Read-only for own files
- **Tenant Isolation**: Each clinic's data is completely segregated
- **Encryption**: 
  - At-rest: SSE-S3 / SSE-KMS
  - In-transit: TLS 1.3
- **Audit Logging**: Comprehensive access and modification logs
- **Rate Limiting**: Per-tenant rate limiting to prevent abuse

### Compliance
- **HIPAA Ready**: Architecture supports HIPAA compliance requirements
- **GDPR Compliant**: Data residency and deletion capabilities
- **Audit Trail**: Complete audit logs for compliance reporting

## 🛠 Tech Stack

| Component       | Technology                          | Version    |
|-----------------|-------------------------------------|------------|
| Language        | Go                                  | 1.21+      |
| Framework       | Chi Router / Gin                    | Latest     |
| Database        | PostgreSQL                          | 15+        |
| ORM             | GORM / sqlx                         | Latest     |
| Cache           | Redis                               | 7+         |
| Storage         | AWS S3                             | Latest SDK |
| Authentication  | JWT                                 | -          |
| Container       | Docker                             | 24+        |
| Orchestration   | Kubernetes / ECS                    | -          |
| Monitoring      | Prometheus + Grafana                | Latest     |
| Logging         | ELK Stack / CloudWatch              | -          |
| CI/CD           | GitHub Actions / GitLab CI          | -          |


## 🛠 Tech Stack

| Component       | Technology                          | Version    |
|-----------------|-------------------------------------|------------|
| Language        | Go                                  | 1.21+      |
| Framework       | Chi Router / Gin                    | Latest     |
| Database        | PostgreSQL                          | 15+        |
| ORM             | GORM / sqlx                         | Latest     |
| Cache           | Redis                               | 7+         |
| Storage         | AWS S3                             | Latest SDK |
| Authentication  | JWT                                 | -          |
| Container       | Docker                             | 24+        |
| Orchestration   | Kubernetes / ECS                    | -          |
| Monitoring      | Prometheus + Grafana                | Latest     |
| Logging         | ELK Stack / CloudWatch              | -          |
| CI/CD           | GitHub Actions / GitLab CI          | -          |


## 📋 Prerequisites

- Go 1.21 or higher
- Docker & Docker Compose
- AWS Account with S3 access
- PostgreSQL 15+
- Redis 7+ (optional, for caching)
- Make (optional)

## 🚀 Quick Start

### Local Development

1. **Clone the repository**
```bash
git clone https://github.com/your-org/polyclinic-file-storage-service.git
cd polyclinic-file-storage-service
```

2. Copy environment file

```bash
cp .env.example .env
# Edit .env with your configurations
```

3. Start dependencies

```bash
docker-compose up -d postgres redis
```

4. Run database migrations

```bash
make migrate-up
```

5. Start the service

```bash
make run
```

Using Docker

```bash
# Build and run with Docker Compose
docker-compose up -d

# Check logs
docker-compose logs -f file-storage-service
```

Quick Test

```bash
# Health check
curl http://localhost:8080/health

# Upload a file (requires authentication)
curl -X POST http://localhost:8080/api/v1/files/upload \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "X-Tenant-ID: tenant_123" \
  -F "file=@sample_medical_image.dcm"
```


⚙️ Configuration

Environment Variables

Variable Description Default Required
SERVER_PORT Server port 8080 No
ENVIRONMENT Environment (dev/staging/prod) development No
AWS_REGION AWS region us-east-1 Yes
AWS_ACCESS_KEY_ID AWS access key - Yes*
AWS_SECRET_ACCESS_KEY AWS secret key - Yes*
S3_BUCKET_PREFIX Prefix for tenant buckets polyclinic No
S3_PRESIGNED_URL_EXPIRY Pre-signed URL expiry (minutes) 60 No
DATABASE_URL PostgreSQL connection string - Yes
REDIS_URL Redis connection string - No
JWT_SECRET JWT signing secret - Yes
MAX_FILE_SIZE Maximum file size in MB 5120 No
ALLOWED_EXTENSIONS Comma-separated allowed extensions dcm,jpg,png,pdf,dicom No
ENABLE_AUDIT_LOG Enable audit logging true No

*Not required if using IAM roles

Tenant Configuration

# Example tenant configuration
tenant_config:
  clinic_id: "clinic_001"
  storage_quota: "1TB"
  max_file_size: "2GB"
  allowed_types: ["dicom", "pdf", "jpg", "png"]
  retention_policy:
    default_days: 365
    archive_after: 90
  encryption:
    type: "SSE-KMS"
    kms_key_id: "arn:aws:kms:region:account:key/key-id"
  backup:
    enabled: true
    frequency: "daily"
    retention: "30 days"

    🏢 Multi-Tenancy

Tenant Isolation Strategy

· Database-Level: Separate schemas or databases per tenant
· Storage-Level: Dedicated S3 buckets with tenant-specific prefixes
· Access Control: IAM policies scoped to tenant buckets

Tenant Onboarding

1. Create tenant record in database
2. Provision S3 bucket: polyclinic-{tenant-id}-storage
3. Apply bucket policies and CORS
4. Configure lifecycle rules
5. Set up CloudFront distribution (optional)

```bash
# Automated tenant provisioning
make create-tenant TENANT_ID=clinic_001 TENANT_NAME="City Medical Center"
```


🔒 Security

Authentication Flow

```
Client → JWT Token → API Gateway → Token Validation → Tenant Resolution → Resource Access
```

IAM Policy Example

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject"
      ],
      "Resource": "arn:aws:s3:::polyclinic-${tenant_id}-storage/*",
      "Condition": {
        "StringEquals": {
          "s3:x-amz-server-side-encryption": "aws:kms"
        }
      }
    }
  ]
}
```


Security Best Practices

· ✅ All API endpoints require HTTPS
· ✅ JWT tokens with short expiration (15 minutes)
· ✅ Refresh token rotation
· ✅ Request validation and sanitization
· ✅ SQL injection prevention (parameterized queries)
· ✅ XSS protection headers
· ✅ CORS whitelist per tenant
· ✅ Rate limiting per IP and tenant
· ✅ File type validation (content inspection)
· ✅ Virus scanning integration (ClamAV)

🚢 Deployment

Docker Deployment

```bash
# Build image
docker build -t polyclinic-file-storage:latest .

# Run container
docker run -d \
  --name file-storage \
  -p 8080:8080 \
  -e AWS_REGION=us-east-1 \
  -e DATABASE_URL=postgres://... \
  polyclinic-file-storage:latest
```

Kubernetes Deployment

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml
kubectl apply -f k8s/hpa.yaml
```

AWS ECS Deployment

```bash
# Deploy using AWS CLI
aws ecs update-service \
  --cluster polyclinic-cluster \
  --service file-storage-service \
  --force-new-deployment
```

📊 Monitoring

Health Metrics

· /health - Basic health check
· /health/ready - Readiness probe
· /metrics - Prometheus metrics endpoint

Key Metrics

· Upload success/failure rate
· Average upload/download time
· Storage utilization per tenant
· Active upload sessions
· API response times
· Error rates

Grafana Dashboard

```bash
# Import dashboard
curl -X POST http://grafana:3000/api/dashboards/import \
  -H "Content-Type: application/json" \
  -d @monitoring/grafana-dashboard.json
```

Alerting Rules

```yaml
groups:
  - name: file_storage_alerts
    rules:
      - alert: HighUploadFailureRate
        expr: rate(upload_failures[5m]) > 0.05
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "Upload failure rate exceeds 5%"
      
      - alert: StorageQuotaNearLimit
        expr: tenant_storage_usage / tenant_storage_quota > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Tenant storage usage above 90%"
```

🧪 Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run end-to-end tests
make test-e2e

# Generate coverage report
make coverage
```

📈 Performance

· Concurrent Uploads: 1000+ simultaneous uploads
· Upload Speed: Optimized for 100MB/s+ with multipart upload
· Latency: < 100ms for metadata operations
· Availability: 99.99% SLA with multi-AZ deployment
· Scalability: Auto-scaling based on upload queue depth



Development Guidelines

· Follow Go best practices and idioms
· Write tests for new features
· Update documentation accordingly
· Ensure CI pipeline passes
· Add proper logging and metrics
