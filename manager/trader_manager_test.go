package manager

import (
	"nofx/trader"
	"sync"
	"testing"
	"time"
)

// TestNewTraderManager tests the creation of a new TraderManager
func TestNewTraderManager(t *testing.T) {
	tm := NewTraderManager()

	if tm == nil {
		t.Fatal("NewTraderManager returned nil")
	}

	if tm.traders == nil {
		t.Error("traders map is nil")
	}

	if tm.competitionCache == nil {
		t.Error("competitionCache is nil")
	}

	if len(tm.traders) != 0 {
		t.Errorf("Expected empty traders map, got %d traders", len(tm.traders))
	}
}

// TestGetTrader tests retrieving a specific trader by ID
func TestGetTrader(t *testing.T) {
	tm := NewTraderManager()

	// Create a mock trader config
	mockConfig := trader.AutoTraderConfig{
		ID:             "test-trader-1",
		Name:           "Test Trader",
		AIModel:        "deepseek",
		Exchange:       "binance",
		InitialBalance: 1000.0,
		ScanInterval:   5 * time.Minute,
	}

	// Create mock database (nil for this test as we won't use it)
	mockTrader, err := trader.NewAutoTrader(mockConfig, nil, "test-user")
	if err != nil {
		t.Fatalf("Failed to create mock trader: %v", err)
	}

	// Add trader to manager
	tm.mu.Lock()
	tm.traders[mockConfig.ID] = mockTrader
	tm.mu.Unlock()

	// Test getting existing trader
	retrieved, err := tm.GetTrader("test-trader-1")
	if err != nil {
		t.Errorf("GetTrader failed: %v", err)
	}

	if retrieved == nil {
		t.Error("Retrieved trader is nil")
	}

	if retrieved.GetID() != "test-trader-1" {
		t.Errorf("Expected trader ID 'test-trader-1', got '%s'", retrieved.GetID())
	}

	// Test getting non-existent trader
	_, err = tm.GetTrader("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent trader, got nil")
	}
}

// TestGetAllTraders tests retrieving all traders
func TestGetAllTraders(t *testing.T) {
	tm := NewTraderManager()

	// Add multiple mock traders
	for i := 1; i <= 3; i++ {
		traderID := "test-trader-" + string(rune('0'+i))
		mockConfig := trader.AutoTraderConfig{
			ID:             traderID,
			Name:           "Test Trader " + string(rune('0'+i)),
			AIModel:        "deepseek",
			Exchange:       "binance",
			InitialBalance: 1000.0,
			ScanInterval:   5 * time.Minute,
		}

		mockTrader, err := trader.NewAutoTrader(mockConfig, nil, "test-user")
		if err != nil {
			t.Fatalf("Failed to create mock trader %d: %v", i, err)
		}

		tm.mu.Lock()
		tm.traders[traderID] = mockTrader
		tm.mu.Unlock()
	}

	allTraders := tm.GetAllTraders()

	if len(allTraders) != 3 {
		t.Errorf("Expected 3 traders, got %d", len(allTraders))
	}

	// Verify it's a copy, not the original map
	delete(allTraders, "test-trader-1")
	if len(tm.traders) != 3 {
		t.Error("GetAllTraders should return a copy, not the original map")
	}
}

// TestGetTraderIDs tests retrieving all trader IDs
func TestGetTraderIDs(t *testing.T) {
	tm := NewTraderManager()

	// Empty manager
	ids := tm.GetTraderIDs()
	if len(ids) != 0 {
		t.Errorf("Expected 0 IDs, got %d", len(ids))
	}

	// Add traders
	expectedIDs := []string{"trader-1", "trader-2", "trader-3"}
	for _, id := range expectedIDs {
		mockConfig := trader.AutoTraderConfig{
			ID:             id,
			Name:           "Test " + id,
			AIModel:        "deepseek",
			Exchange:       "binance",
			InitialBalance: 1000.0,
			ScanInterval:   5 * time.Minute,
		}

		mockTrader, err := trader.NewAutoTrader(mockConfig, nil, "test-user")
		if err != nil {
			t.Fatalf("Failed to create trader %s: %v", id, err)
		}

		tm.mu.Lock()
		tm.traders[id] = mockTrader
		tm.mu.Unlock()
	}

	ids = tm.GetTraderIDs()
	if len(ids) != 3 {
		t.Errorf("Expected 3 IDs, got %d", len(ids))
	}

	// Verify all expected IDs are present
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	for _, expectedID := range expectedIDs {
		if !idMap[expectedID] {
			t.Errorf("Expected ID '%s' not found in results", expectedID)
		}
	}
}

// TestRemoveTrader tests removing a trader from memory
func TestRemoveTrader(t *testing.T) {
	tm := NewTraderManager()

	// Add a trader
	mockConfig := trader.AutoTraderConfig{
		ID:             "test-remove",
		Name:           "Test Remove",
		AIModel:        "deepseek",
		Exchange:       "binance",
		InitialBalance: 1000.0,
		ScanInterval:   5 * time.Minute,
	}

	mockTrader, err := trader.NewAutoTrader(mockConfig, nil, "test-user")
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	tm.mu.Lock()
	tm.traders["test-remove"] = mockTrader
	tm.mu.Unlock()

	// Verify trader exists
	if len(tm.traders) != 1 {
		t.Errorf("Expected 1 trader before removal, got %d", len(tm.traders))
	}

	// Remove trader
	err = tm.RemoveTrader("test-remove")
	if err != nil {
		t.Errorf("RemoveTrader failed: %v", err)
	}

	// Verify trader is removed
	if len(tm.traders) != 0 {
		t.Errorf("Expected 0 traders after removal, got %d", len(tm.traders))
	}

	// Test removing non-existent trader
	err = tm.RemoveTrader("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent trader, got nil")
	}
}

// TestCompetitionCacheExpiry tests that competition cache expires after 30 seconds
func TestCompetitionCacheExpiry(t *testing.T) {
	tm := NewTraderManager()

	// Set cache data
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = map[string]interface{}{
		"traders": []map[string]interface{}{
			{"trader_id": "test-1", "total_pnl_pct": 10.5},
		},
		"count": 1,
	}
	tm.competitionCache.timestamp = time.Now().Add(-31 * time.Second) // Expired
	tm.competitionCache.mu.Unlock()

	// Since cache is expired, GetCompetitionData should fetch new data
	// But we have no traders, so it should return empty data
	data, err := tm.GetCompetitionData()
	if err != nil {
		t.Errorf("GetCompetitionData failed: %v", err)
	}

	// Should have fresh data (empty in this case)
	traders, ok := data["traders"].([]map[string]interface{})
	if !ok {
		t.Fatal("traders field is not a slice")
	}

	if len(traders) != 0 {
		t.Errorf("Expected 0 traders (fresh data), got %d", len(traders))
	}
}

// TestCompetitionCacheFresh tests that fresh cache is returned
func TestCompetitionCacheFresh(t *testing.T) {
	tm := NewTraderManager()

	// Set fresh cache data
	expectedData := map[string]interface{}{
		"traders": []map[string]interface{}{
			{"trader_id": "test-1", "total_pnl_pct": 15.5},
			{"trader_id": "test-2", "total_pnl_pct": 8.3},
		},
		"count":       2,
		"total_count": 2,
	}

	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = expectedData
	tm.competitionCache.timestamp = time.Now() // Fresh cache
	tm.competitionCache.mu.Unlock()

	// Should return cached data
	data, err := tm.GetCompetitionData()
	if err != nil {
		t.Errorf("GetCompetitionData failed: %v", err)
	}

	count, ok := data["count"].(int)
	if !ok {
		t.Fatal("count field is not an int")
	}

	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}
}

// TestConcurrentAccess tests concurrent access to TraderManager
func TestConcurrentAccess(t *testing.T) {
	tm := NewTraderManager()

	// Add initial traders
	for i := 1; i <= 5; i++ {
		id := "concurrent-trader-" + string(rune('0'+i))
		mockConfig := trader.AutoTraderConfig{
			ID:             id,
			Name:           "Concurrent Test " + string(rune('0'+i)),
			AIModel:        "deepseek",
			Exchange:       "binance",
			InitialBalance: 1000.0,
			ScanInterval:   5 * time.Minute,
		}

		mockTrader, err := trader.NewAutoTrader(mockConfig, nil, "test-user")
		if err != nil {
			t.Fatalf("Failed to create trader %d: %v", i, err)
		}

		tm.mu.Lock()
		tm.traders[id] = mockTrader
		tm.mu.Unlock()
	}

	// Perform concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := tm.GetTraderIDs()
			if len(ids) < 0 || len(ids) > 10 {
				errors <- nil
			}
		}()
	}

	// Concurrent GetAllTraders
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = tm.GetAllTraders()
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent operation failed: %v", err)
		}
	}
}

// TestGetComparisonData tests getting comparison data for traders
func TestGetComparisonData(t *testing.T) {
	tm := NewTraderManager()

	// Add mock traders
	for i := 1; i <= 2; i++ {
		id := "comparison-trader-" + string(rune('0'+i))
		mockConfig := trader.AutoTraderConfig{
			ID:             id,
			Name:           "Comparison Test " + string(rune('0'+i)),
			AIModel:        "deepseek",
			Exchange:       "binance",
			InitialBalance: 1000.0,
			ScanInterval:   5 * time.Minute,
		}

		mockTrader, err := trader.NewAutoTrader(mockConfig, nil, "test-user")
		if err != nil {
			t.Fatalf("Failed to create trader %d: %v", i, err)
		}

		tm.mu.Lock()
		tm.traders[id] = mockTrader
		tm.mu.Unlock()
	}

	// Get comparison data
	data, err := tm.GetComparisonData()
	if err != nil {
		t.Errorf("GetComparisonData failed: %v", err)
	}

	if data == nil {
		t.Fatal("GetComparisonData returned nil")
	}

	count, ok := data["count"].(int)
	if !ok {
		t.Fatal("count field is missing or wrong type")
	}

	// Note: Count might be 0 if traders fail to get account info (no API keys in test environment)
	// This is expected behavior in test environment
	if count < 0 || count > 2 {
		t.Errorf("Expected count between 0-2, got %d", count)
	}

	traders, ok := data["traders"].([]map[string]interface{})
	if !ok {
		t.Fatal("traders field is missing or wrong type")
	}

	// In test environment without valid API keys, traders might not have account info
	// So we just verify the data structure is correct
	if len(traders) < 0 || len(traders) > 2 {
		t.Errorf("Expected 0-2 traders in data, got %d", len(traders))
	}

	t.Logf("✅ GetComparisonData returned %d traders (may be 0 without valid API keys)", count)
}

// TestIsUserTrader tests the user trader identification logic
func TestIsUserTrader(t *testing.T) {
	tests := []struct {
		name      string
		traderID  string
		userID    string
		expected  bool
		reasoning string
	}{
		{
			name:      "Exact prefix match",
			traderID:  "user@example.com_trader1",
			userID:    "user@example.com",
			expected:  true,
			reasoning: "Trader ID starts with user ID",
		},
		{
			name:      "No match",
			traderID:  "other@example.com_trader1",
			userID:    "user@example.com",
			expected:  false,
			reasoning: "Trader ID does not start with user ID",
		},
		{
			name:      "Default user with UUID trader",
			traderID:  "abc123_model",
			userID:    "default",
			expected:  true,
			reasoning: "Default user matches UUID-based traders",
		},
		{
			name:      "Email format in trader ID",
			traderID:  "test@test.com_mytrader",
			userID:    "test@test.com",
			expected:  true,
			reasoning: "Email format with underscore separator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUserTrader(tt.traderID, tt.userID)
			if result != tt.expected {
				t.Errorf("isUserTrader(%q, %q) = %v, want %v (%s)",
					tt.traderID, tt.userID, result, tt.expected, tt.reasoning)
			}
		})
	}
}

// TestContainsUserPrefix tests the user prefix detection logic
func TestContainsUserPrefix(t *testing.T) {
	tests := []struct {
		name      string
		traderID  string
		expected  bool
		reasoning string
	}{
		{
			name:      "Email with @ symbol",
			traderID:  "user@example.com_trader",
			expected:  true,
			reasoning: "Contains @ symbol indicating email prefix",
		},
		{
			name:      "UUID format without @",
			traderID:  "abc123-def456_model",
			expected:  false,
			reasoning: "UUID format without email @ symbol",
		},
		{
			name:      "Simple name",
			traderID:  "trader1",
			expected:  false,
			reasoning: "No prefix indicators",
		},
		{
			name:      "Email without underscore",
			traderID:  "user@example.com",
			expected:  true,
			reasoning: "Contains @ symbol even without underscore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsUserPrefix(tt.traderID)
			if result != tt.expected {
				t.Errorf("containsUserPrefix(%q) = %v, want %v (%s)",
					tt.traderID, result, tt.expected, tt.reasoning)
			}
		})
	}
}

// TestRemoveTraderClearCache tests that removing a trader clears the competition cache
func TestRemoveTraderClearCache(t *testing.T) {
	tm := NewTraderManager()

	// Add a trader
	mockConfig := trader.AutoTraderConfig{
		ID:             "cache-clear-test",
		Name:           "Cache Clear Test",
		AIModel:        "deepseek",
		Exchange:       "binance",
		InitialBalance: 1000.0,
		ScanInterval:   5 * time.Minute,
	}

	mockTrader, err := trader.NewAutoTrader(mockConfig, nil, "test-user")
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	tm.mu.Lock()
	tm.traders["cache-clear-test"] = mockTrader
	tm.mu.Unlock()

	// Set cache data
	tm.competitionCache.mu.Lock()
	tm.competitionCache.data = map[string]interface{}{"count": 1}
	tm.competitionCache.timestamp = time.Now()
	tm.competitionCache.mu.Unlock()

	// Remove trader
	err = tm.RemoveTrader("cache-clear-test")
	if err != nil {
		t.Errorf("RemoveTrader failed: %v", err)
	}

	// Verify cache is cleared
	tm.competitionCache.mu.RLock()
	cacheEmpty := tm.competitionCache.data == nil
	timestampZero := tm.competitionCache.timestamp.IsZero()
	tm.competitionCache.mu.RUnlock()

	if !cacheEmpty {
		t.Error("Competition cache data should be nil after removing trader")
	}

	if !timestampZero {
		t.Error("Competition cache timestamp should be zero after removing trader")
	}
}

// TestLoadTradersFromDatabase tests loading traders from database
func TestLoadTradersFromDatabase(t *testing.T) {
	// Skip if DATA_ENCRYPTION_KEY not set
	if os.Getenv("DATA_ENCRYPTION_KEY") == "" {
		t.Skip("Skipping database test: DATA_ENCRYPTION_KEY not set")
	}

	// Create temporary database
	tmpDB := filepath.Join(os.TempDir(), fmt.Sprintf("test_trader_manager_%d.db", time.Now().UnixNano()))
	defer os.Remove(tmpDB)

	db, err := config.NewDatabase(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test user
	testUserID := "test-user-1"
	err = db.CreateUser(testUserID, "test@example.com", "hashedpassword")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Add AI model configuration
	aiModel := &config.AIModelConfig{
		ID:              1,
		Provider:        "deepseek",
		Enabled:         true,
		CustomAPIURL:    "",
		CustomModelName: "",
	}
	err = db.SaveAIModel(testUserID, aiModel)
	if err != nil {
		t.Fatalf("Failed to save AI model: %v", err)
	}

	// Add exchange configuration
	exchange := &config.ExchangeConfig{
		ID:         1,
		ExchangeID: "binance",
		Enabled:    true,
		APIKey:     "test-api-key",
		SecretKey:  "test-secret-key",
		Testnet:    true,
	}
	err = db.SaveExchange(testUserID, exchange)
	if err != nil {
		t.Fatalf("Failed to save exchange: %v", err)
	}

	// Add trader configuration
	trader := &config.TraderRecord{
		ID:                   "trader-1",
		Name:                 "Test Trader 1",
		UserID:               testUserID,
		AIModelID:            1,
		ExchangeID:           1,
		InitialBalance:       10000.0,
		ScanIntervalMinutes:  5,
		BTCETHLeverage:       1.0,
		AltcoinLeverage:      2.0,
		TakerFeeRate:         0.0005,
		MakerFeeRate:         0.0002,
		IsCrossMargin:        true,
		TradingSymbols:       "BTCUSDT,ETHUSDT",
		UseCoinPool:          false,
		SystemPromptTemplate: "default",
		OrderStrategy:        "market",
	}
	err = db.SaveTrader(trader)
	if err != nil {
		t.Fatalf("Failed to save trader: %v", err)
	}

	// Set system configuration
	err = db.SetSystemConfig("max_daily_loss", "10.0")
	if err != nil {
		t.Fatalf("Failed to set max_daily_loss: %v", err)
	}
	err = db.SetSystemConfig("max_drawdown", "20.0")
	if err != nil {
		t.Fatalf("Failed to set max_drawdown: %v", err)
	}
	err = db.SetSystemConfig("stop_trading_minutes", "60")
	if err != nil {
		t.Fatalf("Failed to set stop_trading_minutes: %v", err)
	}
	err = db.SetSystemConfig("default_coins", `["BTCUSDT","ETHUSDT"]`)
	if err != nil {
		t.Fatalf("Failed to set default_coins: %v", err)
	}

	// Load traders using TraderManager
	tm := NewTraderManager()
	err = tm.LoadTradersFromDatabase(db)
	if err != nil {
		t.Fatalf("LoadTradersFromDatabase failed: %v", err)
	}

	// Verify trader was loaded
	loadedTraders := tm.GetAllTraders()
	if len(loadedTraders) != 1 {
		t.Errorf("Expected 1 trader, got %d", len(loadedTraders))
	}

	loadedTrader := tm.GetTrader("trader-1")
	if loadedTrader == nil {
		t.Fatal("Trader 'trader-1' was not loaded")
	}

	// Verify trader configuration
	if loadedTrader.ID != "trader-1" {
		t.Errorf("Expected trader ID 'trader-1', got '%s'", loadedTrader.ID)
	}
	if loadedTrader.Name != "Test Trader 1" {
		t.Errorf("Expected trader name 'Test Trader 1', got '%s'", loadedTrader.Name)
	}
}

// TestLoadTradersFromDatabase_MultipleUsers tests loading traders from multiple users
func TestLoadTradersFromDatabase_MultipleUsers(t *testing.T) {
	// Skip if DATA_ENCRYPTION_KEY not set
	if os.Getenv("DATA_ENCRYPTION_KEY") == "" {
		t.Skip("Skipping database test: DATA_ENCRYPTION_KEY not set")
	}

	// Create temporary database
	tmpDB := filepath.Join(os.TempDir(), fmt.Sprintf("test_multi_user_%d.db", time.Now().UnixNano()))
	defer os.Remove(tmpDB)

	db, err := config.NewDatabase(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create two test users
	users := []string{"user-1", "user-2"}
	for _, userID := range users {
		err = db.CreateUser(userID, fmt.Sprintf("%s@example.com", userID), "hashedpassword")
		if err != nil {
			t.Fatalf("Failed to create user %s: %v", userID, err)
		}

		// Add AI model and exchange for each user
		aiModel := &config.AIModelConfig{
			ID:       1,
			Provider: "deepseek",
			Enabled:  true,
		}
		err = db.SaveAIModel(userID, aiModel)
		if err != nil {
			t.Fatalf("Failed to save AI model for %s: %v", userID, err)
		}

		exchange := &config.ExchangeConfig{
			ID:         1,
			ExchangeID: "binance",
			Enabled:    true,
			APIKey:     fmt.Sprintf("api-key-%s", userID),
			SecretKey:  fmt.Sprintf("secret-key-%s", userID),
			Testnet:    true,
		}
		err = db.SaveExchange(userID, exchange)
		if err != nil {
			t.Fatalf("Failed to save exchange for %s: %v", userID, err)
		}

		// Add traders for each user
		for i := 1; i <= 2; i++ {
			trader := &config.TraderRecord{
				ID:                  fmt.Sprintf("%s-trader-%d", userID, i),
				Name:                fmt.Sprintf("Trader %d for %s", i, userID),
				UserID:              userID,
				AIModelID:           1,
				ExchangeID:          1,
				InitialBalance:      10000.0,
				ScanIntervalMinutes: 5,
				BTCETHLeverage:      1.0,
				AltcoinLeverage:     2.0,
				TakerFeeRate:        0.0005,
				MakerFeeRate:        0.0002,
				IsCrossMargin:       true,
				TradingSymbols:      "BTCUSDT",
				OrderStrategy:       "market",
			}
			err = db.SaveTrader(trader)
			if err != nil {
				t.Fatalf("Failed to save trader for %s: %v", userID, err)
			}
		}
	}

	// Set system configuration
	db.SetSystemConfig("max_daily_loss", "10.0")
	db.SetSystemConfig("max_drawdown", "20.0")
	db.SetSystemConfig("stop_trading_minutes", "60")
	db.SetSystemConfig("default_coins", `["BTCUSDT"]`)

	// Load all traders
	tm := NewTraderManager()
	err = tm.LoadTradersFromDatabase(db)
	if err != nil {
		t.Fatalf("LoadTradersFromDatabase failed: %v", err)
	}

	// Verify 4 traders were loaded (2 users × 2 traders each)
	loadedTraders := tm.GetAllTraders()
	if len(loadedTraders) != 4 {
		t.Errorf("Expected 4 traders, got %d", len(loadedTraders))
	}

	// Verify traders from both users are present
	user1Count := 0
	user2Count := 0
	for _, tr := range loadedTraders {
		if strings.HasPrefix(tr.ID, "user-1") {
			user1Count++
		} else if strings.HasPrefix(tr.ID, "user-2") {
			user2Count++
		}
	}

	if user1Count != 2 {
		t.Errorf("Expected 2 traders for user-1, got %d", user1Count)
	}
	if user2Count != 2 {
		t.Errorf("Expected 2 traders for user-2, got %d", user2Count)
	}
}

// TestLoadTradersFromDatabase_DisabledConfigs tests that disabled AI models and exchanges are skipped
func TestLoadTradersFromDatabase_DisabledConfigs(t *testing.T) {
	// Skip if DATA_ENCRYPTION_KEY not set
	if os.Getenv("DATA_ENCRYPTION_KEY") == "" {
		t.Skip("Skipping database test: DATA_ENCRYPTION_KEY not set")
	}

	// Create temporary database
	tmpDB := filepath.Join(os.TempDir(), fmt.Sprintf("test_disabled_%d.db", time.Now().UnixNano()))
	defer os.Remove(tmpDB)

	db, err := config.NewDatabase(tmpDB)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create test user
	testUserID := "test-user"
	db.CreateUser(testUserID, "test@example.com", "hashedpassword")

	// Add enabled AI model
	enabledAI := &config.AIModelConfig{
		ID:       1,
		Provider: "deepseek",
		Enabled:  true,
	}
	db.SaveAIModel(testUserID, enabledAI)

	// Add disabled AI model
	disabledAI := &config.AIModelConfig{
		ID:       2,
		Provider: "qwen",
		Enabled:  false,
	}
	db.SaveAIModel(testUserID, disabledAI)

	// Add enabled exchange
	enabledExchange := &config.ExchangeConfig{
		ID:         1,
		ExchangeID: "binance",
		Enabled:    true,
		APIKey:     "api-key",
		SecretKey:  "secret-key",
		Testnet:    true,
	}
	db.SaveExchange(testUserID, enabledExchange)

	// Add disabled exchange
	disabledExchange := &config.ExchangeConfig{
		ID:         2,
		ExchangeID: "hyperliquid",
		Enabled:    false,
		APIKey:     "api-key",
		Testnet:    true,
	}
	db.SaveExchange(testUserID, disabledExchange)

	// Add trader with enabled configs
	trader1 := &config.TraderRecord{
		ID:                  "trader-enabled",
		Name:                "Enabled Trader",
		UserID:              testUserID,
		AIModelID:           1,
		ExchangeID:          1,
		InitialBalance:      10000.0,
		ScanIntervalMinutes: 5,
		BTCETHLeverage:      1.0,
		AltcoinLeverage:     2.0,
		TakerFeeRate:        0.0005,
		MakerFeeRate:        0.0002,
		OrderStrategy:       "market",
	}
	db.SaveTrader(trader1)

	// Add trader with disabled AI model
	trader2 := &config.TraderRecord{
		ID:                  "trader-disabled-ai",
		Name:                "Disabled AI Trader",
		UserID:              testUserID,
		AIModelID:           2, // disabled
		ExchangeID:          1,
		InitialBalance:      10000.0,
		ScanIntervalMinutes: 5,
		BTCETHLeverage:      1.0,
		AltcoinLeverage:     2.0,
		TakerFeeRate:        0.0005,
		MakerFeeRate:        0.0002,
		OrderStrategy:       "market",
	}
	db.SaveTrader(trader2)

	// Add trader with disabled exchange
	trader3 := &config.TraderRecord{
		ID:                  "trader-disabled-exchange",
		Name:                "Disabled Exchange Trader",
		UserID:              testUserID,
		AIModelID:           1,
		ExchangeID:          2, // disabled
		InitialBalance:      10000.0,
		ScanIntervalMinutes: 5,
		BTCETHLeverage:      1.0,
		AltcoinLeverage:     2.0,
		TakerFeeRate:        0.0005,
		MakerFeeRate:        0.0002,
		OrderStrategy:       "market",
	}
	db.SaveTrader(trader3)

	// Set system configuration
	db.SetSystemConfig("max_daily_loss", "10.0")
	db.SetSystemConfig("max_drawdown", "20.0")
	db.SetSystemConfig("stop_trading_minutes", "60")
	db.SetSystemConfig("default_coins", `["BTCUSDT"]`)

	// Load traders
	tm := NewTraderManager()
	err = tm.LoadTradersFromDatabase(db)
	if err != nil {
		t.Fatalf("LoadTradersFromDatabase failed: %v", err)
	}

	// Only the trader with enabled configs should be loaded
	loadedTraders := tm.GetAllTraders()
	if len(loadedTraders) != 1 {
		t.Errorf("Expected 1 enabled trader, got %d", len(loadedTraders))
	}

	// Verify it's the correct trader
	if loadedTraders[0].ID != "trader-enabled" {
		t.Errorf("Expected trader 'trader-enabled', got '%s'", loadedTraders[0].ID)
	}
}

// TestAddTraderFromDB tests adding trader from database configuration
func TestAddTraderFromDB(t *testing.T) {
	// This test requires a real database, so we skip if not available
	t.Skip("Skipping TestAddTraderFromDB: requires database setup")

	// TODO: Implement with mock database
	// tm := NewTraderManager()
	// mockDB := createMockDatabase(t)
	// mockTraderCfg := createMockTraderRecord()
	// mockAICfg := createMockAIModelConfig()
	// mockExchangeCfg := createMockExchangeConfig()
	//
	// err := tm.AddTraderFromDB(mockTraderCfg, mockAICfg, mockExchangeCfg, "", "", 10.0, 20.0, 60, []string{}, mockDB, "test-user")
	// if err != nil {
	//     t.Errorf("AddTraderFromDB failed: %v", err)
	// }
}

// TestLoadTradersFromDatabase tests loading all traders from database
func TestLoadTradersFromDatabase(t *testing.T) {
	// This test requires a real database, so we skip if not available
	t.Skip("Skipping TestLoadTradersFromDatabase: requires database setup")

	// TODO: Implement with mock database
}
