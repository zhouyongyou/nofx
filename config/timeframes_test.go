package config

import (
	"os"
	"strings"
	"testing"
)

// TestTimeframes_CRUD 測試 Timeframes 的 CRUD 操作
func TestTimeframes_CRUD(t *testing.T) {
	db, cleanup := setupTestDBForTimeframes(t)
	defer cleanup()

	userID := "test-user-tf-001"
	aiModelID, exchangeID := setupAIModelAndExchange(t, db, userID)

	t.Run("創建_默認值", func(t *testing.T) {
		trader := &TraderRecord{
			ID:                  "trader-default",
			UserID:              userID,
			Name:                "Default Trader",
			AIModelID:           aiModelID,
			ExchangeID:          exchangeID,
			InitialBalance:      1000.0,
			ScanIntervalMinutes: 60,
			Timeframes:          "", // 空值
		}

		err := db.CreateTrader(trader)
		if err != nil {
			t.Fatalf("創建失敗: %v", err)
		}

		traders, _ := db.GetTraders(userID)
		if traders[0].Timeframes != "4h" {
			t.Errorf("預期默認 '4h', 實際 '%s'", traders[0].Timeframes)
		}
		t.Logf("✅ 默認值測試通過: '' → '4h'")
	})

	t.Run("創建_單個值", func(t *testing.T) {
		testValues := []string{"1m", "5m", "15m", "1h", "4h", "1d"}

		for _, tf := range testValues {
			trader := &TraderRecord{
				ID:                  "trader-" + tf,
				UserID:              userID,
				Name:                "Trader " + tf,
				AIModelID:           aiModelID,
				ExchangeID:          exchangeID,
				InitialBalance:      1000.0,
				ScanIntervalMinutes: 60,
				Timeframes:          tf,
			}

			err := db.CreateTrader(trader)
			if err != nil {
				t.Fatalf("創建失敗 (%s): %v", tf, err)
			}
		}

		traders, _ := db.GetTraders(userID)
		t.Logf("✅ 單個值測試通過，創建了 %d 個 trader", len(traders))
	})

	t.Run("創建_多個值", func(t *testing.T) {
		testCases := []struct {
			name string
			tf   string
		}{
			{"兩個", "1m,4h"},
			{"三個", "5m,1h,1d"},
			{"五個", "1m,5m,15m,1h,4h"},
			{"帶空格", "1m, 4h, 1d"},
		}

		for _, tc := range testCases {
			trader := &TraderRecord{
				ID:                  "trader-multi-" + tc.name,
				UserID:              userID,
				Name:                "Multi " + tc.name,
				AIModelID:           aiModelID,
				ExchangeID:          exchangeID,
				InitialBalance:      1000.0,
				ScanIntervalMinutes: 60,
				Timeframes:          tc.tf,
			}

			err := db.CreateTrader(trader)
			if err != nil {
				t.Fatalf("創建失敗 (%s): %v", tc.name, err)
			}

			// 驗證可解析
			parts := strings.Split(tc.tf, ",")
			t.Logf("  %s: %s → %d 個時間框架", tc.name, tc.tf, len(parts))
		}

		t.Logf("✅ 多個值測試通過")
	})

	t.Run("更新", func(t *testing.T) {
		trader := &TraderRecord{
			ID:                  "trader-update",
			UserID:              userID,
			Name:                "Update Trader",
			AIModelID:           aiModelID,
			ExchangeID:          exchangeID,
			InitialBalance:      1000.0,
			ScanIntervalMinutes: 60,
			Timeframes:          "4h",
		}

		_ = db.CreateTrader(trader)

		updates := []string{"1m", "1m,4h", "5m,1h,1d"}
		for _, newTf := range updates {
			trader.Timeframes = newTf
			err := db.UpdateTrader(trader)
			if err != nil {
				t.Fatalf("更新失敗 (%s): %v", newTf, err)
			}

			traders, _ := db.GetTraders(userID)
			found := false
			for _, tr := range traders {
				if tr.ID == trader.ID && tr.Timeframes == newTf {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("更新驗證失敗: %s", newTf)
			}
		}

		t.Logf("✅ 更新測試通過")
	})
}

// TestTimeframes_GetAllTimeframes 測試獲取所有時間框架並集
func TestTimeframes_GetAllTimeframes(t *testing.T) {
	db, cleanup := setupTestDBForTimeframes(t)
	defer cleanup()

	userID := "test-user-tf-002"
	aiModelID, exchangeID := setupAIModelAndExchange(t, db, userID)

	// 創建多個 trader
	traders := []struct {
		id        string
		tf        string
		isRunning bool
	}{
		{"t1", "1m,4h", true},        // 運行中
		{"t2", "4h,1d", true},        // 運行中
		{"t3", "5m,15m", false},      // 未運行 - 不應包含
		{"t4", "1h", true},           // 運行中
		{"t5", "", true},             // 空字符串 - 使用默認 4h
		{"t6", "1m,1h,1d", false},    // 未運行 - 不應包含
	}

	for _, tr := range traders {
		trader := &TraderRecord{
			ID:                  tr.id,
			UserID:              userID,
			Name:                tr.id,
			AIModelID:           aiModelID,
			ExchangeID:          exchangeID,
			InitialBalance:      1000.0,
			ScanIntervalMinutes: 60,
			IsRunning:           tr.isRunning,
			Timeframes:          tr.tf,
		}
		_ = db.CreateTrader(trader)
	}

	// 獲取所有時間框架
	allTf := db.GetAllTimeframes()

	// 驗證
	expected := map[string]bool{
		"1m": true, // 來自 t1
		"4h": true, // 來自 t1, t2, t5(默認)
		"1d": true, // 來自 t2
		"1h": true, // 來自 t4
	}

	unexpected := map[string]bool{
		"5m":  true, // 來自未運行的 t3
		"15m": true, // 來自未運行的 t3
	}

	resultSet := make(map[string]bool)
	for _, tf := range allTf {
		resultSet[tf] = true
	}

	for exp := range expected {
		if !resultSet[exp] {
			t.Errorf("缺少預期的: %s", exp)
		}
	}

	for unexp := range unexpected {
		if resultSet[unexp] {
			t.Errorf("包含了不應存在的: %s", unexp)
		}
	}

	t.Logf("✅ GetAllTimeframes 測試通過")
	t.Logf("   結果: %v", allTf)
}

// TestTimeframes_GetTraderConfig 測試完整配置返回
func TestTimeframes_GetTraderConfig(t *testing.T) {
	db, cleanup := setupTestDBForTimeframes(t)
	defer cleanup()

	userID := "test-user-tf-003"
	aiModelID, exchangeID := setupAIModelAndExchange(t, db, userID)

	trader := &TraderRecord{
		ID:                  "trader-config",
		UserID:              userID,
		Name:                "Config Trader",
		AIModelID:           aiModelID,
		ExchangeID:          exchangeID,
		InitialBalance:      1000.0,
		ScanIntervalMinutes: 60,
		Timeframes:          "1m,5m,1h,4h,1d",
	}

	_ = db.CreateTrader(trader)

	traderRecord, aiModel, exchange, err := db.GetTraderConfig(userID, trader.ID)
	if err != nil {
		t.Fatalf("獲取配置失敗: %v", err)
	}

	if traderRecord.Timeframes != trader.Timeframes {
		t.Errorf("Timeframes 不匹配: 預期 '%s', 實際 '%s'",
			trader.Timeframes, traderRecord.Timeframes)
	}

	t.Logf("✅ GetTraderConfig 測試通過: %s", traderRecord.Timeframes)
	t.Logf("   AI Model: %s, Exchange: %s", aiModel.Provider, exchange.Name)
}

// TestTimeframes_EdgeCases 測試邊緣情況
func TestTimeframes_EdgeCases(t *testing.T) {
	db, cleanup := setupTestDBForTimeframes(t)
	defer cleanup()

	userID := "test-user-tf-004"
	aiModelID, exchangeID := setupAIModelAndExchange(t, db, userID)

	testCases := []struct {
		name     string
		tf       string
		expected string
		note     string
	}{
		{
			name:     "空字符串",
			tf:       "",
			expected: "4h",
			note:     "應使用默認值",
		},
		{
			name:     "重複值",
			tf:       "4h,4h,4h",
			expected: "4h,4h,4h",
			note:     "數據庫保留重複，應用層應去重",
		},
		{
			name:     "長字符串",
			tf:       "1m,5m,15m,30m,1h,2h,4h,6h,8h,12h,1d",
			expected: "1m,5m,15m,30m,1h,2h,4h,6h,8h,12h,1d",
			note:     "支持多個時間框架",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			trader := &TraderRecord{
				ID:                  "trader-edge-" + tc.name,
				UserID:              userID,
				Name:                tc.name,
				AIModelID:           aiModelID,
				ExchangeID:          exchangeID,
				InitialBalance:      1000.0,
				ScanIntervalMinutes: 60,
				Timeframes:          tc.tf,
			}

			err := db.CreateTrader(trader)
			if err != nil {
				t.Fatalf("創建失敗: %v", err)
			}

			traders, _ := db.GetTraders(userID)
			var result string
			for _, tr := range traders {
				if tr.ID == trader.ID {
					result = tr.Timeframes
					break
				}
			}

			if result != tc.expected {
				t.Errorf("預期 '%s', 實際 '%s'", tc.expected, result)
			}

			t.Logf("✅ %s: '%s' → '%s' (%s)", tc.name, tc.tf, result, tc.note)
		})
	}
}

// Helper functions

func setupTestDBForTimeframes(t *testing.T) (*Database, func()) {
	tmpFile := t.TempDir() + "/test_timeframes.db"

	db, err := NewDatabase(tmpFile)
	if err != nil {
		t.Fatalf("創建測試數據庫失敗: %v", err)
	}

	// 創建測試用戶
	testUsers := []string{
		"test-user-tf-001", "test-user-tf-002",
		"test-user-tf-003", "test-user-tf-004",
	}

	for _, userID := range testUsers {
		user := &User{
			ID:           userID,
			Email:        userID + "@test.com",
			PasswordHash: "hash",
			OTPSecret:    "",
			OTPVerified:  false,
		}
		_ = db.CreateUser(user)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpFile)
	}

	return db, cleanup
}

// setupAIModelAndExchange 創建測試用的 AI model 和 exchange，返回它們的 integer ID
func setupAIModelAndExchange(t *testing.T, db *Database, userID string) (int, int) {
	// 創建 AI model (id="deepseek", enabled=true, no custom configs)
	err := db.UpdateAIModel(userID, "deepseek", true, "", "", "")
	if err != nil {
		t.Fatalf("創建 AI model 失敗: %v", err)
	}

	// 創建 exchange
	err = db.UpdateExchange(userID, "binance", true, "test-key", "test-secret", false, "", "", "", "")
	if err != nil {
		t.Fatalf("創建 exchange 失敗: %v", err)
	}

	// 獲取 AI model ID
	aiModels, err := db.GetAIModels(userID)
	if err != nil || len(aiModels) == 0 {
		t.Fatalf("獲取 AI model 失敗: %v", err)
	}
	aiModelID := aiModels[0].ID

	// 獲取 exchange ID
	exchanges, err := db.GetExchanges(userID)
	if err != nil || len(exchanges) == 0 {
		t.Fatalf("獲取 exchange 失敗: %v", err)
	}
	exchangeID := exchanges[0].ID

	return aiModelID, exchangeID
}
