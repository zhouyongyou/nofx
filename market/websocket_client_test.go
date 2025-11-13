package market

import (
	"encoding/json"
	"testing"
	"time"
)

// TestNewWSClient tests creating a new WebSocket client
func TestNewWSClient(t *testing.T) {
	client := NewWSClient()

	if client == nil {
		t.Fatal("NewWSClient returned nil")
	}

	if client.subscribers == nil {
		t.Error("subscribers map should be initialized")
	}

	if client.done == nil {
		t.Error("done channel should be initialized")
	}

	if !client.reconnect {
		t.Error("reconnect should be true by default")
	}
}

// TestWSMessageParsing tests parsing WebSocket messages
func TestWSMessageParsing(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "kline message",
			message:  `{"stream":"btcusdt@kline_1m","data":{"e":"kline","E":1234567890,"s":"BTCUSDT","k":{"t":1234567800,"T":1234567859,"s":"BTCUSDT","i":"1m","f":1234567,"L":1234589,"o":"50000.00","c":"50050.00","h":"50100.00","l":"49900.00","v":"1234.56","n":100,"x":true,"q":"61728000.00","V":"600.00","Q":"30000000.00"}}}`,
			expected: "btcusdt@kline_1m",
		},
		{
			name:     "ticker message",
			message:  `{"stream":"btcusdt@ticker","data":{"e":"24hrTicker","E":1234567890,"s":"BTCUSDT","p":"100.00","P":"0.20","w":"50000.00","c":"50100.00","Q":"1.5","o":"50000.00","h":"51000.00","l":"49000.00","v":"100000.00","q":"5000000000.00","O":1234481490,"C":1234567890,"F":1234567,"L":1234589,"n":1000}}`,
			expected: "btcusdt@ticker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wsMsg WSMessage
			err := json.Unmarshal([]byte(tt.message), &wsMsg)
			if err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			if wsMsg.Stream != tt.expected {
				t.Errorf("Expected stream '%s', got '%s'", tt.expected, wsMsg.Stream)
			}

			if len(wsMsg.Data) == 0 {
				t.Error("Data field should not be empty")
			}
		})
	}
}

// TestKlineWSDataParsing tests parsing kline WebSocket data
func TestKlineWSDataParsing(t *testing.T) {
	klineJSON := `{
		"e": "kline",
		"E": 1234567890,
		"s": "BTCUSDT",
		"k": {
			"t": 1234567800,
			"T": 1234567859,
			"s": "BTCUSDT",
			"i": "1m",
			"f": 1234567,
			"L": 1234589,
			"o": "50000.00",
			"c": "50050.00",
			"h": "50100.00",
			"l": "49900.00",
			"v": "1234.56",
			"n": 100,
			"x": true,
			"q": "61728000.00",
			"V": "600.00",
			"Q": "30000000.00"
		}
	}`

	var klineData KlineWSData
	err := json.Unmarshal([]byte(klineJSON), &klineData)
	if err != nil {
		t.Fatalf("Failed to unmarshal kline data: %v", err)
	}

	// Verify basic fields
	if klineData.EventType != "kline" {
		t.Errorf("Expected event type 'kline', got '%s'", klineData.EventType)
	}

	if klineData.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol 'BTCUSDT', got '%s'", klineData.Symbol)
	}

	if klineData.EventTime != 1234567890 {
		t.Errorf("Expected event time 1234567890, got %d", klineData.EventTime)
	}

	// Verify kline fields
	if klineData.Kline.Symbol != "BTCUSDT" {
		t.Errorf("Expected kline symbol 'BTCUSDT', got '%s'", klineData.Kline.Symbol)
	}

	if klineData.Kline.Interval != "1m" {
		t.Errorf("Expected interval '1m', got '%s'", klineData.Kline.Interval)
	}

	if klineData.Kline.OpenPrice != "50000.00" {
		t.Errorf("Expected open price '50000.00', got '%s'", klineData.Kline.OpenPrice)
	}

	if klineData.Kline.ClosePrice != "50050.00" {
		t.Errorf("Expected close price '50050.00', got '%s'", klineData.Kline.ClosePrice)
	}

	if klineData.Kline.HighPrice != "50100.00" {
		t.Errorf("Expected high price '50100.00', got '%s'", klineData.Kline.HighPrice)
	}

	if klineData.Kline.LowPrice != "49900.00" {
		t.Errorf("Expected low price '49900.00', got '%s'", klineData.Kline.LowPrice)
	}

	if klineData.Kline.Volume != "1234.56" {
		t.Errorf("Expected volume '1234.56', got '%s'", klineData.Kline.Volume)
	}

	if klineData.Kline.NumberOfTrades != 100 {
		t.Errorf("Expected 100 trades, got %d", klineData.Kline.NumberOfTrades)
	}

	if !klineData.Kline.IsFinal {
		t.Error("Expected kline to be final")
	}
}

// TestTickerWSDataParsing tests parsing ticker WebSocket data
func TestTickerWSDataParsing(t *testing.T) {
	tickerJSON := `{
		"e": "24hrTicker",
		"E": 1234567890,
		"s": "BTCUSDT",
		"p": "100.00",
		"P": "0.20",
		"w": "50000.00",
		"c": "50100.00",
		"Q": "1.5",
		"o": "50000.00",
		"h": "51000.00",
		"l": "49000.00",
		"v": "100000.00",
		"q": "5000000000.00",
		"O": 1234481490,
		"C": 1234567890,
		"F": 1234567,
		"L": 1234589,
		"n": 1000
	}`

	var tickerData TickerWSData
	err := json.Unmarshal([]byte(tickerJSON), &tickerData)
	if err != nil {
		t.Fatalf("Failed to unmarshal ticker data: %v", err)
	}

	// Verify basic fields
	if tickerData.EventType != "24hrTicker" {
		t.Errorf("Expected event type '24hrTicker', got '%s'", tickerData.EventType)
	}

	if tickerData.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol 'BTCUSDT', got '%s'", tickerData.Symbol)
	}

	if tickerData.PriceChange != "100.00" {
		t.Errorf("Expected price change '100.00', got '%s'", tickerData.PriceChange)
	}

	if tickerData.PriceChangePercent != "0.20" {
		t.Errorf("Expected price change percent '0.20', got '%s'", tickerData.PriceChangePercent)
	}

	if tickerData.LastPrice != "50100.00" {
		t.Errorf("Expected last price '50100.00', got '%s'", tickerData.LastPrice)
	}

	if tickerData.OpenPrice != "50000.00" {
		t.Errorf("Expected open price '50000.00', got '%s'", tickerData.OpenPrice)
	}

	if tickerData.HighPrice != "51000.00" {
		t.Errorf("Expected high price '51000.00', got '%s'", tickerData.HighPrice)
	}

	if tickerData.LowPrice != "49000.00" {
		t.Errorf("Expected low price '49000.00', got '%s'", tickerData.LowPrice)
	}

	if tickerData.Volume != "100000.00" {
		t.Errorf("Expected volume '100000.00', got '%s'", tickerData.Volume)
	}

	if tickerData.Count != 1000 {
		t.Errorf("Expected count 1000, got %d", tickerData.Count)
	}
}

// TestAddSubscriber tests adding subscribers
func TestAddSubscriber(t *testing.T) {
	client := NewWSClient()

	stream := "btcusdt@kline_1m"
	bufferSize := 10

	ch := client.AddSubscriber(stream, bufferSize)
	if ch == nil {
		t.Fatal("AddSubscriber returned nil channel")
	}

	// Verify subscriber was added
	client.mu.RLock()
	_, exists := client.subscribers[stream]
	client.mu.RUnlock()

	if !exists {
		t.Error("Subscriber was not added to subscribers map")
	}

	// Verify channel buffer size
	if cap(ch) != bufferSize {
		t.Errorf("Expected channel buffer size %d, got %d", bufferSize, cap(ch))
	}
}

// TestRemoveSubscriber tests removing subscribers
func TestRemoveSubscriber(t *testing.T) {
	client := NewWSClient()

	stream := "btcusdt@ticker"
	client.AddSubscriber(stream, 10)

	// Verify subscriber exists
	client.mu.RLock()
	_, exists := client.subscribers[stream]
	client.mu.RUnlock()

	if !exists {
		t.Fatal("Subscriber was not added")
	}

	// Remove subscriber
	client.RemoveSubscriber(stream)

	// Verify subscriber was removed
	client.mu.RLock()
	_, exists = client.subscribers[stream]
	client.mu.RUnlock()

	if exists {
		t.Error("Subscriber was not removed")
	}
}

// TestMultipleSubscribers tests managing multiple subscribers
func TestMultipleSubscribers(t *testing.T) {
	client := NewWSClient()

	streams := []string{
		"btcusdt@kline_1m",
		"ethusdt@kline_1m",
		"btcusdt@ticker",
		"ethusdt@ticker",
	}

	channels := make(map[string]<-chan []byte)

	// Add multiple subscribers
	for _, stream := range streams {
		ch := client.AddSubscriber(stream, 5)
		channels[stream] = ch
	}

	// Verify all subscribers were added
	client.mu.RLock()
	subscriberCount := len(client.subscribers)
	client.mu.RUnlock()

	if subscriberCount != len(streams) {
		t.Errorf("Expected %d subscribers, got %d", len(streams), subscriberCount)
	}

	// Remove one subscriber
	client.RemoveSubscriber(streams[0])

	// Verify correct count after removal
	client.mu.RLock()
	subscriberCount = len(client.subscribers)
	client.mu.RUnlock()

	if subscriberCount != len(streams)-1 {
		t.Errorf("Expected %d subscribers after removal, got %d", len(streams)-1, subscriberCount)
	}
}

// TestHandleMessage tests message routing to subscribers
func TestHandleMessage(t *testing.T) {
	client := NewWSClient()

	stream := "btcusdt@kline_1m"
	ch := client.AddSubscriber(stream, 10)

	// Create test message
	message := `{"stream":"btcusdt@kline_1m","data":{"e":"kline","s":"BTCUSDT"}}`

	// Handle message in goroutine to avoid blocking
	go client.handleMessage([]byte(message))

	// Wait for message to be routed
	select {
	case data := <-ch:
		var klineData map[string]interface{}
		err := json.Unmarshal(data, &klineData)
		if err != nil {
			t.Fatalf("Failed to unmarshal routed data: %v", err)
		}

		if klineData["e"] != "kline" {
			t.Errorf("Expected event type 'kline', got '%v'", klineData["e"])
		}

		if klineData["s"] != "BTCUSDT" {
			t.Errorf("Expected symbol 'BTCUSDT', got '%v'", klineData["s"])
		}

	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for message to be routed")
	}
}

// TestHandleMessage_NoSubscriber tests handling messages with no subscriber
func TestHandleMessage_NoSubscriber(t *testing.T) {
	client := NewWSClient()

	// Create message for non-existent subscriber
	message := `{"stream":"nonexistent@kline_1m","data":{"e":"kline"}}`

	// This should not panic
	client.handleMessage([]byte(message))
}

// TestHandleMessage_InvalidJSON tests handling invalid JSON messages
func TestHandleMessage_InvalidJSON(t *testing.T) {
	client := NewWSClient()

	// Invalid JSON message
	message := `{invalid json`

	// This should not panic
	client.handleMessage([]byte(message))
}

// TestClose tests closing the WebSocket client
func TestClose(t *testing.T) {
	client := NewWSClient()

	// Add some subscribers
	stream1 := "btcusdt@kline_1m"
	stream2 := "ethusdt@ticker"
	client.AddSubscriber(stream1, 5)
	client.AddSubscriber(stream2, 5)

	// Close client
	client.Close()

	// Verify reconnect is disabled
	if client.reconnect {
		t.Error("reconnect should be false after close")
	}

	// Verify done channel is closed
	select {
	case <-client.done:
		// Expected - channel is closed
	default:
		t.Error("done channel should be closed")
	}

	// Verify connection is nil
	client.mu.RLock()
	conn := client.conn
	client.mu.RUnlock()

	if conn != nil {
		t.Error("connection should be nil after close")
	}

	// Verify subscribers are cleared
	client.mu.RLock()
	subscriberCount := len(client.subscribers)
	client.mu.RUnlock()

	if subscriberCount != 0 {
		t.Errorf("Expected 0 subscribers after close, got %d", subscriberCount)
	}
}

// TestConcurrentSubscriberAccess tests concurrent access to subscribers
func TestConcurrentSubscriberAccess(t *testing.T) {
	client := NewWSClient()

	// Concurrently add and remove subscribers
	done := make(chan bool)

	// Goroutine 1: Add subscribers
	go func() {
		for i := 0; i < 50; i++ {
			stream := "btcusdt@kline_1m"
			client.AddSubscriber(stream, 5)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 2: Remove subscribers
	go func() {
		for i := 0; i < 50; i++ {
			stream := "btcusdt@kline_1m"
			client.RemoveSubscriber(stream)
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Goroutine 3: Read subscriber count
	go func() {
		for i := 0; i < 50; i++ {
			client.mu.RLock()
			_ = len(client.subscribers)
			client.mu.RUnlock()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done
}

// TestKlineDataTypes tests that kline data has correct types
func TestKlineDataTypes(t *testing.T) {
	klineJSON := `{
		"e": "kline",
		"E": 1234567890,
		"s": "BTCUSDT",
		"k": {
			"t": 1234567800,
			"T": 1234567859,
			"s": "BTCUSDT",
			"i": "1m",
			"f": 1234567,
			"L": 1234589,
			"o": "50000.00",
			"c": "50050.00",
			"h": "50100.00",
			"l": "49900.00",
			"v": "1234.56",
			"n": 100,
			"x": true,
			"q": "61728000.00",
			"V": "600.00",
			"Q": "30000000.00"
		}
	}`

	var klineData KlineWSData
	err := json.Unmarshal([]byte(klineJSON), &klineData)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify types (this tests the struct definition)
	if klineData.Kline.StartTime == 0 {
		t.Error("StartTime should not be zero")
	}

	if klineData.Kline.IsFinal != true {
		t.Error("IsFinal should be true")
	}

	if klineData.Kline.NumberOfTrades == 0 {
		t.Error("NumberOfTrades should not be zero")
	}
}
