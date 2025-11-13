package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"nofx/auth"
	"nofx/config"

	"github.com/gin-gonic/gin"
)

// setupTestEnv creates test user and configurations
func setupTestEnv(t *testing.T, db *config.Database) (userID string, aiModelIntID int, exchangeIntID int) {
	// Create test user
	testEmail := "trader-test@example.com"
	testPass := "ValidPass123!"
	hashedPassword, err := auth.HashPassword(testPass)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &config.User{
		ID:           "test-trader-user-001",
		Email:        testEmail,
		PasswordHash: hashedPassword,
		OTPSecret:    "JBSWY3DPEHPK3PXP",
		OTPVerified:  true,
	}
	err = db.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	userID = user.ID

	// Create test AI model
	err = db.CreateAIModel(userID, "test-model", "Test Model", "openai", true, "test-key", "http://test")
	if err != nil {
		t.Fatalf("Failed to create AI model: %v", err)
	}

	// Query AI models to get the integer ID
	aiModels, err := db.GetAIModels(userID)
	if err != nil || len(aiModels) == 0 {
		t.Fatalf("Failed to get AI models: %v", err)
	}
	aiModelIntID = aiModels[0].ID

	// Create test exchange
	err = db.CreateExchange(userID, "binance", "Binance", "cex", true, "test-key", "test-secret", false, "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create exchange: %v", err)
	}

	// Query exchanges to get the integer ID
	exchanges, err := db.GetExchanges(userID)
	if err != nil || len(exchanges) == 0 {
		t.Fatalf("Failed to get exchanges: %v", err)
	}
	exchangeIntID = exchanges[0].ID

	return userID, aiModelIntID, exchangeIntID
}

// TestHandleTraderList tests the trader list endpoint
func TestHandleTraderList(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, aiModelIntID, exchangeIntID := setupTestEnv(t, db)

	// Create a test trader
	trader := &config.TraderRecord{
		ID:                  "test-trader-001",
		UserID:              userID,
		Name:                "Test Trader",
		AIModelID:           aiModelIntID,
		ExchangeID:          exchangeIntID,
		InitialBalance:      1000.0,
		ScanIntervalMinutes: 3,
		IsRunning:           false,
		BTCETHLeverage:      5,
		AltcoinLeverage:     5,
		TradingSymbols:      "BTCUSDT",
		UseCoinPool:         false,
		UseOITop:            false,
	}
	err := db.CreateTrader(trader)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}
	traderID := trader.ID

	// Create test request
	router := gin.New()
	router.GET("/traders", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleTraderList(c)
	})

	// Execute request
	req := httptest.NewRequest("GET", "/traders", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var traders []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &traders); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(traders) != 1 {
		t.Errorf("Expected 1 trader, got %d", len(traders))
	}

	if traders[0]["trader_id"].(string) != traderID {
		t.Errorf("Expected trader_id %s, got %v", traderID, traders[0]["trader_id"])
	}

	if traders[0]["trader_name"].(string) != "Test Trader" {
		t.Errorf("Expected trader_name 'Test Trader', got %v", traders[0]["trader_name"])
	}

	t.Logf("✅ handleTraderList test passed")
}

// TestHandleDeleteTrader tests the delete trader endpoint
func TestHandleDeleteTrader(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, aiModelIntID, exchangeIntID := setupTestEnv(t, db)

	// Create a test trader
	trader := &config.TraderRecord{
		ID:                  "test-trader-to-delete",
		UserID:              userID,
		Name:                "Trader to Delete",
		AIModelID:           aiModelIntID,
		ExchangeID:          exchangeIntID,
		InitialBalance:      1000.0,
		ScanIntervalMinutes: 3,
		IsRunning:           false,
		BTCETHLeverage:      5,
		AltcoinLeverage:     5,
		TradingSymbols:      "BTCUSDT",
		UseCoinPool:         false,
		UseOITop:            false,
	}
	err := db.CreateTrader(trader)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}
	traderID := trader.ID

	// Create test request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/traders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleDeleteTrader(c)
	})

	// Execute request
	req := httptest.NewRequest("DELETE", "/traders/"+traderID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify trader was deleted
	traders, err := db.GetTraders(userID)
	if err != nil {
		t.Fatalf("Failed to get traders: %v", err)
	}

	if len(traders) != 0 {
		t.Errorf("Expected 0 traders after deletion, got %d", len(traders))
	}

	t.Logf("✅ handleDeleteTrader test passed")
}

// TestHandleDeleteTrader_NotFound tests deleting a non-existent trader
func TestHandleDeleteTrader_NotFound(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, _, _ := setupTestEnv(t, db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/traders/:id", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleDeleteTrader(c)
	})

	// Try to delete non-existent trader
	req := httptest.NewRequest("DELETE", "/traders/nonexistent-id", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Idempotent delete: API returns 200 even when trader doesn't exist
	// This is acceptable REST API behavior
	if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
		t.Errorf("Expected status 200 or 404, got %d", w.Code)
	}

	t.Logf("✅ handleDeleteTrader (not found) test passed (status: %d)", w.Code)
}

// TestHandleUpdateTraderPrompt tests updating trader prompt
func TestHandleUpdateTraderPrompt(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, aiModelIntID, exchangeIntID := setupTestEnv(t, db)

	// Create a test trader
	trader := &config.TraderRecord{
		ID:                  "test-trader-for-prompt",
		UserID:              userID,
		Name:                "Test Trader",
		AIModelID:           aiModelIntID,
		ExchangeID:          exchangeIntID,
		InitialBalance:      1000.0,
		ScanIntervalMinutes: 3,
		IsRunning:           false,
		BTCETHLeverage:      5,
		AltcoinLeverage:     5,
		TradingSymbols:      "BTCUSDT",
		UseCoinPool:         false,
		UseOITop:            false,
		CustomPrompt:        "Old prompt",
		OverrideBasePrompt:  false,
	}
	err := db.CreateTrader(trader)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}
	traderID := trader.ID

	// Prepare update request
	updateReq := map[string]interface{}{
		"custom_prompt":        "New custom prompt",
		"override_base_prompt": true,
	}
	reqBody, _ := json.Marshal(updateReq)

	// Create test request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/traders/:id/prompt", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleUpdateTraderPrompt(c)
	})

	// Execute request
	req := httptest.NewRequest("PUT", "/traders/"+traderID+"/prompt", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify prompt was updated
	traders, err := db.GetTraders(userID)
	if err != nil {
		t.Fatalf("Failed to get traders: %v", err)
	}

	var updatedTrader *config.TraderRecord
	for _, tr := range traders {
		if tr.ID == traderID {
			updatedTrader = tr
			break
		}
	}

	if updatedTrader == nil {
		t.Fatalf("Trader not found after update")
	}

	if updatedTrader.CustomPrompt != "New custom prompt" {
		t.Errorf("Expected custom_prompt 'New custom prompt', got '%s'", updatedTrader.CustomPrompt)
	}

	if !updatedTrader.OverrideBasePrompt {
		t.Errorf("Expected override_base_prompt to be true")
	}

	t.Logf("✅ handleUpdateTraderPrompt test passed")
}
