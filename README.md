# vfrog CLI

Command-line interface for the vfrog platform. Provides fast, reliable, scriptable access to vfrog resources without using the web UI.

## Installation

### Download Binary (Recommended)

Download the appropriate binary for your environment:

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

**Development environment:**

```bash
# macOS (Apple Silicon)
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-dev-darwin-arm64 -o vfrog-dev
chmod +x vfrog-dev && sudo mv vfrog-dev /usr/local/bin/
```

**Staging environment:**

```bash
# macOS (Apple Silicon)
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-staging-darwin-arm64 -o vfrog-staging
chmod +x vfrog-staging && sudo mv vfrog-staging /usr/local/bin/
```

### Verify Installation

```bash
vfrog version
vfrog --help
```

### Shell Autocompletion

Enable autocompletion for your shell:

**Zsh (macOS default):**

```bash
# Add to your ~/.zshrc
echo 'source <(vfrog completion zsh)' >> ~/.zshrc

# Or generate a completion file (recommended for faster shell startup)
vfrog completion zsh > "${fpath[1]}/_vfrog"

# Reload shell
source ~/.zshrc
```

**Bash:**

```bash
# Linux: Add to ~/.bashrc
echo 'source <(vfrog completion bash)' >> ~/.bashrc

# macOS: Install bash-completion first
brew install bash-completion@2
echo 'source <(vfrog completion bash)' >> ~/.bash_profile

# Reload shell
source ~/.bashrc  # or ~/.bash_profile on macOS
```

**Fish:**

```bash
vfrog completion fish > ~/.config/fish/completions/vfrog.fish
```

**PowerShell:**

```powershell
vfrog completion powershell | Out-String | Invoke-Expression
# Or add to your PowerShell profile for persistence
```

## Available Binaries

Each release includes three binaries configured for different environments:

| Binary          | Environment | Supabase            | API URL                        |
| --------------- | ----------- | ------------------- | ------------------------------ |
| `vfrog`         | production  | Production Supabase | `https://api.vfrog.ai`         |
| `vfrog-staging` | staging     | Staging Supabase    | `https://api-staging.vfrog.ai` |
| `vfrog-dev`     | development | Dev Supabase        | `https://api-dev.vfrog.ai`     |

Credentials are baked into each binary at build time from GCP Secret Manager.

## Authentication

```bash
vfrog login
```

This will prompt for your email and password and store authentication tokens locally in `~/.vfrog/config.json`.

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

For CI/CD automation:

```bash
# Download the correct binary for your environment
curl -L https://github.com/vfrog/vfrog-cli/releases/latest/download/vfrog-linux-amd64 -o vfrog
chmod +x vfrog

# Login
vfrog login --email ci@example.com --password $CI_PASSWORD

# Configure context
vfrog config set organisation --organisation_id $ORG_ID
vfrog config set project --project_id $PROJECT_ID

# Use the CLI
vfrog dataset_images upload https://example.com/image.jpg
```

## Development

### Building from Source

```bash
git clone https://github.com/vfrog/vfrog-cli.git
cd vfrog-cli
go build -o vfrog ./main.go
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
├── cmd/              # Cobra commands
│   ├── root.go
│   ├── version.go
│   ├── login.go
│   ├── config.go
│   ├── organisations.go
│   ├── projects.go
│   ├── dataset_images.go
│   ├── objects.go
│   ├── iterations.go
│   └── inference.go
├── internal/
│   ├── config/       # Configuration management (with build-time defaults)
│   ├── auth/         # Supabase authentication
│   ├── api/
│   │   ├── supabase/ # Supabase PostgREST client
│   │   ├── vfrogapi/ # vfrog API client
│   │   └── inference/ # Inference server client
│   └── output/       # Output formatting
├── .github/workflows/
│   └── release.yml   # CI/CD for building and releasing binaries
└── main.go
```

## Limitations (v0.1)

- Local file uploads for `dataset_images upload` and `objects create` are not supported (URLs only)
- Device-code login flow not implemented

## License

MIT
