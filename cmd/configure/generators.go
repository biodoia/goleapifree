package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	defaultEndpoint = "http://localhost:8090/v1"
	defaultAPIKey   = "goleapai-free-tier"
)

// GenerateConfig generates configuration for a detected tool
func GenerateConfig(tool DetectedTool) error {
	switch tool.Type {
	case ToolClaudeCode:
		return GenerateClaudeCodeConfig(tool)
	case ToolContinue:
		return GenerateContinueConfig(tool)
	case ToolCursor:
		return GenerateCursorConfig(tool)
	case ToolAider:
		return GenerateAiderConfig(tool)
	case ToolCodex:
		return GenerateCodexConfig(tool)
	default:
		return fmt.Errorf("unsupported tool type: %s", tool.Type)
	}
}

// GenerateClaudeCodeConfig generates Claude Code configuration
func GenerateClaudeCodeConfig(tool DetectedTool) error {
	config := map[string]interface{}{
		"apiEndpoint": defaultEndpoint,
		"apiKey":      defaultAPIKey,
		"provider":    "openai",
		"model":       "gpt-4o",
		"maxTokens":   4096,
		"temperature": 0.7,
		"configured": map[string]interface{}{
			"by":   "goleapai-configure",
			"at":   time.Now().Format(time.RFC3339),
			"note": "Auto-configured to use GoLeapAI Gateway",
		},
	}

	// Ensure config directory exists
	if err := os.MkdirAll(tool.ConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(tool.ConfigFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Debug().
		Str("file", tool.ConfigFile).
		Msg("Claude Code config generated")

	return nil
}

// GenerateContinueConfig generates Continue configuration
func GenerateContinueConfig(tool DetectedTool) error {
	config := map[string]interface{}{
		"models": []map[string]interface{}{
			{
				"title":       "GoLeapAI GPT-4o",
				"provider":    "openai",
				"model":       "gpt-4o",
				"apiKey":      defaultAPIKey,
				"apiBase":     defaultEndpoint,
				"contextLength": 128000,
			},
			{
				"title":       "GoLeapAI GPT-4",
				"provider":    "openai",
				"model":       "gpt-4",
				"apiKey":      defaultAPIKey,
				"apiBase":     defaultEndpoint,
				"contextLength": 8192,
			},
			{
				"title":       "GoLeapAI Claude",
				"provider":    "anthropic",
				"model":       "claude-3-5-sonnet-20241022",
				"apiKey":      defaultAPIKey,
				"apiBase":     defaultEndpoint,
				"contextLength": 200000,
			},
		},
		"tabAutocompleteModel": map[string]interface{}{
			"title":    "GoLeapAI Autocomplete",
			"provider": "openai",
			"model":    "gpt-3.5-turbo",
			"apiKey":   defaultAPIKey,
			"apiBase":  defaultEndpoint,
		},
		"embeddingsProvider": map[string]interface{}{
			"provider": "openai",
			"model":    "text-embedding-ada-002",
			"apiKey":   defaultAPIKey,
			"apiBase":  defaultEndpoint,
		},
		"configured": map[string]interface{}{
			"by":   "goleapai-configure",
			"at":   time.Now().Format(time.RFC3339),
			"note": "Auto-configured to use GoLeapAI Gateway with multiple models",
		},
	}

	// Ensure config directory exists
	if err := os.MkdirAll(tool.ConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(tool.ConfigFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Debug().
		Str("file", tool.ConfigFile).
		Msg("Continue config generated")

	return nil
}

// GenerateCursorConfig generates Cursor configuration
func GenerateCursorConfig(tool DetectedTool) error {
	// Cursor uses VS Code settings format
	config := map[string]interface{}{
		"cursor.openaiBaseUrl":     defaultEndpoint,
		"cursor.openaiApiKey":      defaultAPIKey,
		"cursor.defaultModel":      "gpt-4o",
		"cursor.aiEnabled":         true,
		"cursor.contextLength":     128000,
		"cursor.temperature":       0.7,
		"cursor.maxTokens":         4096,
		"editor.inlineSuggest.enabled": true,
		"configured": map[string]interface{}{
			"by":   "goleapai-configure",
			"at":   time.Now().Format(time.RFC3339),
			"note": "Auto-configured to use GoLeapAI Gateway",
		},
	}

	// Ensure config directory exists
	userDir := filepath.Join(tool.ConfigPath, "User")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Read existing config if present
	existingConfig := make(map[string]interface{})
	if data, err := os.ReadFile(tool.ConfigFile); err == nil {
		if err := json.Unmarshal(data, &existingConfig); err != nil {
			log.Warn().Err(err).Msg("Failed to parse existing config, will overwrite")
		}
	}

	// Merge with existing config
	for key, value := range config {
		existingConfig[key] = value
	}

	// Write config file
	data, err := json.MarshalIndent(existingConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(tool.ConfigFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Debug().
		Str("file", tool.ConfigFile).
		Msg("Cursor config generated")

	return nil
}

// GenerateAiderConfig generates Aider configuration
func GenerateAiderConfig(tool DetectedTool) error {
	// Aider uses YAML format
	config := fmt.Sprintf(`# GoLeapAI Configuration
# Auto-configured by goleapai-configure at %s

openai-api-base: %s
openai-api-key: %s
model: gpt-4o
edit-format: whole
auto-commits: true
dirty-commits: true
pretty: true

# Additional models available
# model: gpt-4
# model: claude-3-5-sonnet-20241022
# model: gpt-3.5-turbo

# Note: All requests will be routed through GoLeapAI Gateway
# which provides access to multiple free AI providers
`, time.Now().Format(time.RFC3339), defaultEndpoint, defaultAPIKey)

	// Ensure config directory exists
	if err := os.MkdirAll(tool.ConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	if err := os.WriteFile(tool.ConfigFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Debug().
		Str("file", tool.ConfigFile).
		Msg("Aider config generated")

	return nil
}

// GenerateCodexConfig generates Codex configuration
func GenerateCodexConfig(tool DetectedTool) error {
	config := map[string]interface{}{
		"apiEndpoint": defaultEndpoint,
		"apiKey":      defaultAPIKey,
		"model":       "gpt-4o",
		"maxTokens":   4096,
		"temperature": 0.7,
		"provider":    "openai",
		"configured": map[string]interface{}{
			"by":   "goleapai-configure",
			"at":   time.Now().Format(time.RFC3339),
			"note": "Auto-configured to use GoLeapAI Gateway",
		},
	}

	// Ensure config directory exists
	if err := os.MkdirAll(tool.ConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(tool.ConfigFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Debug().
		Str("file", tool.ConfigFile).
		Msg("Codex config generated")

	return nil
}

// BackupConfig creates a backup of existing configuration
func BackupConfig(tool DetectedTool) error {
	if _, err := os.Stat(tool.ConfigFile); os.IsNotExist(err) {
		return nil // No config to backup
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupFile := fmt.Sprintf("%s.backup-%s", tool.ConfigFile, timestamp)

	// Read original config
	data, err := os.ReadFile(tool.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Write backup
	if err := os.WriteFile(backupFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	log.Info().
		Str("backup", backupFile).
		Msgf("  ✓ Backup created: %s", filepath.Base(backupFile))

	return nil
}

// PreviewConfig shows what configuration would be generated
func PreviewConfig(tool DetectedTool) (string, error) {
	switch tool.Type {
	case ToolClaudeCode:
		return previewClaudeCodeConfig(), nil
	case ToolContinue:
		return previewContinueConfig(), nil
	case ToolCursor:
		return previewCursorConfig(), nil
	case ToolAider:
		return previewAiderConfig(), nil
	case ToolCodex:
		return previewCodexConfig(), nil
	default:
		return "", fmt.Errorf("unsupported tool type: %s", tool.Type)
	}
}

func previewClaudeCodeConfig() string {
	return fmt.Sprintf(`{
  "apiEndpoint": "%s",
  "apiKey": "%s",
  "provider": "openai",
  "model": "gpt-4o"
}`, defaultEndpoint, defaultAPIKey)
}

func previewContinueConfig() string {
	return fmt.Sprintf(`{
  "models": [
    {
      "title": "GoLeapAI GPT-4o",
      "apiBase": "%s",
      "apiKey": "%s",
      "model": "gpt-4o"
    },
    ...
  ]
}`, defaultEndpoint, defaultAPIKey)
}

func previewCursorConfig() string {
	return fmt.Sprintf(`{
  "cursor.openaiBaseUrl": "%s",
  "cursor.openaiApiKey": "%s",
  "cursor.defaultModel": "gpt-4o"
}`, defaultEndpoint, defaultAPIKey)
}

func previewAiderConfig() string {
	return fmt.Sprintf(`openai-api-base: %s
openai-api-key: %s
model: gpt-4o`, defaultEndpoint, defaultAPIKey)
}

func previewCodexConfig() string {
	return fmt.Sprintf(`{
  "apiEndpoint": "%s",
  "apiKey": "%s",
  "model": "gpt-4o"
}`, defaultEndpoint, defaultAPIKey)
}

// RestoreConfig restores a backed up configuration
func RestoreConfig(tool DetectedTool, backupFile string) error {
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(tool.ConfigFile, data, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	log.Info().
		Str("tool", tool.Name).
		Msg("Configuration restored from backup")

	return nil
}
