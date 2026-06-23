#!/bin/bash
# scripts/init-s3-buckets.sh
# Initialize S3 buckets for the file storage service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
AWS_REGION=${AWS_REGION:-"us-east-1"}
BUCKET_PREFIX=${BUCKET_PREFIX:-"polyclinic"}
TENANT_ID=${1:-"default-tenant"}

echo -e "${GREEN}Initializing S3 buckets...${NC}"

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo -e "${RED}AWS CLI is not installed. Please install it first.${NC}"
    exit 1
fi

# Create main bucket
BUCKET_NAME="${BUCKET_PREFIX}-${TENANT_ID}-storage"
echo -e "${YELLOW}Creating bucket: ${BUCKET_NAME}${NC}"

if aws s3api head-bucket --bucket "${BUCKET_NAME}" 2>/dev/null; then
    echo -e "${YELLOW}Bucket already exists.${NC}"
else
    # Create bucket
    if [ "${AWS_REGION}" = "us-east-1" ]; then
        aws s3api create-bucket \
            --bucket "${BUCKET_NAME}" \
            --region "${AWS_REGION}"
    else
        aws s3api create-bucket \
            --bucket "${BUCKET_NAME}" \
            --region "${AWS_REGION}" \
            --create-bucket-configuration LocationConstraint="${AWS_REGION}"
    fi
    echo -e "${GREEN}Bucket created successfully.${NC}"
fi

# Enable versioning
echo -e "${YELLOW}Enabling versioning...${NC}"
aws s3api put-bucket-versioning \
    --bucket "${BUCKET_NAME}" \
    --versioning-configuration Status=Enabled

# Enable encryption
echo -e "${YELLOW}Enabling default encryption...${NC}"
aws s3api put-bucket-encryption \
    --bucket "${BUCKET_NAME}" \
    --server-side-encryption-configuration '{
        "Rules": [
            {
                "ApplyServerSideEncryptionByDefault": {
                    "SSEAlgorithm": "AES256"
                }
            }
        ]
    }'

# Block public access
echo -e "${YELLOW}Blocking public access...${NC}"
aws s3api put-public-access-block \
    --bucket "${BUCKET_NAME}" \
    --public-access-block-configuration \
        "BlockPublicAcls=true,IgnorePublicAcls=true,BlockPublicPolicy=true,RestrictPublicBuckets=true"

# Set lifecycle rules
echo -e "${YELLOW}Setting lifecycle rules...${NC}"
aws s3api put-bucket-lifecycle-configuration \
    --bucket "${BUCKET_NAME}" \
    --lifecycle-configuration '{
        "Rules": [
            {
                "ID": "TransitionToIA",
                "Status": "Enabled",
                "Filter": {
                    "Prefix": ""
                },
                "Transitions": [
                    {
                        "Days": 30,
                        "StorageClass": "STANDARD_IA"
                    }
                ]
            },
            {
                "ID": "ExpireOldVersions",
                "Status": "Enabled",
                "Filter": {
                    "Prefix": ""
                },
                "NoncurrentVersionExpiration": {
                    "NoncurrentDays": 90
                }
            }
        ]
    }'

# Set CORS configuration
echo -e "${YELLOW}Setting CORS configuration...${NC}"
aws s3api put-bucket-cors \
    --bucket "${BUCKET_NAME}" \
    --cors-configuration '{
        "CORSRules": [
            {
                "AllowedHeaders": ["*"],
                "AllowedMethods": ["GET", "PUT", "POST", "DELETE"],
                "AllowedOrigins": ["*"],
                "ExposeHeaders": ["ETag"],
                "MaxAgeSeconds": 3000
            }
        ]
    }'

# Add tags
echo -e "${YELLOW}Adding tags...${NC}"
aws s3api put-bucket-tagging \
    --bucket "${BUCKET_NAME}" \
    --tagging "TagSet=[{Key=tenant,Value=${TENANT_ID}},{Key=environment,Value=${ENVIRONMENT:-development}},{Key=service,Value=file-storage}]"

echo -e "${GREEN}Bucket initialization complete!${NC}"
echo -e "${GREEN}Bucket name: ${BUCKET_NAME}${NC}"
