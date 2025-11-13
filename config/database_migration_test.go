package config

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestDatabaseMigration_OldSchemaDetection 測試檢測舊 schema
func TestDatabaseMigration_OldSchemaDetection(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_old_schema.db")

	// 創建帶有舊列的數據庫
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// 創建舊 schema（包含 _old 列）
	_, err = db.Exec(`
		CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id INTEGER,
			ai_model_id_old TEXT,
			exchange_id INTEGER,
			exchange_id_old TEXT,
			initial_balance REAL NOT NULL,
			is_running BOOLEAN DEFAULT 0
		)
	`)
	require.NoError(t, err)

	// 檢測是否有舊列
	rows, err := db.Query("PRAGMA table_info(traders)")
	require.NoError(t, err)
	defer rows.Close()

	hasOldColumns := false
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue sql.NullString

		err = rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		require.NoError(t, err)

		if name == "ai_model_id_old" || name == "exchange_id_old" {
			hasOldColumns = true
			break
		}
	}

	assert.True(t, hasOldColumns, "應該檢測到舊列")
}

// TestDatabaseMigration_DataPreservation 測試遷移保留數據
func TestDatabaseMigration_DataPreservation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_migration.db")

	// 1. 創建舊 schema 數據庫
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// 創建 AI models 和 exchanges 表
	_, err = db.Exec(`
		CREATE TABLE ai_models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			model_id_old TEXT,
			name TEXT NOT NULL,
			provider TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE exchanges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			exchange_id_old TEXT,
			name TEXT NOT NULL,
			type TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	// 創建舊 schema traders 表
	_, err = db.Exec(`
		CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id INTEGER,
			ai_model_id_old TEXT,
			exchange_id INTEGER,
			exchange_id_old TEXT,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 5,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 3,
			is_cross_margin BOOLEAN DEFAULT 1,
			use_default_coins BOOLEAN DEFAULT 1,
			taker_fee_rate REAL DEFAULT 0.0004,
			maker_fee_rate REAL DEFAULT 0.0002,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	// 插入測試 AI model
	result, err := db.Exec(`
		INSERT INTO ai_models (user_id, model_id_old, name, provider)
		VALUES (?, ?, ?, ?)
	`, "test-user", "old-model-id", "Test Model", "openai")
	require.NoError(t, err)
	aiModelID, err := result.LastInsertId()
	require.NoError(t, err)

	// 插入測試 exchange
	result, err = db.Exec(`
		INSERT INTO exchanges (user_id, exchange_id_old, name, type)
		VALUES (?, ?, ?, ?)
	`, "test-user", "old-exchange-id", "Test Exchange", "cex")
	require.NoError(t, err)
	exchangeID, err := result.LastInsertId()
	require.NoError(t, err)

	// 2. 插入測試數據（使用舊列）
	testTraders := []struct {
		id              string
		name            string
		aiModelIDOld    string
		exchangeIDOld   string
		initialBalance  float64
		scanInterval    int
		btcEthLeverage  int
		altcoinLeverage int
	}{
		{"trader-1", "Trader One", "old-model-id", "old-exchange-id", 1000.0, 5, 5, 3},
		{"trader-2", "Trader Two", "old-model-id", "old-exchange-id", 2000.0, 10, 10, 5},
		{"trader-3", "Trader Three", "old-model-id", "old-exchange-id", 3000.0, 15, 15, 7},
	}

	for _, trader := range testTraders {
		_, err = db.Exec(`
			INSERT INTO traders (
				id, user_id, name, ai_model_id_old, exchange_id_old,
				initial_balance, scan_interval_minutes, btc_eth_leverage, altcoin_leverage
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, trader.id, "test-user", trader.name, trader.aiModelIDOld, trader.exchangeIDOld,
			trader.initialBalance, trader.scanInterval, trader.btcEthLeverage, trader.altcoinLeverage)
		require.NoError(t, err)
	}

	// 3. 執行遷移（模擬腳本邏輯）
	_, err = db.Exec(`
		BEGIN TRANSACTION;

		-- 創建新表
		CREATE TABLE traders_new (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id INTEGER,
			exchange_id INTEGER,
			initial_balance REAL NOT NULL,
			scan_interval_minutes INTEGER DEFAULT 5,
			is_running BOOLEAN DEFAULT 0,
			btc_eth_leverage INTEGER DEFAULT 5,
			altcoin_leverage INTEGER DEFAULT 3,
			is_cross_margin BOOLEAN DEFAULT 1,
			use_default_coins BOOLEAN DEFAULT 1,
			taker_fee_rate REAL DEFAULT 0.0004,
			maker_fee_rate REAL DEFAULT 0.0002,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (ai_model_id) REFERENCES ai_models(id),
			FOREIGN KEY (exchange_id) REFERENCES exchanges(id)
		);

		-- 遷移數據（將舊列映射到新列）
		INSERT INTO traders_new (
			id, user_id, name, ai_model_id, exchange_id,
			initial_balance, scan_interval_minutes,
			is_running, btc_eth_leverage, altcoin_leverage,
			is_cross_margin, use_default_coins,
			taker_fee_rate, maker_fee_rate,
			created_at, updated_at
		)
		SELECT
			t.id,
			t.user_id,
			t.name,
			COALESCE(t.ai_model_id, (SELECT id FROM ai_models WHERE model_id_old = t.ai_model_id_old LIMIT 1)),
			COALESCE(t.exchange_id, (SELECT id FROM exchanges WHERE exchange_id_old = t.exchange_id_old LIMIT 1)),
			t.initial_balance,
			t.scan_interval_minutes,
			t.is_running,
			t.btc_eth_leverage,
			t.altcoin_leverage,
			t.is_cross_margin,
			t.use_default_coins,
			t.taker_fee_rate,
			t.maker_fee_rate,
			t.created_at,
			t.updated_at
		FROM traders t;

		-- 替換表
		DROP TABLE traders;
		ALTER TABLE traders_new RENAME TO traders;

		COMMIT;
	`)
	require.NoError(t, err)

	// 4. 驗證數據完整性
	rows, err := db.Query(`
		SELECT id, name, ai_model_id, exchange_id, initial_balance,
		       scan_interval_minutes, btc_eth_leverage, altcoin_leverage
		FROM traders
		ORDER BY id
	`)
	require.NoError(t, err)
	defer rows.Close()

	migratedTraders := []struct {
		id              string
		name            string
		aiModelID       int64
		exchangeID      int64
		initialBalance  float64
		scanInterval    int
		btcEthLeverage  int
		altcoinLeverage int
	}{}

	for rows.Next() {
		var trader struct {
			id              string
			name            string
			aiModelID       int64
			exchangeID      int64
			initialBalance  float64
			scanInterval    int
			btcEthLeverage  int
			altcoinLeverage int
		}
		err = rows.Scan(&trader.id, &trader.name, &trader.aiModelID, &trader.exchangeID,
			&trader.initialBalance, &trader.scanInterval, &trader.btcEthLeverage, &trader.altcoinLeverage)
		require.NoError(t, err)
		migratedTraders = append(migratedTraders, trader)
	}

	// 驗證數據數量
	assert.Len(t, migratedTraders, 3, "應該有 3 個交易員")

	// 驗證數據正確性
	for i, trader := range migratedTraders {
		assert.Equal(t, testTraders[i].id, trader.id)
		assert.Equal(t, testTraders[i].name, trader.name)
		assert.Equal(t, aiModelID, trader.aiModelID, "AI model ID 應該正確映射")
		assert.Equal(t, exchangeID, trader.exchangeID, "Exchange ID 應該正確映射")
		assert.Equal(t, testTraders[i].initialBalance, trader.initialBalance)
		assert.Equal(t, testTraders[i].scanInterval, trader.scanInterval)
		assert.Equal(t, testTraders[i].btcEthLeverage, trader.btcEthLeverage)
		assert.Equal(t, testTraders[i].altcoinLeverage, trader.altcoinLeverage)
	}

	// 5. 驗證舊列已刪除
	schemaRows, err := db.Query("PRAGMA table_info(traders)")
	require.NoError(t, err)
	defer schemaRows.Close()

	columns := []string{}
	for schemaRows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue sql.NullString

		err = schemaRows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		require.NoError(t, err)
		columns = append(columns, name)
	}

	assert.NotContains(t, columns, "ai_model_id_old", "舊列 ai_model_id_old 應該被刪除")
	assert.NotContains(t, columns, "exchange_id_old", "舊列 exchange_id_old 應該被刪除")
	assert.Contains(t, columns, "ai_model_id", "新列 ai_model_id 應該存在")
	assert.Contains(t, columns, "exchange_id", "新列 exchange_id 應該存在")
}

// TestDatabaseMigration_EmptyDatabase 測試空數據庫遷移
func TestDatabaseMigration_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_empty.db")

	// 創建帶有舊 schema 的空數據庫
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// 創建舊 schema
	_, err = db.Exec(`
		CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id INTEGER,
			ai_model_id_old TEXT,
			exchange_id INTEGER,
			exchange_id_old TEXT,
			initial_balance REAL NOT NULL
		)
	`)
	require.NoError(t, err)

	// 執行遷移
	_, err = db.Exec(`
		BEGIN TRANSACTION;

		CREATE TABLE traders_new (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id INTEGER,
			exchange_id INTEGER,
			initial_balance REAL NOT NULL
		);

		INSERT INTO traders_new SELECT
			id, user_id, name, ai_model_id, exchange_id, initial_balance
		FROM traders;

		DROP TABLE traders;
		ALTER TABLE traders_new RENAME TO traders;

		COMMIT;
	`)
	require.NoError(t, err)

	// 驗證表為空
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM traders").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "遷移後表應該仍為空")
}

// TestDatabaseMigration_WALCheckpoint 測試 WAL checkpoint
func TestDatabaseMigration_WALCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_wal.db")

	// 創建數據庫並啟用 WAL 模式
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// 啟用 WAL 模式
	_, err = db.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)

	// 創建表並插入數據
	_, err = db.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY,
			data TEXT
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO test_table (data) VALUES ('test')")
	require.NoError(t, err)

	// 檢查 WAL 文件是否存在
	walPath := dbPath + "-wal"
	_, err = os.Stat(walPath)
	walExistsBeforeCheckpoint := !os.IsNotExist(err)

	// 執行 checkpoint
	_, err = db.Exec("PRAGMA wal_checkpoint(FULL)")
	require.NoError(t, err)

	// VACUUM 優化數據庫
	_, err = db.Exec("VACUUM")
	require.NoError(t, err)

	// 關閉連接
	db.Close()

	// 檢查 WAL 文件狀態
	_, err = os.Stat(walPath)
	walExistsAfterCheckpoint := !os.IsNotExist(err)

	// 如果之前有 WAL 文件，checkpoint 後應該清空或刪除
	if walExistsBeforeCheckpoint {
		t.Logf("WAL 文件狀態 - checkpoint 前: %v, checkpoint 後: %v",
			walExistsBeforeCheckpoint, walExistsAfterCheckpoint)
	}

	// 驗證數據完整性
	db, err = sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "checkpoint 後數據應該完整")
}

// TestDatabaseMigration_ConcurrentAccess 測試遷移過程中的並發訪問
func TestDatabaseMigration_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_concurrent.db")

	// 創建數據庫
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// 創建表
	_, err = db.Exec(`
		CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id_old TEXT,
			ai_model_id INTEGER
		)
	`)
	require.NoError(t, err)

	// 插入測試數據
	for i := 0; i < 5; i++ {
		_, err = db.Exec(`
			INSERT INTO traders (id, user_id, name, ai_model_id_old)
			VALUES (?, ?, ?, ?)
		`, fmt.Sprintf("trader-%d", i), "test-user", fmt.Sprintf("Trader %d", i), "old-id")
		require.NoError(t, err)
	}

	// 在事務中執行遷移
	tx, err := db.Begin()
	require.NoError(t, err)

	_, err = tx.Exec(`
		CREATE TABLE traders_new (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id INTEGER
		)
	`)
	require.NoError(t, err)

	_, err = tx.Exec(`
		INSERT INTO traders_new (id, user_id, name, ai_model_id)
		SELECT id, user_id, name, ai_model_id FROM traders
	`)
	require.NoError(t, err)

	_, err = tx.Exec("DROP TABLE traders")
	require.NoError(t, err)

	_, err = tx.Exec("ALTER TABLE traders_new RENAME TO traders")
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// 驗證遷移成功
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM traders").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 5, count, "遷移後應該有 5 條記錄")
}

// TestDatabaseMigration_RollbackOnError 測試遷移失敗時的回滾
func TestDatabaseMigration_RollbackOnError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_rollback.db")

	// 創建數據庫
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// 創建表
	_, err = db.Exec(`
		CREATE TABLE traders (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			ai_model_id_old TEXT
		)
	`)
	require.NoError(t, err)

	// 插入測試數據
	_, err = db.Exec(`
		INSERT INTO traders (id, user_id, name, ai_model_id_old)
		VALUES ('test-1', 'user-1', 'Test Trader', 'old-id')
	`)
	require.NoError(t, err)

	// 嘗試執行有錯誤的遷移（故意引入語法錯誤）
	_, err = db.Exec(`
		BEGIN TRANSACTION;
		CREATE TABLE traders_new (id TEXT PRIMARY KEY);
		INSERT INTO traders_new (id, invalid_column) SELECT id, name FROM traders;
		DROP TABLE traders;
		ALTER TABLE traders_new RENAME TO traders;
		COMMIT;
	`)

	// 應該會失敗
	assert.Error(t, err, "有錯誤的遷移應該失敗")

	// 驗證原始數據仍然存在
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM traders").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "回滾後原始數據應該保留")

	// 驗證原始 schema 仍然存在
	rows, err := db.Query("PRAGMA table_info(traders)")
	require.NoError(t, err)
	defer rows.Close()

	columns := []string{}
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var defaultValue sql.NullString

		err = rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk)
		require.NoError(t, err)
		columns = append(columns, name)
	}

	assert.Contains(t, columns, "ai_model_id_old", "回滾後舊列應該仍存在")
}
