# GoLeapAI Configure Tool

Auto-configuration tool for CLI coding applications to use GoLeapAI Gateway.

## Features

- **Auto-Detection**: Automatically detects installed CLI coding tools
- **Smart Configuration**: Generates optimal configuration for each tool
- **Backup Support**: Creates backups before modifying existing configs
- **Connectivity Testing**: Tests gateway connectivity and validates setup
- **Dry Run Mode**: Preview changes before applying them

## Supported Tools

| Tool | Config Location | Status |
|------|----------------|--------|
| Claude Code | `~/.config/claude-code` | ✅ Supported |
| Continue | `~/.continue` | ✅ Supported |
| Cursor | `~/.cursor` | ✅ Supported |
| Aider | `~/.aider` | ✅ Supported |
| Codex | `~/.codex` | ✅ Supported |

## Installation

### Build from source

```bash
cd /home/lisergico25/projects/goleapifree/cmd/configure
go build -o goleapai-configure
```

### Install to PATH

```bash
go build -o $GOPATH/bin/goleapai-configure
```

Or use the main `goleapai` binary:

```bash
goleapai configure --help
```

## Usage

### Basic Usage

```bash
# Configure all detected tools
goleapai-configure --all

# Or using the main binary
goleapai configure --all
```

### Test Mode

Test connectivity without making any changes:

```bash
goleapai-configure --test
```

### Dry Run

Preview what changes would be made:

```bash
goleapai-configure --dry-run
```

### Without Backup

Configure without creating backups (not recommended):

```bash
goleapai-configure --all --no-backup
```

### Verbose Mode

Get detailed output:

```bash
goleapai-configure --all -v
```

## Configuration Details

### Claude Code

**Location**: `~/.config/claude-code/config.json`

**Generated Config**:
```json
{
  "apiEndpoint": "http://localhost:8090/v1",
  "apiKey": "goleapai-free-tier",
  "provider": "openai",
  "model": "gpt-4o",
  "maxTokens": 4096,
  "temperature": 0.7
}
```

### Continue

**Location**: `~/.continue/config.json`

**Generated Config**:
```json
{
  "models": [
    {
      "title": "GoLeapAI GPT-4o",
      "provider": "openai",
      "model": "gpt-4o",
      "apiKey": "goleapai-free-tier",
      "apiBase": "http://localhost:8090/v1",
      "contextLength": 128000
    },
    {
      "title": "GoLeapAI Claude",
      "provider": "anthropic",
      "model": "claude-3-5-sonnet-20241022",
      "apiKey": "goleapai-free-tier",
      "apiBase": "http://localhost:8090/v1",
      "contextLength": 200000
    }
  ]
}
```

### Cursor

**Location**: `~/.cursor/User/settings.json`

**Generated Config**:
```json
{
  "cursor.openaiBaseUrl": "http://localhost:8090/v1",
  "cursor.openaiApiKey": "goleapai-free-tier",
  "cursor.defaultModel": "gpt-4o",
  "cursor.aiEnabled": true
}
```

### Aider

**Location**: `~/.aider/config.yml`

**Generated Config**:
```yaml
openai-api-base: http://localhost:8090/v1
openai-api-key: goleapai-free-tier
model: gpt-4o
edit-format: whole
auto-commits: true
```

## Connectivity Tests

The tool performs the following connectivity tests:

1. **Endpoint Reachability**: Checks if the gateway is accessible
2. **Health Check**: Verifies gateway health status
3. **API Key Validation**: Tests if the API key is accepted
4. **Chat Completion**: Performs a test chat completion request
5. **Model Listing**: Retrieves available models

## Example Output

```
🔧 GoLeapAI Auto-Configuration Tool
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📡 Detecting installed CLI coding tools...

✅ Found 3 tool(s):
  • Claude Code (~/.config/claude-code/config.json) - not configured
  • Continue (~/.continue/config.json) - already configured
  • Cursor (~/.cursor/User/settings.json) - not configured

⚙️  Configuring tools...
🔧 Configuring Claude Code...
  ✓ Backup created
  ✓ Claude Code configured successfully
🔧 Configuring Cursor...
  ✓ Backup created
  ✓ Cursor configured successfully

🧪 Testing connectivity...
✅ PASS Endpoint Reachability: Endpoint reachable (HTTP 200)
✅ PASS Health Check: Gateway healthy
✅ PASS API Key Validation: API key accepted
✅ PASS Chat Completion: Response received
✅ PASS Model Listing: Found 15 models available

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Configuration Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Successfully configured: [Claude Code Cursor]

💡 Next steps:
  1. Start GoLeapAI gateway: goleapai serve
  2. Use your CLI tool normally - it will use GoLeapAI!
  3. Monitor traffic: goleapai stats
```

## Backup Files

Backups are created with timestamps:

```
~/.config/claude-code/config.json.backup-20260205-143022
~/.continue/config.json.backup-20260205-143022
```

## Restore from Backup

To restore a backup manually:

```bash
# Find your backup
ls -la ~/.config/claude-code/*.backup-*

# Restore it
cp ~/.config/claude-code/config.json.backup-20260205-143022 \
   ~/.config/claude-code/config.json
```

## Troubleshooting

### Gateway Not Running

If connectivity tests fail, ensure the gateway is running:

```bash
# Start the gateway
goleapai serve

# Or in the background
goleapai serve &
```

### Port Already in Use

If port 8090 is in use, modify the gateway config:

```bash
goleapai serve --port 8091
```

Then update tool configs to use the new port.

### Permission Denied

If you get permission errors, check directory permissions:

```bash
ls -la ~/.config/claude-code
chmod 755 ~/.config/claude-code
```

## Manual Configuration

If auto-configuration fails, you can configure manually:

### Claude Code

Edit `~/.config/claude-code/config.json`:
```json
{
  "apiEndpoint": "http://localhost:8090/v1",
  "apiKey": "goleapai-free-tier"
}
```

### Continue

Edit `~/.continue/config.json`:
```json
{
  "models": [{
    "apiBase": "http://localhost:8090/v1",
    "apiKey": "goleapai-free-tier",
    "model": "gpt-4o"
  }]
}
```

### Cursor

Edit `~/.cursor/User/settings.json`:
```json
{
  "cursor.openaiBaseUrl": "http://localhost:8090/v1",
  "cursor.openaiApiKey": "goleapai-free-tier"
}
```

## Architecture

```
cmd/configure/
├── main.go         # Main entry point and CLI interface
├── detectors.go    # Tool detection logic
├── generators.go   # Configuration generators
├── tester.go       # Connectivity testing
└── README.md       # This file
```

## API Reference

### Detectors

- `DetectAllTools()`: Detect all supported tools
- `FindClaudeCode()`: Find Claude Code installation
- `FindContinue()`: Find Continue installation
- `FindCursor()`: Find Cursor installation
- `FindAider()`: Find Aider installation
- `FindCodex()`: Find Codex installation

### Generators

- `GenerateConfig(tool)`: Generate config for detected tool
- `BackupConfig(tool)`: Create backup of existing config
- `PreviewConfig(tool)`: Preview generated configuration

### Testers

- `TestAll()`: Run all connectivity tests
- `TestEndpointReachability()`: Test gateway accessibility
- `TestHealthCheck()`: Test health endpoint
- `TestAPIKeyValidation()`: Test API key
- `TestChatCompletion()`: Test chat completion
- `TestModelListing()`: Test model listing

## Contributing

To add support for a new tool:

1. Add detection logic in `detectors.go`
2. Add config generation in `generators.go`
3. Add the new tool type to the enum
4. Update this README

## License

Part of GoLeapAI project - see main LICENSE file.
