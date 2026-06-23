#!/bin/bash
# scripts/deploy.sh
# Deployment script for Polyclinic File Storage Service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
APP_NAME="polyclinic-file-storage"
ENVIRONMENT=${1:-"staging"}
AWS_REGION=${AWS_REGION:-"us-east-1"}
ECR_REPOSITORY="${APP_NAME}"
ECS_CLUSTER="polyclinic-cluster"
ECS_SERVICE="${APP_NAME}-${ENVIRONMENT}"

echo -e "${BLUE}========================================"
echo "Deploying ${APP_NAME} to ${ENVIRONMENT}"
echo -e "========================================${NC}"

# Pre-deployment checks
echo -e "${YELLOW}Running pre-deployment checks...${NC}"

# Check AWS CLI
if ! command -v aws &> /dev/null; then
    echo -e "${RED}AWS CLI is not installed${NC}"
    exit 1
fi

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Docker is not installed${NC}"
    exit 1
fi

# Run tests
echo -e "${YELLOW}Running tests...${NC}"
go test ./... || {
    echo -e "${RED}Tests failed${NC}"
    exit 1
}

# Build the application
echo -e "${YELLOW}Building application...${NC}"
docker build -t "${APP_NAME}:latest" .

# Login to ECR
echo -e "${YELLOW}Logging into Amazon ECR...${NC}"
aws ecr get-login-password --region "${AWS_REGION}" | \
    docker login --username AWS --password-stdin "${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"

# Tag and push Docker image
ECR_IMAGE="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com/${ECR_REPOSITORY}:${ENVIRONMENT}-$(git rev-parse --short HEAD)"
echo -e "${YELLOW}Tagging and pushing image: ${ECR_IMAGE}${NC}"
docker tag "${APP_NAME}:latest" "${ECR_IMAGE}"
docker push "${ECR_IMAGE}"

# Update ECS service
echo -e "${YELLOW}Updating ECS service...${NC}"
aws ecs update-service \
    --cluster "${ECS_CLUSTER}" \
    --service "${ECS_SERVICE}" \
    --force-new-deployment \
    --region "${AWS_REGION}"

# Wait for deployment to complete
echo -e "${YELLOW}Waiting for deployment to complete...${NC}"
aws ecs wait services-stable \
    --cluster "${ECS_CLUSTER}" \
    --services "${ECS_SERVICE}" \
    --region "${AWS_REGION}"

# Run database migrations
echo -e "${YELLOW}Running database migrations...${NC}"
# Get a running task
TASK_ARN=$(aws ecs list-tasks \
    --cluster "${ECS_CLUSTER}" \
    --service-name "${ECS_SERVICE}" \
    --query 'taskArns[0]' \
    --output text \
    --region "${AWS_REGION}")

if [ "${TASK_ARN}" != "None" ]; then
    aws ecs execute-command \
        --cluster "${ECS_CLUSTER}" \
        --task "${TASK_ARN}" \
        --container "${APP_NAME}" \
        --command "/app/migrate up" \
        --interactive \
        --region "${AWS_REGION}"
fi

# Run post-deployment health checks
echo -e "${YELLOW}Running health checks...${NC}"
ALB_DNS=$(aws elbv2 describe-load-balancers \
    --names "${ECS_SERVICE}-alb" \
    --query 'LoadBalancers[0].DNSName' \
    --output text \
    --region "${AWS_REGION}")

HEALTH_CHECK_URL="http://${ALB_DNS}/health"

for i in {1..30}; do
    if curl -s -f "${HEALTH_CHECK_URL}" > /dev/null; then
        echo -e "${GREEN}Health check passed!${NC}"
        break
    fi
    echo -e "${YELLOW}Waiting for service to be healthy... (${i}/30)${NC}"
    sleep 10
done

# Check if health check failed
if ! curl -s -f "${HEALTH_CHECK_URL}" > /dev/null; then
    echo -e "${RED}Health check failed! Rolling back...${NC}"
    # Rollback logic here
    exit 1
fi

# Clear cache (if Redis is used)
echo -e "${YELLOW}Clearing cache...${NC}"
# Add Redis cache clearing logic here

echo -e "${GREEN}========================================="
echo "Deployment completed successfully!"
echo "Environment: ${ENVIRONMENT}"
echo "Image: ${ECR_IMAGE}"
echo -e "=========================================${NC}"

# Send deployment notification
if [ -n "${SLACK_WEBHOOK_URL}" ]; then
    curl -X POST "${SLACK_WEBHOOK_URL}" \
        -H 'Content-type: application/json' \
        --data "{
            \"text\": \"✅ Deployment Successful\n*Service:* ${APP_NAME}\n*Environment:* ${ENVIRONMENT}\n*Image:* ${ECR_IMAGE}\n*Time:* $(date)\"
        }"
fi
