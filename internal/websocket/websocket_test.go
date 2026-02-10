package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEvent_ToJSON(t *testing.T) {
	event, err := NewEvent(EventTypeLog, LogEvent{
		Level:     "info",
		Message:   "Test message",
		Component: "test",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	data, err := event.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize event: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to deserialize event: %v", err)
	}

	if decoded.Type != EventTypeLog {
		t.Errorf("Expected type %s, got %s", EventTypeLog, decoded.Type)
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:       uuid.New().String(),
		hub:      hub,
		send:     make(chan []byte, 256),
		channels: make(map[Channel]bool),
	}

	// Register client
	hub.register <- client
	time.Sleep(10 * time.Millisecond)

	stats := hub.GetStats()
	if stats.TotalClients != 1 {
		t.Errorf("Expected 1 client, got %d", stats.TotalClients)
	}

	// Unregister client
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	stats = hub.GetStats()
	if stats.TotalClients != 0 {
		t.Errorf("Expected 0 clients, got %d", stats.TotalClients)
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:       uuid.New().String(),
		hub:      hub,
		send:     make(chan []byte, 256),
		channels: make(map[Channel]bool),
	}

	// Register and subscribe
	hub.register <- client
	hub.Subscribe(client, ChannelLogs)
	time.Sleep(10 * time.Millisecond)

	// Broadcast message
	event, err := NewEvent(EventTypeLog, LogEvent{
		Level:     "info",
		Message:   "Test broadcast",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Failed to create event: %v", err)
	}

	hub.Broadcast(ChannelLogs, event)
	time.Sleep(10 * time.Millisecond)

	// Check if message was received
	select {
	case msg := <-client.send:
		var decoded Event
		if err := json.Unmarshal(msg, &decoded); err != nil {
			t.Fatalf("Failed to decode message: %v", err)
		}
		if decoded.Type != EventTypeLog {
			t.Errorf("Expected type %s, got %s", EventTypeLog, decoded.Type)
		}
	default:
		t.Error("Expected to receive message")
	}
}

func TestHub_ChannelSubscription(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client1 := &Client{
		ID:       "client1",
		hub:      hub,
		send:     make(chan []byte, 256),
		channels: make(map[Channel]bool),
	}

	client2 := &Client{
		ID:       "client2",
		hub:      hub,
		send:     make(chan []byte, 256),
		channels: make(map[Channel]bool),
	}

	// Register clients
	hub.register <- client1
	hub.register <- client2
	time.Sleep(10 * time.Millisecond)

	// Subscribe to different channels
	hub.Subscribe(client1, ChannelLogs)
	hub.Subscribe(client2, ChannelStats)
	time.Sleep(10 * time.Millisecond)

	// Broadcast to logs channel
	event, _ := NewEvent(EventTypeLog, LogEvent{
		Level:   "info",
		Message: "Test",
	})
	hub.Broadcast(ChannelLogs, event)
	time.Sleep(10 * time.Millisecond)

	// Client1 should receive, client2 should not
	select {
	case <-client1.send:
		// Expected
	default:
		t.Error("Client1 should have received message")
	}

	select {
	case <-client2.send:
		t.Error("Client2 should not have received message")
	default:
		// Expected
	}
}

func TestHub_ChannelAll(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{
		ID:       uuid.New().String(),
		hub:      hub,
		send:     make(chan []byte, 256),
		channels: make(map[Channel]bool),
	}

	// Register and subscribe to "all"
	hub.register <- client
	hub.Subscribe(client, ChannelAll)
	time.Sleep(10 * time.Millisecond)

	// Broadcast to any channel
	event, _ := NewEvent(EventTypeLog, LogEvent{Level: "info"})
	hub.Broadcast(ChannelLogs, event)
	time.Sleep(10 * time.Millisecond)

	// Should receive message
	select {
	case <-client.send:
		// Expected
	default:
		t.Error("Client subscribed to 'all' should receive all messages")
	}
}

func TestIsValidChannel(t *testing.T) {
	tests := []struct {
		channel string
		valid   bool
	}{
		{"logs", true},
		{"stats", true},
		{"providers", true},
		{"requests", true},
		{"all", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		result := IsValidChannel(tt.channel)
		if result != tt.valid {
			t.Errorf("IsValidChannel(%s) = %v, want %v", tt.channel, result, tt.valid)
		}
	}
}

func TestMockTUIClient(t *testing.T) {
	client := NewMockTUIClient()

	stats := client.GetLatestStats()
	if stats.TotalRequests != 1000 {
		t.Errorf("Expected 1000 requests, got %d", stats.TotalRequests)
	}

	if len(stats.ProviderStats) != 3 {
		t.Errorf("Expected 3 providers, got %d", len(stats.ProviderStats))
	}

	// Simulate update
	client.SimulateUpdate()
	stats = client.GetLatestStats()
	if stats.TotalRequests != 1001 {
		t.Errorf("Expected 1001 requests after update, got %d", stats.TotalRequests)
	}
}

func BenchmarkHub_Broadcast(b *testing.B) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	// Create 100 clients
	clients := make([]*Client, 100)
	for i := 0; i < 100; i++ {
		client := &Client{
			ID:       uuid.New().String(),
			hub:      hub,
			send:     make(chan []byte, 256),
			channels: make(map[Channel]bool),
		}
		hub.register <- client
		hub.Subscribe(client, ChannelLogs)
		clients[i] = client
	}
	time.Sleep(100 * time.Millisecond)

	event, _ := NewEvent(EventTypeLog, LogEvent{
		Level:   "info",
		Message: "Benchmark message",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Broadcast(ChannelLogs, event)
	}
}

func BenchmarkEvent_ToJSON(b *testing.B) {
	event, _ := NewEvent(EventTypeStatsUpdate, StatsUpdateEvent{
		TotalRequests: 1000,
		TotalTokens:   50000,
		SuccessRate:   0.95,
		ProviderStats: make([]ProviderStat, 10),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = event.ToJSON()
	}
}
