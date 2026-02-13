# vfrog CLI

Command-line interface for the vfrog platform. Provides fast, reliable, scriptable access to vfrog resources without using the web UI.

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Step-by-Step Guide](#step-by-step-guide)
  - [Step 1: Authenticate](#step-1-authenticate)
  - [Step 2: Set Up Your Organisation](#step-2-set-up-your-organisation)
  - [Step 3: Create a Project](#step-3-create-a-project)
  - [Step 4: Upload Dataset Images](#step-4-upload-dataset-images)
  - [Step 5: Create Objects (Product Images)](#step-5-create-objects-product-images)
  - [Step 6: Run Iterations (SSAT Workflow)](#step-6-run-iterations-ssat-workflow)
  - [Step 7: Review Annotations](#step-7-review-annotations)
  - [Step 8: Train a Model](#step-8-train-a-model)
  - [Step 9: Run CV Inference](#step-9-run-cv-inference)
  - [Step 10: Export Your Data](#step-10-export-your-data)
- [Command Reference](#command-reference)
- [Configuration](#configuration)
- [CI/CD Usage](#cicd-usage)
- [Development](#development)

---

## Installation

### Download Binary (Recommended)

Download the appropriate binary for your platform and environment:

**Production (recommended for most users):**

```bash
# macOS (Apple Silicon)
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-darwin-arm64 -o vfrog
chmod +x vfrog && sudo mv vfrog /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-darwin-amd64 -o vfrog
chmod +x vfrog && sudo mv vfrog /usr/local/bin/

# Linux (AMD64)
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-linux-amd64 -o vfrog
chmod +x vfrog && sudo mv vfrog /usr/local/bin/
```

**Staging environment:**

```bash
# macOS (Apple Silicon)
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-staging-darwin-arm64 -o vfrog-staging
chmod +x vfrog-staging && sudo mv vfrog-staging /usr/local/bin/
```

**Development environment:**

```bash
# macOS (Apple Silicon)
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-dev-darwin-arm64 -o vfrog-dev
chmod +x vfrog-dev && sudo mv vfrog-dev /usr/local/bin/
```

### Verify Installation

```bash
vfrog version
vfrog --help
```

### Available Binaries

Each release includes binaries configured for different environments:

| Binary          | Environment | API URL                        |
| --------------- | ----------- | ------------------------------ |
| `vfrog`         | production  | `https://api.vfrog.ai`         |
| `vfrog-staging` | staging     | `https://api-staging.vfrog.ai` |
| `vfrog-dev`     | development | `https://api-dev.vfrog.ai`     |

Credentials are baked into each binary at build time.

---

## Quick Start

```bash
# 1. Log in
vfrog login

# 2. Set organisation and project
vfrog config set organisation --organisation_id <org_id>
vfrog config set project --project_id <project_id>

# 3. Upload images and run the full SSAT workflow
vfrog dataset_images upload https://example.com/img1.jpg https://example.com/img2.jpg
vfrog objects create https://example.com/product.jpg --label "My Product"
vfrog config set object --object_id <object_id>
vfrog iterations create <object_id>
vfrog iterations ssat --iteration_number 1
vfrog iterations status --iteration_number 1 --watch

# 4. Train and deploy
vfrog iterations train --iteration_number 1
vfrog iterations status --iteration_number 1 --watch
vfrog iterations deploy --iteration_number 1

# 5. Export results
vfrog export yolo --iteration_id <id> --output ./my-dataset
```

---

## Step-by-Step Guide

### Step 1: Authenticate

Log in with your vfrog platform credentials:

```bash
# Interactive login (prompts for email and password)
vfrog login

# Non-interactive login (for scripts and CI/CD)
vfrog login --email user@example.com --password mypassword
```

Your authentication tokens are stored locally in `~/.vfrog/config-<environment>.json` and automatically refresh when expired.

### Step 2: Set Up Your Organisation

List available organisations and select one as your default:

```bash
# List organisations you belong to
vfrog organisations list

# Set default organisation (required before using most commands)
vfrog config set organisation --organisation_id <org_id>
```

> All subsequent commands use this organisation. Changing the organisation automatically clears the active project.

### Step 3: Create a Project

Projects are containers for your dataset images, objects, and iterations:

```bash
# List existing projects
vfrog projects list

# Create a new project
vfrog projects create "My Detection Project"

# Set default project
vfrog config set project --project_id <project_id>

# Delete a project (prompts for confirmation)
vfrog projects delete --project_id <project_id>

# Delete without confirmation
vfrog projects delete --project_id <project_id> --force
```

### Step 4: Upload Dataset Images

Dataset images are the images your model will learn to search through. There are several ways to add them:

#### From URLs

```bash
# Upload one or more image URLs
vfrog dataset_images upload https://example.com/image1.jpg https://example.com/image2.jpg
```

> Images are downloaded from the provided URLs and re-uploaded to vfrog's CDN, so the original URLs do not need to remain accessible after upload.

#### From Local Files

```bash
# Upload a single local file
vfrog dataset_images upload --file ./photo.jpg

# Upload all images in a directory
vfrog dataset_images upload --dir ./my-images/
```

Local files are uploaded to S3 via signed URL and the resulting URL is stored.

#### From CSV

For bulk imports, use a CSV file:

```bash
vfrog dataset_images import --csv ./images.csv
```

CSV format (header row required, only `image_url` is mandatory):
```csv
image_url,external_id,label
https://example.com/img1.jpg,EXT001,scene_1
https://example.com/img2.jpg,EXT002,scene_2
```

#### List and Delete

```bash
# List all dataset images in the active project
vfrog dataset_images list

# Delete by ID
vfrog dataset_images delete --dataset_image_id <id>
```

### Step 5: Create Objects (Product Images)

Objects are the product/reference images your model learns to detect:

```bash
# Create from URL
vfrog objects create https://example.com/product.jpg --label "Sneaker" --external_id "SKU123"

# Create from local file
vfrog objects create --file ./product.jpg --label "Sneaker" --external_id "123"

# List objects in the active project
vfrog objects list

# Set default object (needed for iteration commands)
vfrog config set object --object_id <object_id>

# Delete
vfrog objects delete --object_id <id>
```

### Step 6: Run Iterations (SSAT Workflow)

Iterations are the core SSAT (Semi-Supervised Active Training) workflow. Each iteration annotates your dataset images, you review them, then train a model.

#### Create an Iteration

```bash
# Create iteration for an object (randomly selects 20 dataset images by default)
vfrog iterations create <object_id>

# Create with a specific number of random images
vfrog iterations create <object_id> --random 50
```

#### List Iterations

```bash
# List iterations for the active object
vfrog iterations list

# List for a specific object
vfrog iterations list --object_id <id>
```

#### Start SSAT Annotation

```bash
# Start SSAT by iteration ID
vfrog iterations ssat --iteration_id <id>

# Start SSAT by iteration number (uses active object)
vfrog iterations ssat --iteration_number 1

# Start SSAT with random image selection (overrides linked images)
vfrog iterations ssat --iteration_id <id> --random 100

# Restart an iteration and immediately run SSAT
vfrog iterations ssat --iteration_id <id> --restart

# Specify industry directly (skips automatic control check)
vfrog iterations ssat --iteration_id <id> --industry Retail
```

**How it works:**
- **Iteration 1:** Runs a control check to detect the project's industry via LLM, then uses the annotator pipeline (cutout extraction + matching). You can skip the control check by providing `--industry` directly.
- **Iteration 2+:** Uses inference with the trained model from the previous iteration

All linked dataset images are processed by default. Use `--random N` to sample from the full project dataset instead.

**Supported industries:** Retail, Agriculture, Aquaculture, Fisheries, Manufacturing, Mechanical Engineering, PPE

#### Monitor Progress

```bash
# Check status once
vfrog iterations status --iteration_id <id>

# Watch status until completion (polls every 5 seconds)
vfrog iterations status --iteration_id <id> --watch

# Watch with custom interval
vfrog iterations status --iteration_id <id> --watch --interval 10
```

#### SSAT Control

Submit SSAT control feedback for quality assessment:

```bash
vfrog iterations control --iteration_id <id>
```

#### Review in HALO

Open the HALO (Human Assisted Labelling of Objects) web UI to review and correct annotations:

```bash
# Print the HALO URL
vfrog iterations halo --iteration_id <id>
```



### Step 7: Review Annotations

View the annotations produced by SSAT:

```bash
# List annotations with bounding box counts (table view)
vfrog iterations annotations --iteration_id <id>

# Full annotation details (JSON view)
vfrog iterations annotations --iteration_id <id> --json
```

Table output shows:
| Column | Description |
|--------|-------------|
| DATASET_IMAGE_ID | The dataset image that was annotated |
| ANNOTATIONS | Number of bounding boxes detected |
| CREATED_AT | When the annotation was created |

Use `--json` for the full annotation data including bounding box coordinates.

### Step 8: Train a Model

After reviewing annotations in HALO, train a YOLO model:

```bash
# Train by iteration ID
vfrog iterations train --iteration_id <id>

# Train by iteration number
vfrog iterations train --iteration_number 1

# Monitor training progress
vfrog iterations status --iteration_id <id> --watch
```

Once training completes, you can:
- Deploy the model to production: `vfrog iterations deploy --iteration_id <id>`
- Create the next iteration: `vfrog iterations next --iteration_id <id>`
- Run inference with the trained model
- Export the annotated dataset

### Step 8b: Deploy to Production

After training completes, deploy the model to make it available for API inference:

```bash
# Deploy by iteration ID
vfrog iterations deploy --iteration_id <id>

# Deploy by iteration number
vfrog iterations deploy --iteration_number 1
```

**What it does:**
1. Verifies the iteration has `trained_status: completed` and a valid model
2. Creates a **class** record linking the model to your project
3. Creates a **model-class mapping** for API routing
4. Updates the iteration's `trained_status` to `validated`

After deployment, the model is available for inference via your API key.
#### Manage Iterations

```bash
# Create the next iteration from the current one
vfrog iterations next --iteration_id <id>

# Restart an iteration (delete and recreate)
vfrog iterations restart --iteration_id <id>

# Delete an iteration
vfrog iterations delete --iteration_id <id>
```

> You can use `--iteration_number` and `--object_id` instead of `--iteration_id` for any iteration command.

### Step 9: Run CV Inference

Run computer vision inference using your API key:

#### Single Image

```bash
# Inference on a URL
vfrog inference --api-key <key> --image_url https://example.com/image.jpg

# Inference on a local file
vfrog inference --api-key <key> --image ./local.jpg
```

#### Batch Inference

Process up to 10 images at once:

```bash
vfrog inference batch --api-key <key> --image_url "url1,url2,url3"

# With an external ID for tracking
vfrog inference batch --api-key <key> --image_url "url1,url2" --external_id "batch-001"
```

#### Check Request Status

```bash
vfrog inference status <request_id> --api-key <key>
```

#### Submit Feedback

Rate inference results to help improve the model:

```bash
# Positive feedback
vfrog inference feedback --request_id <id> --rating 1 --api-key <key>

# Negative feedback
vfrog inference feedback --request_id <id> --rating -1 --api-key <key>

# Neutral
vfrog inference feedback --request_id <id> --rating 0 --api-key <key>
```

### Step 10: Export Your Data

Export annotated datasets for external use:

#### YOLO Format

Exports images, label files, and a `data.yaml` configuration:

```bash
# Export to a directory
vfrog export yolo --iteration_id <id> --output ./my-dataset

# Export and create a ZIP archive
vfrog export yolo --iteration_id <id> --output ./my-dataset --zip
```

Output structure:
```
my-dataset/
  images/        # Downloaded dataset images
  labels/        # YOLO format annotations (class_id center_x center_y width height)
  data.yaml      # Class names, train/val split (90/10)
```

#### JSON Format

Export all annotation data as a structured JSON file:

```bash
vfrog export json --iteration_id <id> --output ./annotations.json
```

The JSON file contains full annotation arrays with bounding box coordinates, dataset image metadata, and export timestamps.

---

## Command Reference

### Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output in JSON format (all commands) |
| `--config` | Custom config file path |
| `-h, --help` | Help for any command |

### All Commands

| Command | Description |
|---------|-------------|
| `vfrog login` | Authenticate with the platform |
| `vfrog version` | Print CLI version and environment |
| `vfrog config set organisation` | Set default organisation |
| `vfrog config set project` | Set default project |
| `vfrog config set object` | Set default object |
| **Organisations** | |
| `vfrog organisations list` | List organisations |
| **Projects** | |
| `vfrog projects list` | List projects |
| `vfrog projects create <name>` | Create a project |
| `vfrog projects delete` | Delete a project |
| **Dataset Images** | |
| `vfrog dataset_images upload` | Upload from URLs or local files |
| `vfrog dataset_images import` | Import from CSV file |
| `vfrog dataset_images list` | List dataset images |
| `vfrog dataset_images delete` | Delete a dataset image |
| **Objects** | |
| `vfrog objects create` | Create from URL or local file |
| `vfrog objects list` | List objects |
| `vfrog objects delete` | Delete an object |
| **Iterations** | |
| `vfrog iterations list` | List iterations |
| `vfrog iterations create` | Create a new iteration |
| `vfrog iterations delete` | Delete an iteration |
| `vfrog iterations ssat` | Start SSAT annotation |
| `vfrog iterations status` | Check iteration status |
| `vfrog iterations annotations` | View iteration annotations |
| `vfrog iterations control` | Submit SSAT control feedback |
| `vfrog iterations train` | Train a model |
| `vfrog iterations deploy` | Deploy trained model to production |
| `vfrog iterations halo` | Get HALO review URL |
| `vfrog iterations next` | Create next iteration |
| `vfrog iterations restart` | Restart an iteration |
| **Inference** | |
| `vfrog inference` | Run single image inference |
| `vfrog inference batch` | Run batch inference (up to 10) |
| `vfrog inference status` | Check inference request status |
| `vfrog inference feedback` | Submit inference feedback |
| **Export** | |
| `vfrog export yolo` | Export in YOLO format |
| `vfrog export json` | Export in JSON format |

Use `vfrog <command> --help` for detailed flag information on any command.

---

## Configuration

### API Key Precedence

For inference commands, API keys are resolved in this order:

1. `--api-key` flag
2. `VFROG_API_KEY` environment variable
3. `api_key` in config file

### Configuration Files

Each environment binary uses a separate config file:

| Binary | Config File |
|--------|-------------|
| `vfrog` (production) | `~/.vfrog/config-production.json` |
| `vfrog-staging` | `~/.vfrog/config-staging.json` |
| `vfrog-dev` | `~/.vfrog/config-dev.json` |
| `vfrog-local` | `~/.vfrog/config-local.json` |

On Windows: `%USERPROFILE%\.vfrog\` (e.g., `C:\Users\YourName\.vfrog\`).

You can have different organisations, projects, and authentication tokens per environment without conflicts.

### Context Requirements

| Commands | Requires |
|----------|----------|
| `projects` | `organisation_id` |
| `dataset_images`, `objects`, `iterations`, `export` | `organisation_id` + `project_id` |
| `iterations list`, iteration number lookups | `object_id` |

When `organisation_id` changes, `project_id` is automatically cleared. When `project_id` changes, `object_id` is automatically cleared.

---

## Shell Autocompletion

The CLI supports autocompletion for bash, zsh, fish, and PowerShell:

```bash
# Zsh (add to ~/.zshrc)
source <(vfrog completion zsh)

# Bash (add to ~/.bashrc)
source <(vfrog completion bash)

# Fish
vfrog completion fish > ~/.config/fish/completions/vfrog.fish

# PowerShell
vfrog completion powershell | Out-String | Invoke-Expression
```

After setup, test with:
```bash
vfrog <TAB>              # Shows all commands
vfrog iterations <TAB>   # Shows subcommands
vfrog --<TAB>            # Shows global flags
```

---

## CI/CD Usage

```bash
# Download the binary
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-linux-amd64 -o vfrog
chmod +x vfrog

# Authenticate
vfrog login --email ci@example.com --password $CI_PASSWORD

# Set context
vfrog config set organisation --organisation_id $ORG_ID
vfrog config set project --project_id $PROJECT_ID

# Upload images from a directory
vfrog dataset_images upload --dir ./training-images/

# Or import from CSV
vfrog dataset_images import --csv ./image-manifest.csv

# Create and run an iteration
vfrog objects create https://cdn.example.com/product.jpg --label "Widget"
vfrog config set object --object_id $(vfrog objects list --json | jq -r '.[0].id')
vfrog iterations create $(vfrog objects list --json | jq -r '.[0].id') --random 100

# Run SSAT and wait for completion
vfrog iterations ssat --iteration_number 1
vfrog iterations status --iteration_number 1 --watch

# Train and wait
vfrog iterations train --iteration_number 1
vfrog iterations status --iteration_number 1 --watch

# Deploy to production
vfrog iterations deploy --iteration_number 1

# Export results
vfrog export yolo --iteration_id $(vfrog iterations list --json | jq -r '.[0].id') --output ./dataset --zip
```

All commands support `--json` output for scripting:

```bash
# Get project ID from JSON output
PROJECT_ID=$(vfrog projects list --json | jq -r '.[0].id')

# Get iteration status
STATUS=$(vfrog iterations status --iteration_id $ITER_ID --json | jq -r '.status')
```

---

## Development

### Prerequisites

Install Go 1.22 or later:

```bash
# macOS (Homebrew)
brew install go

# Linux (Ubuntu/Debian)
sudo apt update && sudo apt install -y golang-go
```

**Windows:** Download and run the MSI installer from [go.dev/dl](https://go.dev/dl/). The installer adds Go to your PATH automatically.

Verify your installation:
```bash
go version   # should show go1.22 or later
```

### Building from Source

```bash
git clone https://github.com/vfrog/vfrog-cli.git
cd vfrog-cli
go build -o vfrog ./main.go
```

On Windows (PowerShell):
```powershell
git clone https://github.com/vfrog/vfrog-cli.git
cd vfrog-cli
go build -o vfrog.exe ./main.go
```

### Building with Make

The Makefile provides targets for each environment. Credentials are baked into the binary at build time.

```bash
# Dev — fetches credentials from GCP Secret Manager
make build-dev-gcp

# Staging — pass credentials manually
make build-staging STAGING_SUPABASE_URL="..." STAGING_SUPABASE_KEY="..."

# Production — pass credentials manually
make build-prod PROD_SUPABASE_URL="..." PROD_SUPABASE_KEY="..."

# Local development — connects to localhost services, fetches credentials from GCP
make build-local-gcp
```

You can also build without credentials (uses defaults):
```bash
make build          # plain build, no environment credentials
make build-dev      # dev binary with env vars (set DEV_SUPABASE_URL, DEV_SUPABASE_KEY)
```

Other useful targets:
```bash
make watch          # rebuild on file changes (requires watchexec)
make install        # build dev binary and install to /usr/local/bin
make clean          # remove build artifacts
```

### Building with Custom Credentials

```bash
LDFLAGS="-X 'github.com/vfrog/vfrog-cli/internal/config.DefaultSupabaseURL=https://your.supabase.co'"
LDFLAGS="${LDFLAGS} -X 'github.com/vfrog/vfrog-cli/internal/config.DefaultSupabasePublishableKey=your-key'"
go build -ldflags "${LDFLAGS}" -o vfrog ./main.go
```

### Testing

```bash
go test ./...
```

### Project Structure

```
vfrog-cli/
├── cmd/                   # Cobra commands
│   ├── root.go            # Root command, global flags
│   ├── version.go         # Version command
│   ├── login.go           # Authentication
│   ├── config.go          # Config management
│   ├── organisations.go   # Organisation commands
│   ├── projects.go        # Project CRUD (list, create, delete)
│   ├── dataset_images.go  # Dataset image management (upload, import, list, delete)
│   ├── objects.go         # Object/product image management
│   ├── iterations.go      # SSAT workflow (ssat, train, deploy, status, annotations, control, halo)
│   ├── inference.go       # CV inference (single, batch, status, feedback)
│   └── export.go          # Data export (YOLO, JSON)
├── internal/
│   ├── config/            # Configuration with build-time defaults
│   ├── auth/              # Supabase authentication + token refresh
│   ├── api/
│   │   ├── supabase/      # Supabase PostgREST client (CRUD)
│   │   ├── vfrogapi/      # vfrog API client (inference, SSAT, training)
│   │   └── storage/       # S3 file upload via signed URLs
│   └── output/            # Table and JSON output formatting
├── Makefile               # Build targets for all environments
├── .github/workflows/
│   ├── ci.yml             # Continuous integration checks
│   └── release.yml        # CI/CD for building and releasing binaries
└── main.go
```

---

## License

MIT
