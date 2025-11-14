package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Test 1: Client Creation
// =============================================================================

func TestNew(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		aiClient := New()
		client, ok := aiClient.(*Client)
		if !ok {
			t.Fatal("expected *Client type")
		}

		if client.Provider != ProviderDeepSeek {
			t.Errorf("expected provider %v, got %v", ProviderDeepSeek, client.Provider)
		}

		if client.BaseURL != "https://api.deepseek.com/v1" {
			t.Errorf("unexpected default BaseURL: %s", client.BaseURL)
		}

		if client.Model != "deepseek-chat" {
			t.Errorf("unexpected default Model: %s", client.Model)
		}

		if client.Timeout != 120*time.Second {
			t.Errorf("expected timeout 120s, got %v", client.Timeout)
		}

		if client.MaxTokens != 2000 {
			t.Errorf("expected MaxTokens 2000, got %d", client.MaxTokens)
		}
	})

	t.Run("with AI_MAX_TOKENS environment variable", func(t *testing.T) {
		// Set environment variable
		os.Setenv("AI_MAX_TOKENS", "5000")
		defer os.Unsetenv("AI_MAX_TOKENS")

		aiClient := New()
		client, ok := aiClient.(*Client)
		if !ok {
			t.Fatal("expected *Client type")
		}

		if client.MaxTokens != 5000 {
			t.Errorf("expected MaxTokens 5000, got %d", client.MaxTokens)
		}
	})

	t.Run("with invalid AI_MAX_TOKENS", func(t *testing.T) {
		os.Setenv("AI_MAX_TOKENS", "invalid")
		defer os.Unsetenv("AI_MAX_TOKENS")

		aiClient := New()
	client, ok := aiClient.(*Client)
	if !ok {
		t.Fatal("expected *Client type")
	}

		// Should use default value
		if client.MaxTokens != 2000 {
			t.Errorf("expected default MaxTokens 2000, got %d", client.MaxTokens)
		}
	})
}

// =============================================================================
// Test 2: DeepSeek API Key Configuration
// =============================================================================

func TestSetDeepSeekAPIKey(t *testing.T) {
	t.Run("with default URL and model", func(t *testing.T) {
		aiClient := NewDeepSeekClient()
		dsClient, ok := aiClient.(*DeepSeekClient)
		if !ok {
			t.Fatal("expected *DeepSeekClient type")
		}
		dsClient.SetAPIKey("test-api-key-1234567890", "", "")

		if dsClient.Client.Provider != ProviderDeepSeek {
			t.Errorf("expected provider %v, got %v", ProviderDeepSeek, dsClient.Client.Provider)
		}

		if dsClient.Client.APIKey != "test-api-key-1234567890" {
			t.Errorf("API key not set correctly")
		}

		if dsClient.Client.BaseURL != "https://api.deepseek.com/v1" {
			t.Errorf("expected default BaseURL, got %s", dsClient.Client.BaseURL)
		}

		if dsClient.Client.Model != "deepseek-chat" {
			t.Errorf("expected default model, got %s", dsClient.Client.Model)
		}
	})

	t.Run("with custom URL and model", func(t *testing.T) {
		aiClient := NewDeepSeekClient()
		dsClient, ok := aiClient.(*DeepSeekClient)
		if !ok {
			t.Fatal("expected *DeepSeekClient type")
		}
		dsClient.SetAPIKey(
			"test-key",
			"https://custom.api.com/v1",
			"custom-model",
		)

		if dsClient.Client.BaseURL != "https://custom.api.com/v1" {
			t.Errorf("custom BaseURL not set: %s", dsClient.Client.BaseURL)
		}

		if dsClient.Client.Model != "custom-model" {
			t.Errorf("custom model not set: %s", dsClient.Client.Model)
		}
	})
}

// =============================================================================
// Test 3: Qwen API Key Configuration
// =============================================================================

func TestSetQwenAPIKey(t *testing.T) {
	t.Run("with default URL and model", func(t *testing.T) {
		aiClient := NewQwenClient()
		qwenClient, ok := aiClient.(*QwenClient)
		if !ok {
			t.Fatal("expected *QwenClient type")
		}
		qwenClient.SetAPIKey("qwen-api-key-1234567890", "", "")

		if qwenClient.Client.Provider != ProviderQwen {
			t.Errorf("expected provider %v, got %v", ProviderQwen, qwenClient.Client.Provider)
		}

		if qwenClient.Client.APIKey != "qwen-api-key-1234567890" {
			t.Errorf("API key not set correctly")
		}

		if qwenClient.Client.BaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
			t.Errorf("unexpected BaseURL: %s", qwenClient.Client.BaseURL)
		}

		if qwenClient.Client.Model != "qwen3-max" {
			t.Errorf("unexpected model: %s", qwenClient.Client.Model)
		}
	})

	t.Run("with custom URL and model", func(t *testing.T) {
		aiClient := NewQwenClient()
		qwenClient, ok := aiClient.(*QwenClient)
		if !ok {
			t.Fatal("expected *QwenClient type")
		}
		qwenClient.SetAPIKey(
			"qwen-key",
			"https://custom-qwen.com/v1",
			"qwen-custom",
		)

		if qwenClient.Client.BaseURL != "https://custom-qwen.com/v1" {
			t.Errorf("custom BaseURL not set: %s", qwenClient.Client.BaseURL)
		}

		if qwenClient.Client.Model != "qwen-custom" {
			t.Errorf("custom model not set: %s", qwenClient.Client.Model)
		}
	})
}

// =============================================================================
// Test 4: Custom API Configuration
// =============================================================================

func TestSetCustomAPI(t *testing.T) {
	t.Run("without # suffix", func(t *testing.T) {
		aiClient := New()
		client, ok := aiClient.(*Client)
		if !ok {
			t.Fatal("expected *Client type")
		}
		client.SetAPIKey(
			"custom-key-1234567890",
			"https://custom-ai.com/v1",
			"custom-model-v1",
		)

		if client.Provider != ProviderCustom {
			t.Errorf("expected provider %v, got %v", ProviderCustom, client.Provider)
		}

		if client.APIKey != "custom-key-1234567890" {
			t.Errorf("API key not set")
		}

		if client.BaseURL != "https://custom-ai.com/v1" {
			t.Errorf("unexpected BaseURL: %s", client.BaseURL)
		}

		if client.Model != "custom-model-v1" {
			t.Errorf("unexpected model: %s", client.Model)
		}

		if client.UseFullURL {
			t.Errorf("UseFullURL should be false without # suffix")
		}
	})

	t.Run("with # suffix for full URL", func(t *testing.T) {
		aiClient := New()
		client, ok := aiClient.(*Client)
		if !ok {
			t.Fatal("expected *Client type")
		}
		client.SetAPIKey(
			"custom-key",
			"https://custom-ai.com/v1/chat#",
			"custom-model",
		)

		if client.BaseURL != "https://custom-ai.com/v1/chat" {
			t.Errorf("# suffix should be trimmed, got: %s", client.BaseURL)
		}

		if !client.UseFullURL {
			t.Errorf("UseFullURL should be true with # suffix")
		}
	})
}

// =============================================================================
// Test 5: AI API Call (Success)
// =============================================================================

func TestCallWithMessages_Success(t *testing.T) {
	// Create mock AI API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify Authorization header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			t.Errorf("missing or invalid Authorization header: %s", authHeader)
		}

		// Verify Content-Type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		// Verify request body structure
		if reqBody["model"] == nil {
			t.Errorf("missing 'model' in request")
		}

		messages, ok := reqBody["messages"].([]interface{})
		if !ok || len(messages) == 0 {
			t.Errorf("invalid or empty 'messages' in request")
		}

		// Return mock AI response
		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `[{"symbol":"BTCUSDT","action":"hold","reason":"test"}]`,
					},
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	// Create client with mock server
	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key-1234567890",
		BaseURL:   mockServer.URL,
		Model:     "test-model",
		Timeout:   5 * time.Second,
		MaxTokens: 2000,
	}

	// Test AI call
	result, err := client.CallWithMessages("system prompt", "user prompt")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "BTCUSDT") {
		t.Errorf("expected response to contain BTCUSDT, got: %s", result)
	}

	if !strings.Contains(result, "hold") {
		t.Errorf("expected response to contain 'hold', got: %s", result)
	}
}

// =============================================================================
// Test 6: AI API Call (Missing API Key)
// =============================================================================

func TestCallWithMessages_MissingAPIKey(t *testing.T) {
	aiClient := New()
	client, ok := aiClient.(*Client)
	if !ok {
		t.Fatal("expected *Client type")
	}
	// Don't set API key

	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Fatal("expected error for missing API key")
	}

	if !strings.Contains(err.Error(), "API密钥未设置") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// =============================================================================
// Test 7: AI API Call (Timeout)
// =============================================================================

func TestCallWithMessages_Timeout(t *testing.T) {
	// Create slow mock server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "test-model",
		Timeout:   1 * time.Second, // Short timeout
		MaxTokens: 2000,
	}

	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Check if it's a timeout error
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("expected timeout error, got: %v", err)
	}
}

// =============================================================================
// Test 8: AI API Call (HTTP Error Status)
// =============================================================================

func TestCallWithMessages_HTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"401 Unauthorized", http.StatusUnauthorized},
		{"429 Too Many Requests", http.StatusTooManyRequests},
		{"500 Internal Server Error", http.StatusInternalServerError},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error":"test error"}`))
			}))
			defer mockServer.Close()

			client := &Client{
				Provider:  ProviderDeepSeek,
				APIKey:    "test-key",
				BaseURL:   mockServer.URL,
				Model:     "test-model",
				Timeout:   5 * time.Second,
				MaxTokens: 2000,
			}

			_, err := client.CallWithMessages("system", "user")

			if err == nil {
				t.Fatalf("expected error for status %d", tt.statusCode)
			}
		})
	}
}

// =============================================================================
// Test 9: AI API Call (Invalid JSON Response)
// =============================================================================

func TestCallWithMessages_InvalidJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer mockServer.Close()

	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "test-model",
		Timeout:   5 * time.Second,
		MaxTokens: 2000,
	}

	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// =============================================================================
// Test 10: AI API Call (Retry Logic)
// =============================================================================

func TestCallWithMessages_RetrySuccess(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Fail first 2 attempts with connection closure (EOF error)
		// This simulates a retryable network error
		if callCount < 3 {
			// Close connection without writing response to trigger EOF
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
			// Fallback if hijacking not supported
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Success response on 3rd attempt
		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `[{"symbol":"BTCUSDT","action":"hold"}]`,
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "test-model",
		Timeout:   5 * time.Second,
		MaxTokens: 2000,
	}

	// Should succeed after retries (3rd attempt)
	result, err := client.CallWithMessages("system", "user")

	if err != nil {
		t.Fatalf("expected success after retry, got error: %v", err)
	}

	if callCount != 3 {
		t.Errorf("expected 3 calls (initial + 2 retries), got %d", callCount)
	}

	if !strings.Contains(result, "BTCUSDT") {
		t.Errorf("unexpected result: %s", result)
	}
}

// =============================================================================
// Test 11: AI API Call (Non-Retryable HTTP Error)
// =============================================================================

func TestCallWithMessages_NonRetryableError(t *testing.T) {
	callCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// 503 is not in the retryable list, should fail immediately
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer mockServer.Close()

	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "test-model",
		Timeout:   5 * time.Second,
		MaxTokens: 2000,
	}

	_, err := client.CallWithMessages("system", "user")

	if err == nil {
		t.Fatal("expected error for 503 status")
	}

	// HTTP error codes are not retryable by default
	// So should only call once
	if callCount != 1 {
		t.Errorf("expected 1 call (non-retryable error), got %d", callCount)
	}
}

// =============================================================================
// Test 12: Request Body Validation
// =============================================================================

func TestCallWithMessages_RequestBodyValidation(t *testing.T) {
	var capturedRequest map[string]interface{}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture request body
		json.NewDecoder(r.Body).Decode(&capturedRequest)

		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "test response",
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "test-model-123",
		Timeout:   5 * time.Second,
		MaxTokens: 3000,
	}

	client.CallWithMessages("test system prompt", "test user prompt")

	// Verify model
	if capturedRequest["model"] != "test-model-123" {
		t.Errorf("expected model 'test-model-123', got %v", capturedRequest["model"])
	}

	// Verify max_tokens
	if maxTokens, ok := capturedRequest["max_tokens"].(float64); !ok || int(maxTokens) != 3000 {
		t.Errorf("expected max_tokens 3000, got %v", capturedRequest["max_tokens"])
	}

	// Verify temperature
	if temp, ok := capturedRequest["temperature"].(float64); !ok || temp != 0.5 {
		t.Errorf("expected temperature 0.5, got %v", capturedRequest["temperature"])
	}

	// Verify messages structure
	messages, ok := capturedRequest["messages"].([]interface{})
	if !ok {
		t.Fatal("messages should be an array")
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(messages))
	}

	// Verify system message
	systemMsg, ok := messages[0].(map[string]interface{})
	if !ok {
		t.Fatal("first message should be a map")
	}

	if systemMsg["role"] != "system" {
		t.Errorf("expected first message role 'system', got %v", systemMsg["role"])
	}

	if systemMsg["content"] != "test system prompt" {
		t.Errorf("unexpected system message content: %v", systemMsg["content"])
	}

	// Verify user message
	userMsg, ok := messages[1].(map[string]interface{})
	if !ok {
		t.Fatal("second message should be a map")
	}

	if userMsg["role"] != "user" {
		t.Errorf("expected second message role 'user', got %v", userMsg["role"])
	}

	if userMsg["content"] != "test user prompt" {
		t.Errorf("unexpected user message content: %v", userMsg["content"])
	}
}

// =============================================================================
// Test 13: Empty Prompts Handling
// =============================================================================

func TestCallWithMessages_EmptyPrompts(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "response",
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "test-model",
		Timeout:   5 * time.Second,
		MaxTokens: 2000,
	}

	// Test with empty system prompt (should work)
	_, err := client.CallWithMessages("", "user prompt")
	if err != nil {
		t.Errorf("should handle empty system prompt, got error: %v", err)
	}

	// Test with empty user prompt (should work, API will handle)
	_, err = client.CallWithMessages("system prompt", "")
	if err != nil {
		t.Errorf("should handle empty user prompt, got error: %v", err)
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkCallWithMessages(b *testing.B) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": `[{"symbol":"BTCUSDT","action":"hold"}]`,
					},
				},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()

	client := &Client{
		Provider:  ProviderDeepSeek,
		APIKey:    "test-key",
		BaseURL:   mockServer.URL,
		Model:     "test-model",
		Timeout:   5 * time.Second,
		MaxTokens: 2000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.CallWithMessages("system", "user")
	}
}
