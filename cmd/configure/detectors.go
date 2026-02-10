package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

// DetectedTool represents a detected CLI coding tool
type DetectedTool struct {
	Name          string
	Type          ToolType
	ConfigPath    string
	ConfigFile    string
	IsConfigured  bool
	Version       string
	CustomData    map[string]interface{}
}

// ToolType represents the type of CLI tool
type ToolType string

const (
	ToolClaudeCode ToolType = "claude-code"
	ToolContinue   ToolType = "continue"
	ToolCursor     ToolType = "cursor"
	ToolAider      ToolType = "aider"
	ToolCodex      ToolType = "codex"
)

// DetectAllTools detects all installed CLI coding tools
func DetectAllTools() ([]DetectedTool, error) {
	var tools []DetectedTool

	detectors := []func() (*DetectedTool, error){
		FindClaudeCode,
		FindContinue,
		FindCursor,
		FindAider,
		FindCodex,
	}

	for _, detector := range detectors {
		tool, err := detector()
		if err != nil {
			log.Debug().Err(err).Msg("Detector error")
			continue
		}
		if tool != nil {
			tools = append(tools, *tool)
		}
	}

	return tools, nil
}

// FindClaudeCode detects Claude Code installation
func FindClaudeCode() (*DetectedTool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPaths := []string{
		filepath.Join(homeDir, ".config", "claude-code"),
		filepath.Join(homeDir, ".claude-code"),
		filepath.Join(homeDir, "Library", "Application Support", "claude-code"), // macOS
	}

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			tool := &DetectedTool{
				Name:       "Claude Code",
				Type:       ToolClaudeCode,
				ConfigPath: configPath,
				ConfigFile: filepath.Join(configPath, "config.json"),
			}

			// Check if already configured
			tool.IsConfigured = checkClaudeCodeConfig(tool.ConfigFile)

			// Get version if available
			tool.Version = getClaudeCodeVersion(configPath)

			return tool, nil
		}
	}

	return nil, nil
}

// FindContinue detects Continue installation
func FindContinue() (*DetectedTool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPaths := []string{
		filepath.Join(homeDir, ".continue"),
		filepath.Join(homeDir, ".config", "continue"),
	}

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			tool := &DetectedTool{
				Name:       "Continue",
				Type:       ToolContinue,
				ConfigPath: configPath,
				ConfigFile: filepath.Join(configPath, "config.json"),
			}

			tool.IsConfigured = checkContinueConfig(tool.ConfigFile)
			tool.Version = getContinueVersion(configPath)

			return tool, nil
		}
	}

	return nil, nil
}

// FindCursor detects Cursor installation
func FindCursor() (*DetectedTool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPaths := []string{
		filepath.Join(homeDir, ".cursor"),
		filepath.Join(homeDir, ".config", "cursor"),
		filepath.Join(homeDir, "Library", "Application Support", "Cursor"), // macOS
	}

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			tool := &DetectedTool{
				Name:       "Cursor",
				Type:       ToolCursor,
				ConfigPath: configPath,
				ConfigFile: filepath.Join(configPath, "User", "settings.json"),
			}

			tool.IsConfigured = checkCursorConfig(tool.ConfigFile)
			tool.Version = getCursorVersion(configPath)

			return tool, nil
		}
	}

	return nil, nil
}

// FindAider detects Aider installation
func FindAider() (*DetectedTool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPaths := []string{
		filepath.Join(homeDir, ".aider"),
		filepath.Join(homeDir, ".config", "aider"),
	}

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			tool := &DetectedTool{
				Name:       "Aider",
				Type:       ToolAider,
				ConfigPath: configPath,
				ConfigFile: filepath.Join(configPath, "config.yml"),
			}

			tool.IsConfigured = checkAiderConfig(tool.ConfigFile)
			tool.Version = getAiderVersion(configPath)

			return tool, nil
		}
	}

	return nil, nil
}

// FindCodex detects Codex CLI installation
func FindCodex() (*DetectedTool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPaths := []string{
		filepath.Join(homeDir, ".codex"),
		filepath.Join(homeDir, ".config", "codex"),
	}

	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			tool := &DetectedTool{
				Name:       "Codex",
				Type:       ToolCodex,
				ConfigPath: configPath,
				ConfigFile: filepath.Join(configPath, "config.json"),
			}

			tool.IsConfigured = checkCodexConfig(tool.ConfigFile)
			tool.Version = getCodexVersion(configPath)

			return tool, nil
		}
	}

	return nil, nil
}

// checkClaudeCodeConfig checks if Claude Code is configured for GoLeapAI
func checkClaudeCodeConfig(configFile string) bool {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	// Check for GoLeapAI endpoint
	if endpoint, ok := config["apiEndpoint"].(string); ok {
		return endpoint == "http://localhost:8090/v1" || endpoint == "http://localhost:8090"
	}

	return false
}

// checkContinueConfig checks if Continue is configured for GoLeapAI
func checkContinueConfig(configFile string) bool {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	// Check models array for GoLeapAI configuration
	if models, ok := config["models"].([]interface{}); ok {
		for _, m := range models {
			if model, ok := m.(map[string]interface{}); ok {
				if apiBase, ok := model["apiBase"].(string); ok {
					if apiBase == "http://localhost:8090/v1" || apiBase == "http://localhost:8090" {
						return true
					}
				}
			}
		}
	}

	return false
}

// checkCursorConfig checks if Cursor is configured for GoLeapAI
func checkCursorConfig(configFile string) bool {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	// Check for custom API endpoint
	if endpoint, ok := config["cursor.openaiBaseUrl"].(string); ok {
		return endpoint == "http://localhost:8090/v1" || endpoint == "http://localhost:8090"
	}

	return false
}

// checkAiderConfig checks if Aider is configured for GoLeapAI
func checkAiderConfig(configFile string) bool {
	// Aider uses YAML, simple string check
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}

	content := string(data)
	return contains(content, "localhost:8090")
}

// checkCodexConfig checks if Codex is configured for GoLeapAI
func checkCodexConfig(configFile string) bool {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return false
	}

	if endpoint, ok := config["apiEndpoint"].(string); ok {
		return endpoint == "http://localhost:8090/v1" || endpoint == "http://localhost:8090"
	}

	return false
}

// Version getters
func getClaudeCodeVersion(configPath string) string {
	versionFile := filepath.Join(configPath, "version")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return "unknown"
	}
	return string(data)
}

func getContinueVersion(configPath string) string {
	return getVersionFromPackage(filepath.Join(configPath, "package.json"))
}

func getCursorVersion(configPath string) string {
	return getVersionFromPackage(filepath.Join(configPath, "package.json"))
}

func getAiderVersion(configPath string) string {
	return "detected"
}

func getCodexVersion(configPath string) string {
	return "detected"
}

func getVersionFromPackage(packageFile string) string {
	data, err := os.ReadFile(packageFile)
	if err != nil {
		return "unknown"
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "unknown"
	}

	if version, ok := pkg["version"].(string); ok {
		return version
	}

	return "unknown"
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// DetectTool detects a specific tool by name
func DetectTool(toolName string) (*DetectedTool, error) {
	switch toolName {
	case "claude-code":
		return FindClaudeCode()
	case "continue":
		return FindContinue()
	case "cursor":
		return FindCursor()
	case "aider":
		return FindAider()
	case "codex":
		return FindCodex()
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}
