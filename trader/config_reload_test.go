package trader

import (
	"nofx/config"
	"testing"
)

// TestReloadAIModelConfig 测试AI模型配置热更新
func TestReloadAIModelConfig(t *testing.T) {
	// 1. 创建初始配置
	initialConfig := AutoTraderConfig{
		ID:              "test_trader",
		Name:            "Test Trader",
		AIModel:         "deepseek",
		CustomModelName: "deepseek", // 初始错误的配置
		CustomAPIKey:    "test-key-123",
		CustomAPIURL:    "https://api.deepseek.com/v1",
		InitialBalance:  1000.0,
	}

	trader, err := NewAutoTrader(initialConfig, nil, "test_user")
	if err != nil {
		t.Fatalf("创建AutoTrader失败: %v", err)
	}

	// 验证初始配置
	if trader.config.CustomModelName != "deepseek" {
		t.Errorf("初始配置错误: 期望 'deepseek', 实际 '%s'", trader.config.CustomModelName)
	}

	// 2. 模拟数据库中的新配置
	newModelConfig := &config.AIModelConfig{
		UserID:          "admin",
		Provider:        "deepseek",
		Enabled:         true,
		APIKey:          "test-key-456", // 新的API Key
		CustomAPIURL:    "https://api.deepseek.com/v1",
		CustomModelName: "deepseek-chat", // 修正后的模型名称
	}

	// 3. 执行配置重载
	err = trader.ReloadAIModelConfig(newModelConfig)
	if err != nil {
		t.Fatalf("重载配置失败: %v", err)
	}

	// 4. 验证配置已更新
	if trader.config.CustomModelName != "deepseek-chat" {
		t.Errorf("配置未更新: 期望 'deepseek-chat', 实际 '%s'", trader.config.CustomModelName)
	}

	if trader.config.CustomAPIKey != "test-key-456" {
		t.Errorf("API Key未更新: 期望 'test-key-456', 实际 '%s'", trader.config.CustomAPIKey)
	}
}

// TestReloadAIModelConfig_EmptyModelName 测试当custom_model_name为空时使用provider
func TestReloadAIModelConfig_EmptyModelName(t *testing.T) {
	initialConfig := AutoTraderConfig{
		ID:              "test_trader",
		Name:            "Test Trader",
		AIModel:         "deepseek",
		CustomModelName: "deepseek-chat",
		InitialBalance:  1000.0,
	}

	trader, err := NewAutoTrader(initialConfig, nil, "test_user")
	if err != nil {
		t.Fatalf("创建AutoTrader失败: %v", err)
	}

	// 模拟数据库配置：custom_model_name为空
	newModelConfig := &config.AIModelConfig{
		UserID:          "admin",
		Provider:        "deepseek",
		Enabled:         true,
		APIKey:          "test-key",
		CustomAPIURL:    "https://api.deepseek.com/v1",
		CustomModelName: "", // 空字符串，应该使用provider
	}

	err = trader.ReloadAIModelConfig(newModelConfig)
	if err != nil {
		t.Fatalf("重载配置失败: %v", err)
	}

	// 当CustomModelName为空时，应该使用provider
	if trader.config.CustomModelName != "" {
		t.Errorf("CustomModelName应该保持为空（使用provider），实际 '%s'", trader.config.CustomModelName)
	}
}

// TestReloadAIModelConfig_QwenModel 测试Qwen模型配置更新
func TestReloadAIModelConfig_QwenModel(t *testing.T) {
	initialConfig := AutoTraderConfig{
		ID:              "test_trader",
		Name:            "Test Trader",
		AIModel:         "qwen",
		CustomModelName: "qwen-max",
		QwenKey:         "old-qwen-key",
		InitialBalance:  1000.0,
	}

	trader, err := NewAutoTrader(initialConfig, nil, "test_user")
	if err != nil {
		t.Fatalf("创建AutoTrader失败: %v", err)
	}

	// 更新Qwen配置
	newModelConfig := &config.AIModelConfig{
		UserID:          "admin",
		Provider:        "qwen",
		Enabled:         true,
		APIKey:          "new-qwen-key",
		CustomAPIURL:    "",
		CustomModelName: "qwen-plus",
	}

	err = trader.ReloadAIModelConfig(newModelConfig)
	if err != nil {
		t.Fatalf("重载配置失败: %v", err)
	}

	// 验证Qwen配置更新
	if trader.config.CustomModelName != "qwen-plus" {
		t.Errorf("Qwen模型名称未更新: 期望 'qwen-plus', 实际 '%s'", trader.config.CustomModelName)
	}

	if trader.config.QwenKey != "new-qwen-key" {
		t.Errorf("Qwen API Key未更新: 期望 'new-qwen-key', 实际 '%s'", trader.config.QwenKey)
	}
}

// TestReloadAIModelConfig_PreservesOtherConfig 测试配置更新不影响其他配置项
func TestReloadAIModelConfig_PreservesOtherConfig(t *testing.T) {
	initialConfig := AutoTraderConfig{
		ID:              "test_trader",
		Name:            "Test Trader",
		AIModel:         "deepseek",
		CustomModelName: "deepseek",
		InitialBalance:  1000.0,
		BTCETHLeverage:  5,
		AltcoinLeverage: 3,
		IsCrossMargin:   true,
		DefaultCoins:    []string{"BTC", "ETH"},
	}

	trader, err := NewAutoTrader(initialConfig, nil, "test_user")
	if err != nil {
		t.Fatalf("创建AutoTrader失败: %v", err)
	}

	// 只更新AI模型配置
	newModelConfig := &config.AIModelConfig{
		UserID:          "admin",
		Provider:        "deepseek",
		Enabled:         true,
		APIKey:          "new-key",
		CustomModelName: "deepseek-chat",
	}

	err = trader.ReloadAIModelConfig(newModelConfig)
	if err != nil {
		t.Fatalf("重载配置失败: %v", err)
	}

	// 验证其他配置项未被修改
	if trader.config.InitialBalance != 1000.0 {
		t.Errorf("InitialBalance被修改: 期望 1000.0, 实际 %f", trader.config.InitialBalance)
	}

	if trader.config.BTCETHLeverage != 5 {
		t.Errorf("BTCETHLeverage被修改: 期望 5, 实际 %d", trader.config.BTCETHLeverage)
	}

	if trader.config.IsCrossMargin != true {
		t.Errorf("IsCrossMargin被修改: 期望 true, 实际 %v", trader.config.IsCrossMargin)
	}

	if len(trader.config.DefaultCoins) != 2 {
		t.Errorf("DefaultCoins被修改: 期望长度 2, 实际 %d", len(trader.config.DefaultCoins))
	}
}
