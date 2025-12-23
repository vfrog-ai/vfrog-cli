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

## Shell Autocompletion

The vfrog CLI supports autocompletion for bash, zsh, fish, and PowerShell. This enables tab completion for commands, subcommands, and flags.

### Quick Setup

**Zsh (macOS default, Oh My Zsh):**

```bash
# Option 1: Add to your ~/.zshrc (dynamic, slower startup)
echo 'source <(vfrog completion zsh)' >> ~/.zshrc

# Option 2: Generate completion file (recommended for faster shell startup)
vfrog completion zsh > "${fpath[1]}/_vfrog"

# Reload shell
source ~/.zshrc
```

**Bash:**

```bash
# Linux: Add to ~/.bashrc
echo 'source <(vfrog completion bash)' >> ~/.bashrc
source ~/.bashrc

# macOS: Install bash-completion first, then add to ~/.bash_profile
brew install bash-completion@2
echo 'source <(vfrog completion bash)' >> ~/.bash_profile
source ~/.bash_profile
```

**Fish:**

```bash
# Create completions directory if it doesn't exist
mkdir -p ~/.config/fish/completions

# Generate completion file
vfrog completion fish > ~/.config/fish/completions/vfrog.fish

# Reload shell (or restart terminal)
```

**PowerShell:**

```powershell
# Add to PowerShell profile for persistence
vfrog completion powershell | Out-String | Add-Content $PROFILE

# Or run once per session
vfrog completion powershell | Out-String | Invoke-Expression
```

### Testing Autocompletion

After setup, test autocompletion by typing:

```bash
vfrog <TAB>          # Shows all available commands
vfrog projects <TAB> # Shows subcommands (list, create)
vfrog --<TAB>        # Shows all available flags
```

### Troubleshooting

- **Zsh**: If completion doesn't work, ensure `compinit` is enabled: `autoload -Uz compinit && compinit`
- **Bash**: On macOS, ensure bash-completion@2 is installed via Homebrew
- **Fish**: Ensure the completions directory exists: `mkdir -p ~/.config/fish/completions`
- **All shells**: Restart your terminal after setup

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

This will prompt for your email and password and store authentication tokens locally in `~/.vfrog/config-<environment>.json` (e.g., `config-local.json` for `vfrog-local`, `config-dev.json` for `vfrog-dev`).

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

# Start SSAT for an iteration (uses linked dataset images, default count based on iteration number)
vfrog iterations ssat --iteration_id <id>

# Start SSAT with random selection of dataset images from the project
vfrog iterations ssat --iteration_id <id> --random 50

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

## Configuration Files

Each environment binary uses a separate config file to avoid conflicts:

- `vfrog-local` → `~/.vfrog/config-local.json`
- `vfrog-dev` → `~/.vfrog/config-dev.json`
- `vfrog-staging` → `~/.vfrog/config-staging.json`
- `vfrog` (production) → `~/.vfrog/config-production.json`

This means you can have different organisations, projects, and authentication tokens for each environment without interference.

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

### Building for Local Development

To build a CLI that connects to your local development environment (docker-compose):

```bash
# Build vfrog-local (points to localhost:8005 for API project)
make build-local LOCAL_SUPABASE_URL="https://your-supabase.supabase.co" LOCAL_SUPABASE_KEY="your-publishable-key"

# Or set environment variables
export LOCAL_SUPABASE_URL="https://your-supabase.supabase.co"
export LOCAL_SUPABASE_KEY="your-publishable-key"
make build-local
```

**Prerequisites:**

1. Start local services: `cd ../annotator_local_dev && docker-compose up -d`
2. Ensure API project is running on `http://localhost:8005`
3. Ensure Supabase credentials are configured in your `.env` file

**Usage:**

```bash
# Use vfrog-local instead of vfrog-dev
./vfrog-local login
./vfrog-local projects list
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
