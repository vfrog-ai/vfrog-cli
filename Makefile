.PHONY: build build-dev build-staging build-prod watch install clean

# Build flags
CGO_ENABLED := 0
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Local environment URLs (for local development with docker-compose)
LOCAL_SUPABASE_URL ?= 
LOCAL_SUPABASE_KEY ?= 
LOCAL_API_URL := http://localhost:8005
LOCAL_PLATFORM_HOST := http://localhost:3000

# Dev environment URLs
DEV_SUPABASE_URL ?= 
DEV_SUPABASE_KEY ?= 
DEV_API_URL := https://api-dev.vfrog.ai
DEV_PLATFORM_HOST := https://platform-dev.vfrog.ai

# Staging environment URLs
STAGING_SUPABASE_URL ?= 
STAGING_SUPABASE_KEY ?= 
STAGING_API_URL := https://api-staging.vfrog.ai
STAGING_PLATFORM_HOST := https://platform-staging.vfrog.ai

# Production environment URLs
PROD_SUPABASE_URL ?= 
PROD_SUPABASE_KEY ?= 
PROD_API_URL := https://api.vfrog.ai
PROD_PLATFORM_HOST := https://platform.vfrog.ai

# Base ldflags
LDFLAGS := -s -w -X 'github.com/vfrog-ai/vfrog-cli/internal/config.Version=$(VERSION)'

# Default build (no credentials baked in)
build:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS)" -o vfrog ./main.go

# Build dev binary
build-dev:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS) \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.Environment=dev' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabaseURL=$(DEV_SUPABASE_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabasePublishableKey=$(DEV_SUPABASE_KEY)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIURL=$(DEV_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIProjectBaseURL=$(DEV_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultPlatformHost=$(DEV_PLATFORM_HOST)'" \
		-o vfrog-dev ./main.go
	@echo "Built vfrog-dev ($(VERSION))"

# Build staging binary
build-staging:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS) \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.Environment=staging' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabaseURL=$(STAGING_SUPABASE_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabasePublishableKey=$(STAGING_SUPABASE_KEY)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIURL=$(STAGING_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIProjectBaseURL=$(STAGING_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultPlatformHost=$(STAGING_PLATFORM_HOST)'" \
		-o vfrog-staging ./main.go
	@echo "Built vfrog-staging ($(VERSION))"

# Build production binary
build-prod:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS) \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.Environment=production' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabaseURL=$(PROD_SUPABASE_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabasePublishableKey=$(PROD_SUPABASE_KEY)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIURL=$(PROD_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIProjectBaseURL=$(PROD_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultPlatformHost=$(PROD_PLATFORM_HOST)'" \
		-o vfrog ./main.go
	@echo "Built vfrog (production) ($(VERSION))"

# Watch for changes and rebuild (requires watchexec: brew install watchexec)
watch:
	watchexec -e go -r -- make build-dev

# Watch with entr (alternative: brew install entr)
watch-entr:
	find . -name '*.go' | entr -r make build-dev

# Install to /usr/local/bin
install: build-dev
	sudo cp vfrog-dev /usr/local/bin/vfrog-dev
	@echo "Installed vfrog-dev to /usr/local/bin/"

# Clean build artifacts
clean:
	rm -f vfrog vfrog-dev vfrog-staging vfrog-local

# Fetch dev secrets from GCP and build
build-dev-gcp:
	@echo "Fetching dev secrets from GCP..."
	$(eval DEV_SUPABASE_URL := $(shell gcloud secrets versions access latest --secret='dev-supabase-url' --project='development-481611-t8'))
	$(eval DEV_SUPABASE_KEY := $(shell gcloud secrets versions access latest --secret='dev-supabase-publishable-key' --project='development-481611-t8'))
	@make build-dev DEV_SUPABASE_URL="$(DEV_SUPABASE_URL)" DEV_SUPABASE_KEY="$(DEV_SUPABASE_KEY)"


# Build local binary (points to localhost services)
build-local:
	CGO_ENABLED=$(CGO_ENABLED) go build -ldflags "$(LDFLAGS) \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.Environment=local' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabaseURL=$(LOCAL_SUPABASE_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultSupabasePublishableKey=$(LOCAL_SUPABASE_KEY)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIURL=$(LOCAL_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultAPIProjectBaseURL=$(LOCAL_API_URL)' \
		-X 'github.com/vfrog-ai/vfrog-cli/internal/config.DefaultPlatformHost=$(LOCAL_PLATFORM_HOST)'" \
		-o vfrog-local ./main.go
	@echo "Built vfrog-local ($(VERSION))"
	@echo "Configured to use:"
	@echo "  - API URL: $(LOCAL_API_URL)"

# Fetch dev secrets from GCP and build
build-local-gcp:
	@echo "Fetching local secrets from GCP..."
	$(eval LOCAL_SUPABASE_URL := $(shell gcloud secrets versions access latest --secret='dev-supabase-url' --project='development-481611-t8'))
	$(eval LOCAL_SUPABASE_KEY := $(shell gcloud secrets versions access latest --secret='dev-supabase-publishable-key' --project='development-481611-t8'))
	@make build-local LOCAL_SUPABASE_URL="$(LOCAL_SUPABASE_URL)" LOCAL_SUPABASE_KEY="$(LOCAL_SUPABASE_KEY)"
