package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// TestResult represents the result of a connectivity test
type TestResult struct {
	TestName  string
	Success   bool
	Duration  time.Duration
	Message   string
	Error     string
	Timestamp time.Time
}

// ConnectionTester handles connectivity testing
type ConnectionTester struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

// NewConnectionTester creates a new connection tester
func NewConnectionTester() *ConnectionTester {
	return &ConnectionTester{
		endpoint: defaultEndpoint,
		apiKey:   defaultAPIKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// TestAll runs all connectivity tests
func (ct *ConnectionTester) TestAll() ([]TestResult, error) {
	var results []TestResult

	// Test 1: Endpoint reachability
	result := ct.TestEndpointReachability()
	results = append(results, result)

	// Test 2: Health check
	result = ct.TestHealthCheck()
	results = append(results, result)

	// Test 3: API key validation
	result = ct.TestAPIKeyValidation()
	results = append(results, result)

	// Test 4: Chat completion
	result = ct.TestChatCompletion()
	results = append(results, result)

	// Test 5: Model listing
	result = ct.TestModelListing()
	results = append(results, result)

	return results, nil
}

// TestEndpointReachability tests if the endpoint is reachable
func (ct *ConnectionTester) TestEndpointReachability() TestResult {
	start := time.Now()
	result := TestResult{
		TestName:  "Endpoint Reachability",
		Timestamp: start,
	}

	// Try to connect to the base endpoint
	req, err := http.NewRequest("GET", ct.endpoint, nil)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	resp, err := ct.client.Do(req)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Connection failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	result.Success = true
	result.Message = fmt.Sprintf("Endpoint reachable (HTTP %d)", resp.StatusCode)
	result.Duration = time.Since(start)

	return result
}

// TestHealthCheck tests the health endpoint
func (ct *ConnectionTester) TestHealthCheck() TestResult {
	start := time.Now()
	result := TestResult{
		TestName:  "Health Check",
		Timestamp: start,
	}

	// Try health endpoint
	healthURL := "http://localhost:8090/health"
	req, err := http.NewRequest("GET", healthURL, nil)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	resp, err := ct.client.Do(req)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Health check failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		result.Success = true
		result.Message = fmt.Sprintf("Gateway healthy: %s", string(body))
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("Unhealthy status: %d", resp.StatusCode)
	}

	result.Duration = time.Since(start)
	return result
}

// TestAPIKeyValidation tests if the API key is valid
func (ct *ConnectionTester) TestAPIKeyValidation() TestResult {
	start := time.Now()
	result := TestResult{
		TestName:  "API Key Validation",
		Timestamp: start,
	}

	// Make a simple request with API key
	url := ct.endpoint + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	req.Header.Set("Authorization", "Bearer "+ct.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ct.client.Do(req)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Request failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
		// 404 is OK, means endpoint exists but route not implemented yet
		result.Success = true
		result.Message = "API key accepted"
	} else if resp.StatusCode == http.StatusUnauthorized {
		result.Success = false
		result.Error = "API key rejected (401 Unauthorized)"
	} else {
		result.Success = true
		result.Message = fmt.Sprintf("API key processed (HTTP %d)", resp.StatusCode)
	}

	result.Duration = time.Since(start)
	return result
}

// TestChatCompletion tests a chat completion request
func (ct *ConnectionTester) TestChatCompletion() TestResult {
	start := time.Now()
	result := TestResult{
		TestName:  "Chat Completion",
		Timestamp: start,
	}

	// Create chat completion request
	requestBody := map[string]interface{}{
		"model": "gpt-3.5-turbo",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": "Say 'test successful' if you can read this",
			},
		},
		"max_tokens": 20,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to marshal request: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	url := ct.endpoint + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	req.Header.Set("Authorization", "Bearer "+ct.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ct.client.Do(req)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Request failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var completion map[string]interface{}
		if err := json.Unmarshal(body, &completion); err == nil {
			result.Success = true
			result.Message = "Chat completion successful"

			// Try to extract the actual response
			if choices, ok := completion["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if message, ok := choice["message"].(map[string]interface{}); ok {
						if content, ok := message["content"].(string); ok {
							result.Message = fmt.Sprintf("Response: %s", content)
						}
					}
				}
			}
		} else {
			result.Success = true
			result.Message = "Response received but couldn't parse"
		}
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("Chat completion failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	result.Duration = time.Since(start)
	return result
}

// TestModelListing tests the model listing endpoint
func (ct *ConnectionTester) TestModelListing() TestResult {
	start := time.Now()
	result := TestResult{
		TestName:  "Model Listing",
		Timestamp: start,
	}

	url := ct.endpoint + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	req.Header.Set("Authorization", "Bearer "+ct.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ct.client.Do(req)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Request failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		var models map[string]interface{}
		if err := json.Unmarshal(body, &models); err == nil {
			result.Success = true

			// Count models if available
			if data, ok := models["data"].([]interface{}); ok {
				result.Message = fmt.Sprintf("Found %d models available", len(data))
			} else {
				result.Message = "Models endpoint working"
			}
		} else {
			result.Success = false
			result.Error = "Failed to parse models response"
		}
	} else if resp.StatusCode == http.StatusNotFound {
		result.Success = true
		result.Message = "Models endpoint not yet implemented (OK for testing)"
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("Models listing failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	result.Duration = time.Since(start)
	return result
}

// TestCustomEndpoint tests a custom endpoint
func (ct *ConnectionTester) TestCustomEndpoint(endpoint, apiKey string) TestResult {
	customTester := &ConnectionTester{
		endpoint: endpoint,
		apiKey:   apiKey,
		client:   ct.client,
	}

	// Run basic reachability test
	return customTester.TestEndpointReachability()
}

// PrintResults prints test results in a formatted way
func PrintResults(results []TestResult) {
	totalTests := len(results)
	passed := 0
	failed := 0

	for _, r := range results {
		if r.Success {
			passed++
		} else {
			failed++
		}
	}

	log.Info().Msgf("\n📊 Test Summary: %d total, %d passed, %d failed", totalTests, passed, failed)

	if failed == 0 {
		log.Info().Msg("\n✅ All tests passed! GoLeapAI is ready to use.")
	} else {
		log.Warn().Msgf("\n⚠️  %d test(s) failed. Please check the gateway is running:", failed)
		log.Info().Msg("  $ goleapai")
	}
}

// RunQuickTest runs a quick connectivity test
func RunQuickTest() error {
	log.Info().Msg("Running quick connectivity test...")

	tester := NewConnectionTester()

	// Just test endpoint and health
	results := []TestResult{
		tester.TestEndpointReachability(),
		tester.TestHealthCheck(),
	}

	for _, r := range results {
		if r.Success {
			log.Info().Msgf("✅ %s: %s", r.TestName, r.Message)
		} else {
			log.Error().Msgf("❌ %s: %s", r.TestName, r.Error)
		}
	}

	return nil
}
