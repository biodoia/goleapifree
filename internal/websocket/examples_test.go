package websocket_test

import (
	"context"
	"fmt"
	"time"

	"github.com/biodoia/goleapifree/internal/websocket"
	"github.com/google/uuid"
)

// Example: Basic server setup
func ExampleHub_basic() {
	// Create and start hub
	hub := websocket.NewHub()
	go hub.Run()
	defer hub.Stop()

	// Create handler (requires config and db, omitted for example)
	// handler := websocket.NewHandler(hub, config, db)

	fmt.Println("WebSocket hub running")
	// Output: WebSocket hub running
}

// Example: Broadcasting events
func ExampleHandler_broadcastEvents() {
	hub := websocket.NewHub()
	go hub.Run()
	defer hub.Stop()

	// In real usage, you would have config and db
	// handler := websocket.NewHandler(hub, config, db)
	// broadcaster := websocket.NewBroadcaster(handler)

	// Example: Broadcast log event
	// broadcaster.LogInfo("Server started", "gateway")

	// Example: Broadcast provider status
	// provider := &models.Provider{
	//     ID:        uuid.New(),
	//     Name:      "OpenAI Free",
	//     Available: true,
	// }
	// broadcaster.ProviderHealthy(provider, 150)

	fmt.Println("Events broadcast")
	// Output: Events broadcast
}

// Example: TUI Client usage
func ExampleTUIClient_usage() {
	// Create TUI client with callbacks
	client := websocket.NewTUIClient("ws://localhost:8080/ws/stats", websocket.TUICallbacks{
		OnStatsUpdate: func(stats *websocket.StatsUpdateEvent) {
			fmt.Printf("Total Requests: %d\n", stats.TotalRequests)
			fmt.Printf("Success Rate: %.2f%%\n", stats.SuccessRate*100)
		},
		OnProviderUpdate: func(status *websocket.ProviderStatusEvent) {
			fmt.Printf("Provider %s: %s\n", status.ProviderName, status.Status)
		},
		OnLog: func(log *websocket.LogEvent) {
			fmt.Printf("[%s] %s\n", log.Level, log.Message)
		},
		OnError: func(err error) {
			fmt.Printf("Error: %v\n", err)
		},
	})

	ctx := context.Background()

	// Connect and subscribe
	if err := client.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}

	if err := client.Start(ctx, websocket.ChannelStats, websocket.ChannelProviders); err != nil {
		fmt.Printf("Failed to start: %v\n", err)
		return
	}

	defer client.Close()

	// Use client data
	stats := client.GetLatestStats()
	if stats != nil {
		fmt.Printf("Current stats: %d requests\n", stats.TotalRequests)
	}
}

// Example: Periodic stats updates
func ExampleBroadcaster_periodicUpdates() {
	hub := websocket.NewHub()
	go hub.Run()
	defer hub.Stop()

	// In real usage:
	// handler := websocket.NewHandler(hub, config, db)
	// broadcaster := websocket.NewBroadcaster(handler)

	// Start periodic updates every 5 seconds
	// stop := broadcaster.StartPeriodicStatsUpdates(5*time.Second, "5m")
	// defer close(stop)

	fmt.Println("Periodic updates started")
	// Output: Periodic updates started
}

// Example: Mock client for testing
func ExampleMockTUIClient_testing() {
	// Create mock client for testing
	mock := websocket.NewMockTUIClient()

	// Get initial stats
	stats := mock.GetLatestStats()
	fmt.Printf("Initial requests: %d\n", stats.TotalRequests)

	// Simulate update
	mock.SimulateUpdate()
	stats = mock.GetLatestStats()
	fmt.Printf("After update: %d\n", stats.TotalRequests)

	// Output:
	// Initial requests: 1000
	// After update: 1001
}

// Example: Creating and handling events
func ExampleNewEvent() {
	// Create a log event
	logEvent := websocket.LogEvent{
		Level:     "info",
		Message:   "Request completed successfully",
		Component: "router",
		Timestamp: time.Now(),
	}

	event, err := websocket.NewEvent(websocket.EventTypeLog, logEvent)
	if err != nil {
		fmt.Printf("Failed to create event: %v\n", err)
		return
	}

	// Serialize to JSON
	data, err := event.ToJSON()
	if err != nil {
		fmt.Printf("Failed to serialize: %v\n", err)
		return
	}

	fmt.Printf("Event type: %s\n", event.Type)
	fmt.Printf("JSON size: %d bytes\n", len(data))
}

// Example: Real-world integration in gateway
func ExampleGateway_integration() {
	// In your gateway startup:
	/*
		// Initialize WebSocket
		wsHub := websocket.NewHub()
		wsHandler := websocket.NewHandler(wsHub, config, db)
		broadcaster := websocket.NewBroadcaster(wsHandler)

		// Start hub
		go wsHub.Run()

		// Setup routes
		app.Get("/ws/logs", wsHandler.HandleLogsWebSocket)
		app.Get("/ws/stats", wsHandler.HandleStatsWebSocket)
		app.Get("/ws/providers", wsHandler.HandleProvidersWebSocket)
		app.Get("/ws/requests", wsHandler.HandleRequestsWebSocket)

		// Start periodic stats updates
		stop := broadcaster.StartPeriodicStatsUpdates(5*time.Second, "5m")

		// When handling requests:
		func handleRequest() {
			// ... process request ...

			// Broadcast request event
			requestLog := &models.RequestLog{
				ID:           uuid.New(),
				ProviderID:   providerID,
				Success:      true,
				LatencyMs:    150,
				InputTokens:  100,
				OutputTokens: 200,
			}
			broadcaster.RequestCompleted(requestLog, "OpenAI Free", "gpt-3.5-turbo")
		}

		// On provider health check:
		func checkProviderHealth(provider *models.Provider) {
			if healthy {
				broadcaster.ProviderHealthy(provider, latency)
			} else {
				broadcaster.ProviderUnhealthy(provider, "Connection timeout")
			}
		}

		// On shutdown:
		defer func() {
			close(stop)
			wsHub.Stop()
		}()
	*/

	fmt.Println("Gateway with WebSocket integration")
	// Output: Gateway with WebSocket integration
}

// Example: Subscribe and unsubscribe
func ExampleHub_subscribe() {
	hub := websocket.NewHub()
	go hub.Run()
	defer hub.Stop()

	// Create client using NewClient
	client := websocket.NewClient(hub, nil)

	// Subscribe to multiple channels
	hub.Subscribe(client, websocket.ChannelLogs)
	hub.Subscribe(client, websocket.ChannelStats)

	// Later, unsubscribe from a channel
	hub.Unsubscribe(client, websocket.ChannelLogs)

	fmt.Println("Subscription management example")
	// Output: Subscription management example
}

// Example: Broadcasting to specific channels
func ExampleHub_Broadcast() {
	hub := websocket.NewHub()
	go hub.Run()
	defer hub.Stop()

	// Broadcast log event to logs channel
	logEvent, _ := websocket.NewEvent(websocket.EventTypeLog, websocket.LogEvent{
		Level:   "error",
		Message: "Database connection failed",
	})
	hub.Broadcast(websocket.ChannelLogs, logEvent)

	// Broadcast stats to stats channel
	statsEvent, _ := websocket.NewEvent(websocket.EventTypeStatsUpdate, websocket.StatsUpdateEvent{
		TotalRequests: 1000,
		SuccessRate:   0.95,
	})
	hub.Broadcast(websocket.ChannelStats, statsEvent)

	// Broadcast provider status
	providerEvent, _ := websocket.NewEvent(websocket.EventTypeProviderStatus, websocket.ProviderStatusEvent{
		ProviderName: "OpenAI Free",
		Status:       "healthy",
		Available:    true,
	})
	hub.Broadcast(websocket.ChannelProviders, providerEvent)

	fmt.Println("Events broadcast to specific channels")
	// Output: Events broadcast to specific channels
}
