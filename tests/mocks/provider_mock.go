package mocks

import (
	"context"
	"sync"
)

// MockProvider is a mock LLM provider for testing
type MockProvider struct {
	mu                sync.RWMutex
	Name              string
	BaseURL           string
	Available         bool
	Latency           int
	ErrorRate         float64
	Responses         []MockResponse
	RequestCount      int
	LastRequest       *MockRequest
	ShouldFail        bool
	FailureMessage    string
}

// MockRequest represents a request to the mock provider
type MockRequest struct {
	Model       string
	Messages    []MockMessage
	Temperature float64
	MaxTokens   int
	Stream      bool
}

// MockMessage represents a message in a request
type MockMessage struct {
	Role    string
	Content string
}

// MockResponse represents a response from the mock provider
type MockResponse struct {
	Content      string
	TokensUsed   int
	FinishReason string
	Error        error
}

// NewMockProvider creates a new mock provider
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		Name:      name,
		BaseURL:   "https://mock-" + name + ".test",
		Available: true,
		Latency:   100,
		ErrorRate: 0.0,
		Responses: []MockResponse{
			{
				Content:      "This is a mock response",
				TokensUsed:   50,
				FinishReason: "stop",
			},
		},
	}
}

// SendRequest simulates sending a request
func (m *MockProvider) SendRequest(ctx context.Context, req *MockRequest) (*MockResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RequestCount++
	m.LastRequest = req

	if m.ShouldFail {
		return nil, &MockError{Message: m.FailureMessage}
	}

	if len(m.Responses) == 0 {
		return &MockResponse{
			Content:      "Default mock response",
			TokensUsed:   50,
			FinishReason: "stop",
		}, nil
	}

	// Return the next response in queue
	resp := m.Responses[0]
	if len(m.Responses) > 1 {
		m.Responses = m.Responses[1:]
	}

	return &resp, resp.Error
}

// IsAvailable returns if the provider is available
func (m *MockProvider) IsAvailable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Available
}

// SetAvailable sets the availability
func (m *MockProvider) SetAvailable(available bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Available = available
}

// GetRequestCount returns the number of requests made
func (m *MockProvider) GetRequestCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.RequestCount
}

// GetLastRequest returns the last request
func (m *MockProvider) GetLastRequest() *MockRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.LastRequest
}

// Reset resets the mock provider state
func (m *MockProvider) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RequestCount = 0
	m.LastRequest = nil
	m.ShouldFail = false
	m.FailureMessage = ""
}

// SetError configures the provider to return errors
func (m *MockProvider) SetError(shouldFail bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ShouldFail = shouldFail
	m.FailureMessage = message
}

// AddResponse adds a response to the queue
func (m *MockProvider) AddResponse(resp MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses = append(m.Responses, resp)
}

// ClearResponses clears all queued responses
func (m *MockProvider) ClearResponses() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses = []MockResponse{}
}

// MockError represents a mock error
type MockError struct {
	Message string
}

func (e *MockError) Error() string {
	return e.Message
}
