package manager

import (
	"nofx/trader"
	"testing"
	"time"
)

// TestRemoveTrader 测试从内存中移除trader
func TestRemoveTrader(t *testing.T) {
	tm := NewTraderManager()

	// 创建一个真实的 AutoTrader 实例
	traderID := "test-trader-123"
	cfg := trader.AutoTraderConfig{
		ID:             traderID,
		Name:           "Test Trader",
		InitialBalance: 1000,
		ScanInterval:   1 * time.Minute,
	}
	at, _ := trader.NewAutoTrader(cfg, nil, "user1")

	tm.traders[traderID] = at

	// 验证 trader 存在
	if !tm.HasTrader(traderID) {
		t.Fatal("trader 应该存在于 map 中")
	}

	// 调用 RemoveTrader
	tm.RemoveTrader(traderID)

	// 验证 trader 已被移除
	if tm.HasTrader(traderID) {
		t.Error("trader 应该已从 map 中移除")
	}
}

// TestRemoveTrader_StopsRunningTrader 测试移除正在运行的 trader 时会自动停止它
func TestRemoveTrader_StopsRunningTrader(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-running"

	// 创建一个真实的 AutoTrader 实例
	cfg := trader.AutoTraderConfig{
		ID:             traderID,
		Name:           "Test Running Trader",
		InitialBalance: 1000,
		ScanInterval:   100 * time.Millisecond, // 短间隔
	}
	at, _ := trader.NewAutoTrader(cfg, nil, "user1")

	tm.traders[traderID] = at

	// 模拟启动 Trader (手动设置状态)
	// 注意：真正的 Run() 是阻塞循环，我们在测试中可以通过 hack 或者 wrapper 来模拟运行状态，
	// 但最准确的是在一个 goroutine 中运行它，然后验证 Stop() 是否能让它退出。
	// 这里我们利用 AutoTrader 的特性：Run() 会设置 isRunning=true，Stop() 会设置 isRunning=false。

	// 启动一个 goroutine 运行 trader
	go func() {
		at.Run()
	}()

	// 等待启动完成 (简单等待，或者可以用更复杂的同步机制)
	time.Sleep(50 * time.Millisecond)

	// 验证正在运行
	status := at.GetStatus()
	if isRunning, ok := status["is_running"].(bool); !ok || !isRunning {
		t.Fatal("Trader 应该是运行状态")
	}

	// 调用 RemoveTrader
	// 期望：RemoveTrader 会调用 at.Stop()，这将导致 at.Run() 循环退出，并设置 isRunning=false
	tm.RemoveTrader(traderID)

	// 验证 trader 已被移除
	if _, exists := tm.traders[traderID]; exists {
		t.Error("trader 应该已从 map 中移除")
	}

	// 验证 trader 已停止
	// Stop() 是阻塞等待 goroutine 结束的，所以这里应该已经停止
	statusAfter := at.GetStatus()
	if isRunning, ok := statusAfter["is_running"].(bool); ok && isRunning {
		t.Error("Trader 应该已经被停止")
	}
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

	// 创建一个真实的 AutoTrader 实例用于并发测试
	cfg := trader.AutoTraderConfig{
		ID:             traderID,
		Name:           "Test Concurrent Trader",
		InitialBalance: 1000,
		ScanInterval:   1 * time.Minute,
	}
	at, _ := trader.NewAutoTrader(cfg, nil, "user1")
	tm.traders[traderID] = at

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
	if tm.HasTrader(traderID) {
		t.Error("trader 应该已从 map 中移除")
	}
}

// TestGetTrader_AfterRemove 测试移除后获取trader返回错误
func TestGetTrader_AfterRemove(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-get"

	// 创建一个真实的 AutoTrader 实例
	cfg := trader.AutoTraderConfig{
		ID:             traderID,
		Name:           "Test Get Trader",
		InitialBalance: 1000,
		ScanInterval:   1 * time.Minute,
	}
	at, _ := trader.NewAutoTrader(cfg, nil, "user1")
	tm.traders[traderID] = at

	// 移除 trader
	tm.RemoveTrader(traderID)

	// 尝试获取已移除的 trader
	_, err := tm.GetTrader(traderID)
	if err == nil {
		t.Error("获取已移除的 trader 应该返回错误")
	}
}

// TestHasTrader 测试HasTrader方法
func TestHasTrader(t *testing.T) {
	tm := NewTraderManager()
	traderID := "test-trader-has"

	// 初始状态：trader 不存在
	if tm.HasTrader(traderID) {
		t.Error("trader 不应该存在")
	}

	// 创建一个真实的 AutoTrader 实例
	cfg := trader.AutoTraderConfig{
		ID:             traderID,
		Name:           "Test Has Trader",
		InitialBalance: 1000,
		ScanInterval:   1 * time.Minute,
	}
	at, _ := trader.NewAutoTrader(cfg, nil, "user1")
	tm.traders[traderID] = at

	// 验证 trader 存在
	if !tm.HasTrader(traderID) {
		t.Error("trader 应该存在")
	}

	// 移除 trader
	tm.RemoveTrader(traderID)

	// 验证 trader 不存在
	if tm.HasTrader(traderID) {
		t.Error("trader 不应该存在")
	}
}
