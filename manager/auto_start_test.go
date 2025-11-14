package manager

import (
	"nofx/auth"
	"nofx/config"
	"os"
	"testing"
)

// setupTestDatabase creates a temporary test database
func setupTestDatabase(t *testing.T) (*config.Database, func()) {
	tmpDB, err := os.CreateTemp("", "test_manager_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	tmpDB.Close()

	db, err := config.NewDatabase(tmpDB.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpDB.Name())
	}

	return db, cleanup
}

// TestStartRunningTraders_NoRunningTraders tests behavior when no traders are running
func TestStartRunningTraders_NoRunningTraders(t *testing.T) {
	db, cleanup := setupTestDatabase(t)
	defer cleanup()

	tm := NewTraderManager()

	// Create a test user
	hashedPassword, _ := auth.HashPassword("TestPass123!")
	user := &config.User{
		ID:           "test-user-001",
		Email:        "test@example.com",
		PasswordHash: hashedPassword,
		OTPSecret:    "JBSWY3DPEHPK3PXP",
		OTPVerified:  true,
	}
	err := db.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create AI model and exchange
	err = db.CreateAIModel(user.ID, "test-model", "Test Model", "openai", true, "test-key", "http://test")
	if err != nil {
		t.Fatalf("Failed to create AI model: %v", err)
	}

	err = db.CreateExchange(user.ID, "binance", "Binance", "cex", true, "test-key", "test-secret", false, "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create exchange: %v", err)
	}

	// Get IDs
	aiModels, _ := db.GetAIModels(user.ID)
	exchanges, _ := db.GetExchanges(user.ID)

	// Create a trader that is NOT running
	trader := &config.TraderRecord{
		ID:                  "test-trader-not-running",
		UserID:              user.ID,
		Name:                "Test Trader Not Running",
		AIModelID:           aiModels[0].ID,
		ExchangeID:          exchanges[0].ID,
		InitialBalance:      1000.0,
		ScanIntervalMinutes: 3,
		IsRunning:           false, // NOT running
		BTCETHLeverage:      5,
		AltcoinLeverage:     5,
		TradingSymbols:      "BTCUSDT",
		UseCoinPool:         false,
		UseOITop:            false,
	}
	err = db.CreateTrader(trader)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	// Load traders into manager
	err = tm.LoadUserTraders(db, user.ID)
	if err != nil {
		t.Fatalf("Failed to load traders: %v", err)
	}

	// Call StartRunningTraders - should not start any traders
	err = tm.StartRunningTraders(db)
	if err != nil {
		t.Errorf("StartRunningTraders failed: %v", err)
	}

	t.Logf("✅ StartRunningTraders correctly handled no running traders scenario")
}

// TestStartRunningTraders_WithRunningTraders tests starting traders marked as running
func TestStartRunningTraders_WithRunningTraders(t *testing.T) {
	db, cleanup := setupTestDatabase(t)
	defer cleanup()

	tm := NewTraderManager()

	// Create test user
	hashedPassword, _ := auth.HashPassword("TestPass123!")
	user := &config.User{
		ID:           "test-user-002",
		Email:        "test2@example.com",
		PasswordHash: hashedPassword,
		OTPSecret:    "JBSWY3DPEHPK3PXP",
		OTPVerified:  true,
	}
	err := db.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create AI model and exchange
	err = db.CreateAIModel(user.ID, "test-model", "Test Model", "openai", true, "test-key", "http://test")
	if err != nil {
		t.Fatalf("Failed to create AI model: %v", err)
	}

	err = db.CreateExchange(user.ID, "binance", "Binance", "cex", true, "test-key", "test-secret", false, "", "", "", "")
	if err != nil {
		t.Fatalf("Failed to create exchange: %v", err)
	}

	// Get IDs
	aiModels, _ := db.GetAIModels(user.ID)
	exchanges, _ := db.GetExchanges(user.ID)

	// Create a trader that IS running
	trader := &config.TraderRecord{
		ID:                  "test-trader-running",
		UserID:              user.ID,
		Name:                "Test Trader Running",
		AIModelID:           aiModels[0].ID,
		ExchangeID:          exchanges[0].ID,
		InitialBalance:      1000.0,
		ScanIntervalMinutes: 3,
		IsRunning:           true, // IS running
		BTCETHLeverage:      5,
		AltcoinLeverage:     5,
		TradingSymbols:      "BTCUSDT",
		UseCoinPool:         false,
		UseOITop:            false,
	}
	err = db.CreateTrader(trader)
	if err != nil {
		t.Fatalf("Failed to create trader: %v", err)
	}

	// Load traders into manager
	err = tm.LoadUserTraders(db, user.ID)
	if err != nil {
		t.Fatalf("Failed to load traders: %v", err)
	}

	// Call StartRunningTraders - should attempt to start the trader
	// Note: The trader will fail to actually run due to invalid API keys,
	// but StartRunningTraders should not return an error
	err = tm.StartRunningTraders(db)
	if err != nil {
		t.Errorf("StartRunningTraders failed: %v", err)
	}

	t.Logf("✅ StartRunningTraders correctly attempted to start running traders")
}

// TestStartRunningTraders_MultipleUsers tests starting traders for multiple users
func TestStartRunningTraders_MultipleUsers(t *testing.T) {
	db, cleanup := setupTestDatabase(t)
	defer cleanup()

	tm := NewTraderManager()

	// Create two test users
	for i := 1; i <= 2; i++ {
		hashedPassword, _ := auth.HashPassword("TestPass123!")
		user := &config.User{
			ID:           "test-user-multi-" + string(rune('0'+i)),
			Email:        "testmulti" + string(rune('0'+i)) + "@example.com",
			PasswordHash: hashedPassword,
			OTPSecret:    "JBSWY3DPEHPK3PXP",
			OTPVerified:  true,
		}
		err := db.CreateUser(user)
		if err != nil {
			t.Fatalf("Failed to create user %d: %v", i, err)
		}

		// Create AI model and exchange for each user
		err = db.CreateAIModel(user.ID, "test-model", "Test Model", "openai", true, "test-key", "http://test")
		if err != nil {
			t.Fatalf("Failed to create AI model for user %d: %v", i, err)
		}

		err = db.CreateExchange(user.ID, "binance", "Binance", "cex", true, "test-key", "test-secret", false, "", "", "", "")
		if err != nil {
			t.Fatalf("Failed to create exchange for user %d: %v", i, err)
		}

		// Get IDs
		aiModels, _ := db.GetAIModels(user.ID)
		exchanges, _ := db.GetExchanges(user.ID)

		// Create a running trader for each user
		trader := &config.TraderRecord{
			ID:                  "test-trader-multi-" + string(rune('0'+i)),
			UserID:              user.ID,
			Name:                "Test Trader Multi " + string(rune('0'+i)),
			AIModelID:           aiModels[0].ID,
			ExchangeID:          exchanges[0].ID,
			InitialBalance:      1000.0,
			ScanIntervalMinutes: 3,
			IsRunning:           true,
			BTCETHLeverage:      5,
			AltcoinLeverage:     5,
			TradingSymbols:      "BTCUSDT",
			UseCoinPool:         false,
			UseOITop:            false,
		}
		err = db.CreateTrader(trader)
		if err != nil {
			t.Fatalf("Failed to create trader for user %d: %v", i, err)
		}

		// Load traders into manager
		err = tm.LoadUserTraders(db, user.ID)
		if err != nil {
			t.Fatalf("Failed to load traders for user %d: %v", i, err)
		}
	}

	// Call StartRunningTraders - should start traders for both users
	err := tm.StartRunningTraders(db)
	if err != nil {
		t.Errorf("StartRunningTraders failed: %v", err)
	}

	t.Logf("✅ StartRunningTraders correctly handled multiple users")
}
