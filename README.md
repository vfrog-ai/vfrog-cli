# vfrog CLI

Command-line interface for the vfrog platform. Provides fast, reliable, scriptable access to vfrog resources without using the web UI.

## Installation

### From Source

```bash
git clone https://github.com/vfrog/vfrog-cli.git
cd vfrog-cli
go build -o vfrog ./main.go
sudo mv vfrog /usr/local/bin/
```

### Binary Releases

Binary releases will be available for macOS, Linux, and Windows (coming soon).

## Configuration

The CLI stores configuration in `~/.vfrog/config.json`. You can configure:

- `supabase_url`: Your Supabase project URL
- `supabase_publishable_key`: Your Supabase publishable key
- `organisation_id`: Default organisation ID
- `project_id`: Default project ID
- `api_url`: vfrog API URL (default: https://api-dev.vfrog.ai)
- `inference_url`: Inference server URL (default: https://inf-dev-01.vfrog.ai)
- `inference_api_key`: Inference API key
- `api_key`: General API key for inference requests
- `platform_host`: Platform host URL (for generating SSAT/HALO links)

## Authentication

### Interactive Login

```bash
vfrog login
```

This will prompt for your email and password and store authentication tokens locally.

### Using Flags

```bash
vfrog login --email user@example.com --password mypassword
```

## Usage Examples

### Organisations

```bash
# List organisations
vfrog organisations list

# Set default organisation
vfrog config set organisation --organisation_id <org_id>
```

### Projects

```bash
# List projects
vfrog projects list

# Create a project
vfrog projects create "My Project"

# Set default project
vfrog config set project --project_id <project_id>
```

### Dataset Images

```bash
# Upload dataset images from URLs
vfrog dataset_images upload https://example.com/image1.jpg https://example.com/image2.jpg

# List dataset images
vfrog dataset_images list

# Delete a dataset image
vfrog dataset_images delete --dataset_image_id <id>
```

### Objects (Product Images)

```bash
# Create an object from URL
vfrog objects create https://example.com/product.jpg --label "Product Name" --external_id "EXT123"

# List objects
vfrog objects list

# Delete an object
vfrog objects delete --object_id <id>
```

### Iterations

```bash
# List iterations for an object
vfrog iterations list --object_id <object_id>

# Create a new iteration (randomly selects 20 dataset images by default)
vfrog iterations create <object_id>

# Create with custom number of random images
vfrog iterations create <object_id> --random 50

# Delete an iteration
vfrog iterations delete --iteration_id <id>

# Get SSAT URL for an iteration
vfrog iterations ssat --iteration_id <id>

# Get HALO URL for an iteration
vfrog iterations halo --iteration_id <id>

# Train a model for an iteration
vfrog iteration train --iteration_id <id>
```

### Inference

```bash
# Run inference on an image URL
vfrog inference --api-key <key> --image_url https://example.com/image.jpg

# Run inference on a local image file
vfrog inference --api-key <key> --image ./local.jpg
```

## API Key Precedence

For inference commands, API keys are resolved in this order:

1. `--api-key` flag
2. `VFROG_API_KEY` environment variable
3. `api_key` in `~/.vfrog/config.json`

For training commands, inference API keys are resolved as:

1. `--api-key` flag (if supported)
2. `VFROG_INFERENCE_API_KEY` environment variable
3. `inference_api_key` in `~/.vfrog/config.json`

## JSON Output

All commands support `--json` flag for machine-readable output:

```bash
vfrog projects list --json
```

## Context Requirements

- `organisation_id` must be set to use `projects` commands
- `project_id` must be set to use `dataset_images`, `objects`, and `iterations` commands
- When `organisation_id` changes, `project_id` is automatically cleared

## CI/CD Usage

For CI/CD automation, you can use environment variables:

```bash
export VFROG_API_KEY=your_api_key
export VFROG_INFERENCE_API_KEY=your_inference_api_key
vfrog inference --image_url https://example.com/image.jpg
```

Or use stored credentials after logging in:

```bash
vfrog login --email ci@example.com --password $CI_PASSWORD
vfrog config set organisation --organisation_id $ORG_ID
vfrog config set project --project_id $PROJECT_ID
vfrog dataset_images upload https://example.com/image.jpg
```

## Development

### Building

```bash
go build -o vfrog ./main.go
```

### Testing

```bash
go test ./...
```

### Project Structure

```
vfrog-cli/
├── cmd/              # Cobra commands
│   ├── root.go
│   ├── login.go
│   ├── config.go
│   ├── organisations.go
│   ├── projects.go
│   ├── dataset_images.go
│   ├── objects.go
│   ├── iterations.go
│   └── inference.go
├── internal/
│   ├── config/       # Configuration management
│   ├── auth/         # Authentication
│   ├── api/
│   │   ├── supabase/ # Supabase PostgREST client
│   │   ├── vfrogapi/ # vfrog API client
│   │   └── inference/ # Inference server client
│   └── output/       # Output formatting
└── main.go
```

## Limitations (v0.1)

- Local file uploads for `dataset_images upload` and `objects create` are not supported (URLs only)
- Device-code login flow not implemented
- Autocompletion not available
- Rich progress UI not implemented

## License

[Your License Here]

