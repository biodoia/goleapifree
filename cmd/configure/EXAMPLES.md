# GoLeapAI Configure - Usage Examples

## Basic Scenarios

### 1. First Time Setup

When you first install GoLeapAI and want to configure your coding tools:

```bash
# Run the auto-configurator
./goleapai-configure --all

# Expected output:
🔧 GoLeapAI Auto-Configuration Tool
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📡 Detecting installed CLI coding tools...

✅ Found 2 tool(s):
  • Claude Code (~/.config/claude-code) - not configured
  • Continue (~/.continue) - not configured

⚙️  Configuring tools...
🔧 Configuring Claude Code...
  ✓ Claude Code configured successfully
🔧 Configuring Continue...
  ✓ Continue configured successfully

🧪 Testing connectivity...
⚠️  Gateway not running - please start it first:
  $ goleapai serve

✅ Successfully configured: [Claude Code Continue]
```

### 2. Check What Would Be Changed (Dry Run)

Before making any changes, preview what would happen:

```bash
./goleapai-configure --dry-run

# Output:
🔍 DRY RUN - No changes will be made

Claude Code:
  Config path: /home/user/.config/claude-code/config.json
  Changes to be made:
    {
      "apiEndpoint": "http://localhost:8090/v1",
      "apiKey": "goleapai-free-tier",
      "model": "gpt-4o"
    }

Continue:
  Config path: /home/user/.continue/config.json
  Changes to be made:
    {
      "models": [{
        "title": "GoLeapAI GPT-4o",
        "apiBase": "http://localhost:8090/v1",
        ...
      }]
    }
```

### 3. Test Connectivity Only

Check if the gateway is running and accessible:

```bash
./goleapai-configure --test

# Output:
🧪 Testing connectivity...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🔌 Connectivity Test Results
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

✅ PASS Endpoint Reachability:
  Duration: 12ms
  Details: Endpoint reachable (HTTP 200)

✅ PASS Health Check:
  Duration: 8ms
  Details: Gateway healthy

✅ PASS API Key Validation:
  Duration: 15ms
  Details: API key accepted

✅ PASS Chat Completion:
  Duration: 1.2s
  Details: Response: test successful

✅ PASS Model Listing:
  Duration: 23ms
  Details: Found 15 models available
```

### 4. Verbose Mode

Get detailed information about what's happening:

```bash
./goleapai-configure --all -v

# Output includes DEBUG logs:
DEBUG Checking path: /home/user/.config/claude-code
DEBUG Found Claude Code installation
DEBUG Config file: /home/user/.config/claude-code/config.json
DEBUG Backup created: config.json.backup-20260205-143022
DEBUG Writing config to: /home/user/.config/claude-code/config.json
...
```

## Advanced Scenarios

### 5. Reconfigure Existing Setup

If you've already configured tools but want to update them:

```bash
# With --all flag, it will update even already configured tools
./goleapai-configure --all

# Output:
✅ Found 2 tool(s):
  • Claude Code - already configured
  • Continue - already configured

⚙️  Configuring tools...
🔧 Configuring Claude Code...
  ✓ Backup created: config.json.backup-20260205-143530
  ✓ Claude Code configured successfully
🔧 Configuring Continue...
  ✓ Backup created: config.json.backup-20260205-143530
  ✓ Continue configured successfully
```

### 6. Configure Without Backups

If you don't want backups (not recommended):

```bash
./goleapai-configure --all --backup=false
```

### 7. Complete Workflow

Full workflow from installation to usage:

```bash
# Step 1: Build the tool
cd /home/lisergico25/projects/goleapifree/cmd/configure
make build

# Step 2: Preview changes
./goleapai-configure --dry-run

# Step 3: Configure tools
./goleapai-configure --all

# Step 4: Start the gateway (in another terminal)
cd /home/lisergico25/projects/goleapifree
go run cmd/backend/main.go serve

# Step 5: Test connectivity
./goleapai-configure --test

# Step 6: Use your CLI tools normally!
# They will now use GoLeapAI automatically
```

## Specific Tool Examples

### Claude Code Only

If you only have Claude Code installed:

```bash
./goleapai-configure --all

# Will detect and configure only Claude Code:
✅ Found 1 tool(s):
  • Claude Code (~/.config/claude-code) - not configured

✓ Claude Code configured successfully
```

### Continue with Multiple Models

Continue gets configured with multiple models:

```json
{
  "models": [
    {
      "title": "GoLeapAI GPT-4o",
      "provider": "openai",
      "model": "gpt-4o",
      "apiBase": "http://localhost:8090/v1",
      "contextLength": 128000
    },
    {
      "title": "GoLeapAI GPT-4",
      "provider": "openai",
      "model": "gpt-4",
      "apiBase": "http://localhost:8090/v1",
      "contextLength": 8192
    },
    {
      "title": "GoLeapAI Claude",
      "provider": "anthropic",
      "model": "claude-3-5-sonnet-20241022",
      "apiBase": "http://localhost:8090/v1",
      "contextLength": 200000
    }
  ]
}
```

You can then select any of these models in Continue!

### Cursor Settings Merge

Cursor configuration merges with existing settings:

**Before** (`~/.cursor/User/settings.json`):
```json
{
  "editor.fontSize": 14,
  "terminal.integrated.shell": "/bin/zsh"
}
```

**After** (existing settings preserved):
```json
{
  "editor.fontSize": 14,
  "terminal.integrated.shell": "/bin/zsh",
  "cursor.openaiBaseUrl": "http://localhost:8090/v1",
  "cursor.openaiApiKey": "goleapai-free-tier",
  "cursor.defaultModel": "gpt-4o",
  "cursor.aiEnabled": true
}
```

## Troubleshooting Examples

### Gateway Not Running

```bash
./goleapai-configure --test

# Output:
❌ FAIL Endpoint Reachability:
  Duration: 1s
  Error: Connection failed: dial tcp localhost:8090: connect: connection refused

# Solution: Start the gateway first
```

### Port Conflict

If port 8090 is in use:

```bash
# 1. Start gateway on different port
goleapai serve --port 8091

# 2. Manually update configs to use port 8091
# Edit ~/.config/claude-code/config.json
# Change "http://localhost:8090/v1" to "http://localhost:8091/v1"
```

### Permission Issues

```bash
./goleapai-configure --all

# Output:
✗ Failed to configure Claude Code
  Error: failed to write config file: permission denied

# Solution: Fix permissions
chmod 755 ~/.config/claude-code
chmod 644 ~/.config/claude-code/config.json
```

### Restore from Backup

If something goes wrong:

```bash
# List backups
ls -la ~/.config/claude-code/*.backup-*

# Restore
cp ~/.config/claude-code/config.json.backup-20260205-143022 \
   ~/.config/claude-code/config.json

# Or re-run configure
./goleapai-configure --all
```

## Integration Examples

### With Docker

```bash
# Start gateway in Docker
docker run -d -p 8090:8090 goleapai/gateway

# Configure tools
./goleapai-configure --all --test
```

### With Systemd

```bash
# Start gateway as service
sudo systemctl start goleapai

# Configure tools
./goleapai-configure --all

# Verify
./goleapai-configure --test
```

### CI/CD Pipeline

```yaml
# .github/workflows/setup.yml
- name: Configure AI Tools
  run: |
    goleapai-configure --all --no-backup
    goleapai-configure --test
```

## Output Examples

### Success Case

```
🔧 GoLeapAI Auto-Configuration Tool
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📡 Detecting installed CLI coding tools...

✅ Found 3 tool(s):
  • Claude Code (~/.config/claude-code) - not configured
  • Continue (~/.continue) - not configured
  • Aider (~/.aider) - not configured

⚙️  Configuring tools...
🔧 Configuring Claude Code...
  ✓ Claude Code configured successfully
🔧 Configuring Continue...
  ✓ Continue configured successfully
🔧 Configuring Aider...
  ✓ Aider configured successfully

🧪 Testing connectivity...
✅ All tests passed!

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📊 Configuration Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ Successfully configured: [Claude Code Continue Aider]

💡 Next steps:
  1. Start GoLeapAI gateway: goleapai serve
  2. Use your CLI tool normally - it will use GoLeapAI!
  3. Monitor traffic: goleapai stats
```

### Partial Failure

```
✅ Successfully configured: [Claude Code Continue]
❌ Failed to configure: [Aider]

⚠️  Some configurations failed. Check logs above for details.
```

### No Tools Detected

```
⚠️  No CLI coding tools detected

Supported tools:
  - Claude Code (~/.config/claude-code)
  - Continue (~/.continue)
  - Cursor (~/.cursor)
  - Aider (~/.aider)
  - Codex (~/.codex)

Please install a supported tool first.
```

## Tips & Tricks

### 1. Quick Setup Script

Create a setup script:

```bash
#!/bin/bash
# setup-goleapai.sh

# Start gateway
goleapai serve &
GATEWAY_PID=$!

# Wait for gateway to be ready
sleep 2

# Configure tools
goleapai-configure --all

# Test
goleapai-configure --test

echo "Setup complete! Gateway PID: $GATEWAY_PID"
```

### 2. Check Configuration

Verify your tool's configuration:

```bash
# Claude Code
cat ~/.config/claude-code/config.json | jq .

# Continue
cat ~/.continue/config.json | jq '.models[]'

# Cursor
cat ~/.cursor/User/settings.json | jq 'select(.["cursor.openaiBaseUrl"])'
```

### 3. Monitor Usage

After configuration:

```bash
# Start gateway with logging
goleapai serve --verbose

# In another terminal, use your tool
# Then check gateway logs for requests
```

### 4. Multiple Environments

Configure for different environments:

```bash
# Development
export GOLEAPAI_ENDPOINT="http://localhost:8090/v1"
goleapai-configure --all

# Production
export GOLEAPAI_ENDPOINT="https://api.mycompany.com/v1"
goleapai-configure --all
```
