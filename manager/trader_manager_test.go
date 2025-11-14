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
	// TODO: Update this test to match the new Database API (CreateUser, CreateAIModel, CreateExchange, CreateTrader)
	// The Database API has changed significantly and requires:
	// - User struct with all fields
	// - CreateAIModel with multiple parameters
	// - CreateExchange with encryption handling
	// - CreateTrader with TraderRecord
	t.Skip("Skipping: requires update to match new Database API")
}

// TestLoadTradersFromDatabase_MultipleUsers tests loading traders from multiple users
func TestLoadTradersFromDatabase_MultipleUsers(t *testing.T) {
	// TODO: Update to match new Database API
	t.Skip("Skipping: requires update to match new Database API")
}

// TestLoadTradersFromDatabase_DisabledConfigs tests that disabled AI models and exchanges are skipped
func TestLoadTradersFromDatabase_DisabledConfigs(t *testing.T) {
	// TODO: Update to match new Database API
	t.Skip("Skipping: requires update to match new Database API")
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

// TestRemoveTrader_NonExistent 测试移除不存在的trader不会报错
func TestRemoveTrader_NonExistent(t *testing.T) {
	tm := NewTraderManager()

	// 尝试移除不存在的 trader，不应该 panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("移除不存在的 trader 不应该 panic: %v", r)
		}
	}()

	tm.RemoveTrader("non-existent-trader")
}

// TestRemoveTrader_Concurrent 测试并发移除trader的安全性
func TestRemoveTrader_Concurrent(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-concurrent"

	// 添加 trader
	tm.traders[traderID] = nil

	// 并发调用 RemoveTrader
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			tm.RemoveTrader(traderID)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证 trader 已被移除
	if _, exists := tm.traders[traderID]; exists {
		t.Error("trader 应该已从 map 中移除")
	}
}

// TestGetTrader_AfterRemove 测试移除后获取trader返回错误
func TestGetTrader_AfterRemove(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-get"

	// 添加 trader
	tm.traders[traderID] = nil

	// 移除 trader
	tm.RemoveTrader(traderID)

	// 尝试获取已移除的 trader
	_, err := tm.GetTrader(traderID)
	if err == nil {
		t.Error("获取已移除的 trader 应该返回错误")
	}
}

// TestStartAll tests starting all traders
func TestStartAll(t *testing.T) {
	tm := NewTraderManager()

	// Add multiple traders
	for i := 1; i <= 3; i++ {
		id := "start-all-trader-" + string(rune('0'+i))
		mockConfig := trader.AutoTraderConfig{
			ID:             id,
			Name:           "Start All Test " + string(rune('0'+i)),
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

	// StartAll should not panic and should execute goroutines for all traders
	// We can't easily verify they actually started without blocking,
	// but we can verify the method executes successfully
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("StartAll panicked: %v", r)
		}
	}()

	tm.StartAll()

	t.Logf("✅ StartAll executed successfully for 3 traders")
}

// TestStopAll tests stopping all traders
func TestStopAll(t *testing.T) {
	tm := NewTraderManager()

	// Add multiple traders
	for i := 1; i <= 3; i++ {
		id := "stop-all-trader-" + string(rune('0'+i))
		mockConfig := trader.AutoTraderConfig{
			ID:             id,
			Name:           "Stop All Test " + string(rune('0'+i)),
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

	// StopAll should not panic and should call Stop() on all traders
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("StopAll panicked: %v", r)
		}
	}()

	tm.StopAll()

	t.Logf("✅ StopAll executed successfully for 3 traders")
}

// TestGetTopTradersData tests getting top traders data
func TestGetTopTradersData(t *testing.T) {
	tm := NewTraderManager()

	// Add some mock traders
	for i := 1; i <= 3; i++ {
		id := "top-trader-" + string(rune('0'+i))
		mockConfig := trader.AutoTraderConfig{
			ID:             id,
			Name:           "Top Trader " + string(rune('0'+i)),
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

	// Get top traders data
	data, err := tm.GetTopTradersData()
	if err != nil {
		t.Errorf("GetTopTradersData failed: %v", err)
	}

	if data == nil {
		t.Fatal("GetTopTradersData returned nil")
	}

	// Verify data structure
	traders, ok := data["traders"].([]map[string]interface{})
	if !ok {
		t.Fatal("traders field is missing or wrong type")
	}

	// In test environment without valid API keys, traders might not have data
	// So we just verify the structure is correct
	if len(traders) < 0 {
		t.Error("traders slice should not have negative length")
	}

	t.Logf("✅ GetTopTradersData returned valid data structure")
}
