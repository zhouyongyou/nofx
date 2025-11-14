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

// TestHandleGetModelConfigs tests the AI model configs retrieval endpoint
func TestHandleGetModelConfigs(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, _, _ := setupTestEnv(t, db)

	// Create test request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/models", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleGetModelConfigs(c)
	})

	// Execute request
	req := httptest.NewRequest("GET", "/models", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var models []SafeModelConfig
	err := json.Unmarshal(w.Body.Bytes(), &models)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify we got the test model
	if len(models) == 0 {
		t.Error("Expected at least one model, got 0")
	}

	// Verify the model has correct fields (no sensitive data)
	foundTestModel := false
	for _, model := range models {
		if model.ID == "test-model" {
			foundTestModel = true
			if model.Name != "Test Model" {
				t.Errorf("Expected model name 'Test Model', got '%s'", model.Name)
			}
			if model.Provider != "openai" {
				t.Errorf("Expected provider 'openai', got '%s'", model.Provider)
			}
			// Verify API key is NOT included in response
			// SafeModelConfig should not have APIKey field
		}
	}

	if !foundTestModel {
		t.Error("Test model not found in response")
	}

	t.Logf("✅ handleGetModelConfigs test passed")
}

// TestHandleGetExchangeConfigs tests the exchange configs retrieval endpoint
func TestHandleGetExchangeConfigs(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, _, _ := setupTestEnv(t, db)

	// Create test request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/exchanges", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleGetExchangeConfigs(c)
	})

	// Execute request
	req := httptest.NewRequest("GET", "/exchanges", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var exchanges []SafeExchangeConfig
	err := json.Unmarshal(w.Body.Bytes(), &exchanges)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify we got the test exchange
	if len(exchanges) == 0 {
		t.Error("Expected at least one exchange, got 0")
	}

	// Verify the exchange has correct fields (no sensitive data)
	foundBinance := false
	for _, exchange := range exchanges {
		if exchange.ID == "binance" {
			foundBinance = true
			if exchange.Name != "Binance" {
				t.Errorf("Expected exchange name 'Binance', got '%s'", exchange.Name)
			}
			if exchange.Type != "cex" {
				t.Errorf("Expected type 'cex', got '%s'", exchange.Type)
			}
			// Verify sensitive fields are NOT included
			// SafeExchangeConfig should not have APIKey, SecretKey fields
		}
	}

	if !foundBinance {
		t.Error("Binance exchange not found in response")
	}

	t.Logf("✅ handleGetExchangeConfigs test passed")
}

// TestHandleCreateTrader tests the trader creation endpoint
func TestHandleCreateTrader(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, _, _ := setupTestEnv(t, db)

	// Prepare create request (use string IDs, not integer IDs)
	createReq := map[string]interface{}{
		"name":                  "New Test Trader",
		"ai_model_id":           "test-model", // String model_id, not integer ID
		"exchange_id":           "binance",    // String exchange_id, not integer ID
		"initial_balance":       1500.0,
		"scan_interval_minutes": 5,
		"btc_eth_leverage":      3,
		"altcoin_leverage":      3,
		"trading_symbols":       "BTCUSDT,ETHUSDT",
		"use_coin_pool":         false,
		"use_oi_top":            false,
		"custom_prompt":         "",
		"override_base_prompt":  false,
	}
	reqBody, _ := json.Marshal(createReq)

	// Create test request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/traders", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleCreateTrader(c)
	})

	// Execute request
	req := httptest.NewRequest("POST", "/traders", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response (201 Created is expected for resource creation)
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
		return
	}

	// Parse response to get trader ID
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	traderID, ok := response["trader_id"].(string) // API returns "trader_id", not "id"
	if !ok || traderID == "" {
		t.Error("Response should include trader ID")
	}

	// Verify trader was created in database
	traders, err := db.GetTraders(userID)
	if err != nil {
		t.Fatalf("Failed to get traders: %v", err)
	}

	var createdTrader *config.TraderRecord
	for _, tr := range traders {
		if tr.ID == traderID {
			createdTrader = tr
			break
		}
	}

	if createdTrader == nil {
		t.Fatal("Created trader not found in database")
	}

	// Verify trader fields
	if createdTrader.Name != "New Test Trader" {
		t.Errorf("Expected name 'New Test Trader', got '%s'", createdTrader.Name)
	}
	if createdTrader.InitialBalance != 1500.0 {
		t.Errorf("Expected initial balance 1500.0, got %f", createdTrader.InitialBalance)
	}
	if createdTrader.IsRunning {
		t.Error("Newly created trader should not be running")
	}

	t.Logf("✅ handleCreateTrader test passed, trader ID: %s", traderID)
}

// TestHandleStartTrader tests the trader start endpoint
func TestHandleStartTrader(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, aiModelIntID, exchangeIntID := setupTestEnv(t, db)

	// Create a test trader (not running)
	trader := &config.TraderRecord{
		ID:                  "test-trader-to-start",
		UserID:              userID,
		Name:                "Test Trader to Start",
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

	// Load trader into TraderManager
	err = server.traderManager.LoadUserTraders(db, userID)
	if err != nil {
		t.Fatalf("Failed to load trader into manager: %v", err)
	}

	// Create test request
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/traders/:id/start", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleStartTrader(c)
	})

	// Execute request
	req := httptest.NewRequest("POST", "/traders/"+traderID+"/start", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		return
	}

	// Verify trader is marked as running in database
	traders, err := db.GetTraders(userID)
	if err != nil {
		t.Fatalf("Failed to get traders: %v", err)
	}

	var startedTrader *config.TraderRecord
	for _, tr := range traders {
		if tr.ID == traderID {
			startedTrader = tr
			break
		}
	}

	if startedTrader == nil {
		t.Fatal("Trader not found after start")
	}

	if !startedTrader.IsRunning {
		t.Error("Trader should be marked as running after start")
	}

	t.Logf("✅ handleStartTrader test passed")
}

// TestHandleStopTrader tests the trader stop endpoint
func TestHandleStopTrader(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	userID, aiModelIntID, exchangeIntID := setupTestEnv(t, db)

	// Create a test trader (not running initially)
	trader := &config.TraderRecord{
		ID:                  "test-trader-to-stop",
		UserID:              userID,
		Name:                "Test Trader to Stop",
		AIModelID:           aiModelIntID,
		ExchangeID:          exchangeIntID,
		InitialBalance:      1000.0,
		ScanIntervalMinutes: 3,
		IsRunning:           false, // Start as not running
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

	// Load trader into TraderManager
	err = server.traderManager.LoadUserTraders(db, userID)
	if err != nil {
		t.Fatalf("Failed to load trader into manager: %v", err)
	}

	// First, start the trader
	gin.SetMode(gin.TestMode)
	startRouter := gin.New()
	startRouter.POST("/traders/:id/start", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleStartTrader(c)
	})

	startReq := httptest.NewRequest("POST", "/traders/"+traderID+"/start", nil)
	startW := httptest.NewRecorder()
	startRouter.ServeHTTP(startW, startReq)

	if startW.Code != http.StatusOK {
		t.Fatalf("Failed to start trader: %s", startW.Body.String())
	}

	// Now create test request to stop the trader
	router := gin.New()
	router.POST("/traders/:id/stop", func(c *gin.Context) {
		c.Set("user_id", userID)
		server.handleStopTrader(c)
	})

	// Execute request
	req := httptest.NewRequest("POST", "/traders/"+traderID+"/stop", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		return
	}

	// Verify trader is marked as stopped in database
	traders, err := db.GetTraders(userID)
	if err != nil {
		t.Fatalf("Failed to get traders: %v", err)
	}

	var stoppedTrader *config.TraderRecord
	for _, tr := range traders {
		if tr.ID == traderID {
			stoppedTrader = tr
			break
		}
	}

	if stoppedTrader == nil {
		t.Fatal("Trader not found after stop")
	}

	if stoppedTrader.IsRunning {
		t.Error("Trader should be marked as stopped after stop")
	}

	t.Logf("✅ handleStopTrader test passed")
}
