package config

import (
	"fmt"
	"os"
	"testing"
)

// ============================================================================
// WAL Mode Tests - 从 upstream PR #817 移植
// ============================================================================

// setupTestDB 创建测试数据库
func setupTestDB(t *testing.T) (*Database, func()) {
	// 创建临时数据库文件
	tmpFile := t.TempDir() + "/test.db"

	db, err := NewDatabase(tmpFile)
	if err != nil {
		t.Fatalf("创建测试数据库失败: %v", err)
	}

	// 创建测试用户
	testUsers := []string{"test-user-001", "test-user-002"}
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

// TestWALModeEnabled 测试 WAL 模式是否启用
// TDD: 验证 NewDatabase 正确启用 WAL 模式
func TestWALModeEnabled(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// 查询当前的 journal_mode
	var journalMode string
	err := db.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("查询 journal_mode 失败: %v", err)
	}

	// 期望是 WAL 模式
	if journalMode != "wal" {
		t.Errorf("期望 journal_mode=wal，实际是 %s", journalMode)
	}
}

// TestSynchronousMode 测试 synchronous 模式设置
// TDD: 验证数据持久性设置
func TestSynchronousMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// 查询 synchronous 设置
	var synchronous int
	err := db.db.QueryRow("PRAGMA synchronous").Scan(&synchronous)
	if err != nil {
		t.Fatalf("查询 synchronous 失败: %v", err)
	}

	// 期望是 FULL (2) 以确保数据持久性
	if synchronous != 2 {
		t.Errorf("期望 synchronous=2 (FULL)，实际是 %d", synchronous)
	}
}

// TestDataPersistenceAcrossReopen 测试数据在数据库关闭并重新打开后是否持久化
// TDD: 模拟 Docker restart 场景
func TestDataPersistenceAcrossReopen(t *testing.T) {
	// 创建临时数据库文件
	tmpFile, err := os.CreateTemp("", "test_persistence_*.db")
	if err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	testTraderID := "test-trader-001"
	testUsername := "test-persistence-user"

	// 第一次打开数据库并写入数据
	{
		db, err := NewDatabase(dbPath)
		if err != nil {
			t.Fatalf("第一次创建数据库失败: %v", err)
		}

		// 创建用户
		user := &User{
			ID:           testUsername,
			Email:        testUsername + "@test.com",
			PasswordHash: "hash",
			OTPSecret:    "",
			OTPVerified:  false,
		}
		if err := db.CreateUser(user); err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}

		// 写入交易员配置
		trader := &TraderRecord{
			ID:             testTraderID,
			UserID:         testUsername,
			Name:           "Test Trader",
			AIModelID:      "deepseek",
			ExchangeID:     "binance",
			InitialBalance: 1000.0,
			IsRunning:      false,
		}
		if err := db.CreateTrader(trader); err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}

		// 模拟正常关闭
		if err := db.Close(); err != nil {
			t.Fatalf("关闭数据库失败: %v", err)
		}
	}

	// 第二次打开数据库并验证数据是否还在
	{
		db, err := NewDatabase(dbPath)
		if err != nil {
			t.Fatalf("第二次打开数据库失败: %v", err)
		}
		defer db.Close()

		// 读取数据
		traders, err := db.GetTraders(testUsername)
		if err != nil {
			t.Fatalf("读取数据失败: %v", err)
		}

		if len(traders) == 0 {
			t.Fatal("数据丢失：没有找到任何交易员配置")
		}

		// 验证数据完整性
		found := false
		for _, trader := range traders {
			if trader.ID == testTraderID {
				found = true
				if trader.Name != "Test Trader" {
					t.Errorf("Trader Name 丢失或损坏，期望 %s，实际 %s", "Test Trader", trader.Name)
				}
				if trader.InitialBalance != 1000.0 {
					t.Errorf("Initial Balance 丢失或损坏，期望 %.2f，实际 %.2f", 1000.0, trader.InitialBalance)
				}
			}
		}

		if !found {
			t.Error("数据丢失：找不到 trader 配置")
		}
	}
}

// TestConcurrentWritesWithWAL 测试 WAL 模式下的并发写入
// TDD: WAL 模式应该支持更好的并发性能
func TestConcurrentWritesWithWAL(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// 这个测试验证多个并发写入可以成功
	// WAL 模式下并发性能更好,但 SQLite 仍然可能出现短暂的锁
	done := make(chan bool, 2)
	errors := make(chan error, 10)

	// 并发写入1：系统配置
	go func() {
		for i := 0; i < 5; i++ {
			key := "test_key_1"
			value := fmt.Sprintf("value_%d", i)
			err := db.SetSystemConfig(key, value)
			if err != nil {
				errors <- err
			}
		}
		done <- true
	}()

	// 并发写入2：系统配置
	go func() {
		for i := 0; i < 5; i++ {
			key := "test_key_2"
			value := fmt.Sprintf("value_%d", i)
			err := db.SetSystemConfig(key, value)
			if err != nil {
				errors <- err
			}
		}
		done <- true
	}()

	// 等待两个 goroutine 完成
	<-done
	<-done
	close(errors)

	// 检查是否有错误
	errorCount := 0
	for err := range errors {
		t.Logf("并发写入错误: %v", err)
		errorCount++
	}

	// WAL 模式下应该能处理并发,但可能有少量锁错误
	// 我们允许最多 2 个错误
	if errorCount > 2 {
		t.Errorf("并发写入失败次数过多: %d", errorCount)
	}
}
