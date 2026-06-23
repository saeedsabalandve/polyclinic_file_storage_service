.PHONY: help build run test clean docker-build docker-run migrate-up migrate-down

# Variables
APP_NAME = polyclinic-file-storage
VERSION = 1.0.0
BUILD_DIR = ./bin
MAIN_FILE = cmd/server/main.go

# Colors for output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
RESET  := $(shell tput -Txterm sgr0)

help: ## Show this help message
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  ${YELLOW}%-15s${GREEN}%s${RESET}\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "${GREEN}Building $(APP_NAME)...${RESET}"
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo "${GREEN}Build complete!${RESET}"

run: ## Run the application locally
	@echo "${GREEN}Running $(APP_NAME)...${RESET}"
	@go run $(MAIN_FILE)

test: ## Run tests
	@echo "${GREEN}Running tests...${RESET}"
	@go test -v -race -coverprofile=coverage.out ./...

test-integration: ## Run integration tests
	@echo "${GREEN}Running integration tests...${RESET}"
	@go test -v -tags=integration ./tests/integration/...

test-e2e: ## Run end-to-end tests
	@echo "${GREEN}Running end-to-end tests...${RESET}"
	@go test -v -tags=e2e ./tests/e2e/...

coverage: test ## Generate test coverage report
	@echo "${GREEN}Generating coverage report...${RESET}"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "${GREEN}Coverage report generated: coverage.html${RESET}"

clean: ## Clean build artifacts
	@echo "${GREEN}Cleaning...${RESET}"
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "${GREEN}Clean complete!${RESET}"

docker-build: ## Build Docker image
	@echo "${GREEN}Building Docker image...${RESET}"
	@docker build -t $(APP_NAME):$(VERSION) .
	@docker tag $(APP_NAME):$(VERSION) $(APP_NAME):latest

docker-run: ## Run Docker container
	@echo "${GREEN}Running Docker container...${RESET}"
	@docker-compose up -d

docker-stop: ## Stop Docker containers
	@echo "${GREEN}Stopping Docker containers...${RESET}"
	@docker-compose down

migrate-up: ## Run database migrations
	@echo "${GREEN}Running migrations up...${RESET}"
	@migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Rollback database migrations
	@echo "${GREEN}Rolling back migrations...${RESET}"
	@migrate -path migrations -database "$(DATABASE_URL)" down

create-tenant: ## Create a new tenant
	@echo "${GREEN}Creating tenant...${RESET}"
	@curl -X POST http://localhost:8080/api/v1/admin/tenants \
		-H "Content-Type: application/json" \
		-H "Authorization: Bearer $(TOKEN)" \
		-d '{"id":"$(TENANT_ID)","name":"$(TENANT_NAME)"}'

lint: ## Run linter
	@echo "${GREEN}Running linter...${RESET}"
	@golangci-lint run ./...

fmt: ## Format code
	@echo "${GREEN}Formatting code...${RESET}"
	@go fmt ./...

vet: ## Run go vet
	@echo "${GREEN}Running go vet...${RESET}"
	@go vet ./...

deps: ## Install dependencies
	@echo "${GREEN}Installing dependencies...${RESET}"
	@go mod download
	@go mod tidy

init: deps ## Initialize project
	@echo "${GREEN}Initializing project...${RESET}"
	@cp .env.example .env
	@echo "${GREEN}Project initialized! Edit .env with your settings.${RESET}"

dev: ## Start development environment
	@echo "${GREEN}Starting development environment...${RESET}"
	@docker-compose -f docker-compose.dev.yml up -d
	@air
