package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"nofx/api"
	"nofx/auth"
	"nofx/config"
	"nofx/crypto"
	"nofx/manager"
	"nofx/market"
	"nofx/pool"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

// ConfigFile 配置文件结构，只包含需要同步到数据库的字段
// TODO 现在与config.Config相同，未来会被替换， 现在为了兼容性不得不保留当前文件
type ConfigFile struct {
	BetaMode           bool                  `json:"beta_mode"`
	APIServerPort      int                   `json:"api_server_port"`
	UseDefaultCoins    bool                  `json:"use_default_coins"`
	DefaultCoins       []string              `json:"default_coins"`
	CoinPoolAPIURL     string                `json:"coin_pool_api_url"`
	OITopAPIURL        string                `json:"oi_top_api_url"`
	MaxDailyLoss       float64               `json:"max_daily_loss"`
	MaxDrawdown        float64               `json:"max_drawdown"`
	StopTradingMinutes int                   `json:"stop_trading_minutes"`
	Leverage           config.LeverageConfig `json:"leverage"`
	JWTSecret          string                `json:"jwt_secret"`
	DataKLineTime      string                `json:"data_k_line_time"`
	Log                *config.LogConfig     `json:"log"` // 日志配置
}

// loadConfigFile 读取并解析config.json文件
func loadConfigFile() (*ConfigFile, error) {
	// 检查config.json是否存在
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		log.Printf("📄 config.json不存在，使用默认配置")
		return nil, nil
	}

	// 读取config.json
	data, err := os.ReadFile("config.json")
	if err != nil {
		return nil, fmt.Errorf("读取config.json失败: %w", err)
	}

	// 解析JSON
	var configFile ConfigFile
	if err := json.Unmarshal(data, &configFile); err != nil {
		return nil, fmt.Errorf("解析config.json失败: %w", err)
	}

	return &configFile, nil
}

// syncConfigToDatabase 将配置同步到数据库
func syncConfigToDatabase(database *config.Database, configFile *ConfigFile) error {
	if configFile == nil {
		return nil
	}

	log.Printf("🔄 开始同步config.json到数据库...")

	// 同步各配置项到数据库
	configs := map[string]string{
		"beta_mode":            fmt.Sprintf("%t", configFile.BetaMode),
		"api_server_port":      strconv.Itoa(configFile.APIServerPort),
		"use_default_coins":    fmt.Sprintf("%t", configFile.UseDefaultCoins),
		"coin_pool_api_url":    configFile.CoinPoolAPIURL,
		"oi_top_api_url":       configFile.OITopAPIURL,
		"max_daily_loss":       fmt.Sprintf("%.1f", configFile.MaxDailyLoss),
		"max_drawdown":         fmt.Sprintf("%.1f", configFile.MaxDrawdown),
		"stop_trading_minutes": strconv.Itoa(configFile.StopTradingMinutes),
	}

	// 同步default_coins（转换为JSON字符串存储）
	if len(configFile.DefaultCoins) > 0 {
		defaultCoinsJSON, err := json.Marshal(configFile.DefaultCoins)
		if err == nil {
			configs["default_coins"] = string(defaultCoinsJSON)
		}
	}

	// 同步杠杆配置
	if configFile.Leverage.BTCETHLeverage > 0 {
		configs["btc_eth_leverage"] = strconv.Itoa(configFile.Leverage.BTCETHLeverage)
	}
	if configFile.Leverage.AltcoinLeverage > 0 {
		configs["altcoin_leverage"] = strconv.Itoa(configFile.Leverage.AltcoinLeverage)
	}

	// 如果JWT密钥不为空，也同步
	if configFile.JWTSecret != "" {
		configs["jwt_secret"] = configFile.JWTSecret
	}

	// 更新数据库配置
	for key, value := range configs {
		if err := database.SetSystemConfig(key, value); err != nil {
			log.Printf("⚠️  更新配置 %s 失败: %v", key, err)
		} else {
			log.Printf("✓ 同步配置: %s = %s", key, value)
		}
	}

	log.Printf("✅ config.json同步完成")
	return nil
}

// loadBetaCodesToDatabase 加载内测码文件到数据库
func loadBetaCodesToDatabase(database *config.Database) error {
	betaCodeFile := "beta_codes.txt"

	// 检查内测码文件是否存在
	if _, err := os.Stat(betaCodeFile); os.IsNotExist(err) {
		log.Printf("📄 内测码文件 %s 不存在，跳过加载", betaCodeFile)
		return nil
	}

	// 获取文件信息
	fileInfo, err := os.Stat(betaCodeFile)
	if err != nil {
		return fmt.Errorf("获取内测码文件信息失败: %w", err)
	}

	log.Printf("🔄 发现内测码文件 %s (%.1f KB)，开始加载...", betaCodeFile, float64(fileInfo.Size())/1024)

	// 加载内测码到数据库
	err = database.LoadBetaCodesFromFile(betaCodeFile)
	if err != nil {
		return fmt.Errorf("加载内测码失败: %w", err)
	}

	// 显示统计信息
	total, used, err := database.GetBetaCodeStats()
	if err != nil {
		log.Printf("⚠️  获取内测码统计失败: %v", err)
	} else {
		log.Printf("✅ 内测码加载完成: 总计 %d 个，已使用 %d 个，剩余 %d 个", total, used, total-used)
	}

	return nil
}

// validateSecurityConfig 验证安全配置
func validateSecurityConfig() error {
	// 检查 DATA_ENCRYPTION_KEY 环境变量
	dataKey := strings.TrimSpace(os.Getenv("DATA_ENCRYPTION_KEY"))
	if dataKey == "" {
		return fmt.Errorf("DATA_ENCRYPTION_KEY 环境变量未设置")
	}

	// 检查密钥长度（base64 编码的 32 字节至少需要 44 个字符）
	if len(dataKey) < 32 {
		return fmt.Errorf("DATA_ENCRYPTION_KEY 长度不足 (当前: %d, 最少: 32)", len(dataKey))
	}

	// 检查是否使用了示例密钥
	if strings.Contains(dataKey, "PLEASE_GENERATE") || strings.Contains(dataKey, "EXAMPLE") {
		return fmt.Errorf("检测到示例密钥，请生成真实密钥")
	}

	log.Printf("✅ 安全配置检查通过")
	return nil
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║    🤖 AI多模型交易系统 - 支持 DeepSeek & Qwen            ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load environment variables from .env file if present (for local/dev runs)
	// In Docker Compose, variables are injected by the runtime and this is harmless.
	_ = godotenv.Load()

	// 🔐 安全检查：验证必需的环境变量
	if err := validateSecurityConfig(); err != nil {
		log.Fatalf("❌ 安全配置检查失败: %v\n\n💡 请运行以下命令修复:\n   ./scripts/setup-env.sh\n", err)
	}

	// 初始化数据库配置
	dbPath := "config.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	// 读取配置文件
	configFile, err := loadConfigFile()
	if err != nil {
		log.Fatalf("❌ 读取config.json失败: %v", err)
	}

	log.Printf("📋 初始化配置数据库: %s", dbPath)
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ 初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 初始化加密服务
	log.Printf("🔐 初始化加密服务...")
	cryptoService, err := crypto.NewCryptoService("secrets/rsa_key")
	if err != nil {
		log.Fatalf("❌ 初始化加密服务失败: %v", err)
	}
	database.SetCryptoService(cryptoService)
	log.Printf("✅ 加密服务初始化成功")

	// 同步config.json到数据库
	if err := syncConfigToDatabase(database, configFile); err != nil {
		log.Printf("⚠️  同步config.json到数据库失败: %v", err)
	}

	// 加载内测码到数据库
	if err := loadBetaCodesToDatabase(database); err != nil {
		log.Printf("⚠️  加载内测码到数据库失败: %v", err)
	}

	// 获取系统配置
	useDefaultCoinsStr, _ := database.GetSystemConfig("use_default_coins")
	useDefaultCoins := useDefaultCoinsStr == "true"
	apiPortStr, _ := database.GetSystemConfig("api_server_port")

	// 设置JWT密钥（优先级：环境变量 > 数据库自动生成）
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret == "" {
		// 尝试从数据库获取（可能是之前自动生成的）
		jwtSecret, _ = database.GetSystemConfig("jwt_secret")
		if jwtSecret == "" {
			// 首次运行：自动生成随机密钥并保存到数据库
			randomBytes := make([]byte, 32)
			_, err := rand.Read(randomBytes)
			if err != nil {
				log.Fatal("❌ 生成随机 JWT 密钥失败:", err)
			}
			jwtSecret = base64.StdEncoding.EncodeToString(randomBytes)

			// 保存到数据库（持久化）
			err = database.SetSystemConfig("jwt_secret", jwtSecret)
			if err != nil {
				log.Fatal("❌ 保存 JWT 密钥到数据库失败:", err)
			}

			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("🔐 首次启动：已自动生成 JWT 密钥")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("")
			log.Println("✓ 密钥已安全保存到数据库 (config.db)")
			log.Println("✓ 重启服务后密钥仍然有效，用户无需重新登录")
			log.Println("")
			log.Println("📝 生产环境建议（可选）：")
			log.Println("  使用自定义密钥：export JWT_SECRET='your-secret'")
			log.Println("")
			log.Println("⚠️  备份提示：config.db 包含敏感数据，请妥善保管")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		} else {
			log.Printf("🔑 使用数据库中的 JWT 密钥")
		}
	} else {
		log.Printf("🔑 使用环境变量 JWT 密钥（优先级最高）")
	}
	auth.SetJWTSecret(jwtSecret)

	// 获取管理员模式配置（用於自動啟動功能）
	// 默認為 true，除非顯式設置為 "false"
	adminModeStr, _ := database.GetSystemConfig("admin_mode")
	adminMode := adminModeStr != "false"

	if adminMode {
		log.Printf("ℹ️  Admin mode: enabled (服務重啟時自動恢復運行中的 traders)")
	} else {
		log.Printf("ℹ️  Admin mode: disabled (手動啟動模式)")
	}

	log.Printf("✓ 配置数据库初始化成功")
	fmt.Println()

	// 从数据库读取默认主流币种列表
	defaultCoinsJSON, _ := database.GetSystemConfig("default_coins")
	var defaultCoins []string

	if defaultCoinsJSON != "" {
		// 尝试从JSON解析
		if err := json.Unmarshal([]byte(defaultCoinsJSON), &defaultCoins); err != nil {
			log.Printf("⚠️  解析default_coins配置失败: %v，使用硬编码默认值", err)
			defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		} else {
			log.Printf("✓ 从数据库加载默认币种列表（共%d个）: %v", len(defaultCoins), defaultCoins)
		}
	} else {
		// 如果数据库中没有配置，使用硬编码默认值
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
		log.Printf("⚠️  数据库中未配置default_coins，使用硬编码默认值")
	}

	pool.SetDefaultCoins(defaultCoins)
	// 设置是否使用默认主流币种
	pool.SetUseDefaultCoins(useDefaultCoins)
	if useDefaultCoins {
		log.Printf("✓ 已启用默认主流币种列表")
	}

	// 设置币种池API URL
	coinPoolAPIURL, _ := database.GetSystemConfig("coin_pool_api_url")
	if coinPoolAPIURL != "" {
		pool.SetCoinPoolAPI(coinPoolAPIURL)
		log.Printf("✓ 已配置AI500币种池API")
	}

	oiTopAPIURL, _ := database.GetSystemConfig("oi_top_api_url")
	if oiTopAPIURL != "" {
		pool.SetOITopAPI(oiTopAPIURL)
		log.Printf("✓ 已配置OI Top API")
	}

	// 创建TraderManager
	traderManager := manager.NewTraderManager()

	// 从数据库加载所有交易员到内存
	err = traderManager.LoadTradersFromDatabase(database)
	if err != nil {
		log.Fatalf("❌ 加载交易员失败: %v", err)
	}

	// 获取数据库中的所有交易员配置（用于显示，使用default用户）
	traders, err := database.GetTraders("default")
	if err != nil {
		log.Fatalf("❌ 获取交易员列表失败: %v", err)
	}

	// 显示加载的交易员信息
	fmt.Println()
	fmt.Println("🤖 数据库中的AI交易员配置:")
	if len(traders) == 0 {
		fmt.Println("  • 暂无配置的交易员，请通过Web界面创建")
	} else {
		for _, trader := range traders {
			status := "停止"
			if trader.IsRunning {
				status = "运行中"
			}
			fmt.Printf("  • %s (Model#%d + Exchange#%d) - 初始资金: %.0f USDT [%s]\n",
				trader.Name, trader.AIModelID, trader.ExchangeID,
				trader.InitialBalance, status)
		}
	}

	// 创建初始化上下文
	// TODO : 传入实际配置, 现在并未实际使用，未来所有模块初始化都将通过上下文传递配置
	// ctx := bootstrap.NewContext(&config.Config{})

	// // 执行所有初始化钩子
	// if err := bootstrap.Run(ctx); err != nil {
	// 	log.Fatalf("初始化失败: %v", err)
	// }

	fmt.Println()
	fmt.Println("🤖 AI全权决策模式:")
	fmt.Printf("  • AI将自主决定每笔交易的杠杆倍数（山寨币最高5倍，BTC/ETH最高5倍）\n")
	fmt.Println("  • AI将自主决定每笔交易的仓位大小")
	fmt.Println("  • AI将自主设置止损和止盈价格")
	fmt.Println("  • AI将基于市场数据、技术指标、账户状态做出全面分析")
	fmt.Println()
	fmt.Println("⚠️  风险提示: AI自动交易有风险，建议小额资金测试！")
	fmt.Println()
	fmt.Println("按 Ctrl+C 停止运行")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	// 获取API服务器端口（优先级：环境变量 > 数据库配置 > 默认值）
	apiPort := 8080 // 默认端口

	// 1. 优先从环境变量 NOFX_BACKEND_PORT 读取
	if envPort := strings.TrimSpace(os.Getenv("NOFX_BACKEND_PORT")); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil && port > 0 {
			apiPort = port
			log.Printf("🔌 使用环境变量端口: %d (NOFX_BACKEND_PORT)", apiPort)
		} else {
			log.Printf("⚠️  环境变量 NOFX_BACKEND_PORT 无效: %s", envPort)
		}
	} else if apiPortStr != "" {
		// 2. 从数据库配置读取（config.json 同步过来的）
		if port, err := strconv.Atoi(apiPortStr); err == nil && port > 0 {
			apiPort = port
			log.Printf("🔌 使用数据库配置端口: %d (api_server_port)", apiPort)
		}
	} else {
		log.Printf("🔌 使用默认端口: %d", apiPort)
	}

	// 创建并启动API服务器
	apiServer := api.NewServer(traderManager, database, cryptoService, apiPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("❌ API服务器错误: %v", err)
		}
	}()

	// 初始化多数据源管理器（健康检查间隔: 60秒）
	log.Println("🌐 初始化多数据源管理器...")
	dataSourceManager := market.NewDataSourceManager(60 * time.Second)

	// 添加 Binance 数据源
	binanceSource := market.NewBinanceDataSource()
	dataSourceManager.AddSource(binanceSource)

	// 添加 Hyperliquid 数据源（主网）
	hyperliquidSource := market.NewHyperliquidDataSource(false)
	dataSourceManager.AddSource(hyperliquidSource)

	// 启动健康检查
	dataSourceManager.Start()
	log.Printf("✅ 数据源管理器已启动，包含 %d 个数据源", 2)

	// 启动流行情数据 - 默认使用所有交易员设置的币种 如果没有设置币种 则优先使用系统默认
	// 获取所有活跃 trader 的时间线配置（合并后的并集）
	timeframes := database.GetAllTimeframes()
	go market.NewWSMonitor(150, timeframes, dataSourceManager).Start(database.GetCustomCoins())
	//go market.NewWSMonitor(150, timeframes).Start([]string{}) //这里是一个使用方式 传入空的话 则使用market市场的所有币种
	// 设置优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Admin模式下自动启动标记为运行状态的交易员
	if adminMode {
		if err := traderManager.StartRunningTraders(database); err != nil {
			log.Printf("⚠️  自动启动交易员失败: %v", err)
		}
	}

	// 等待退出信号
	<-sigChan
	fmt.Println()
	fmt.Println()
	log.Println("📛 收到退出信号，正在优雅关闭...")

	// 步骤 1: 停止所有交易员
	log.Println("⏸️  停止所有交易员...")
	traderManager.StopAll()
	log.Println("✅ 所有交易员已停止")

	// 步骤 2: 关闭 API 服务器
	log.Println("🛑 停止 API 服务器...")
	if err := apiServer.Shutdown(); err != nil {
		log.Printf("⚠️  关闭 API 服务器时出错: %v", err)
	} else {
		log.Println("✅ API 服务器已安全关闭")
	}

	// 步骤 2.5: 停止数据源管理器
	log.Println("🌐 停止数据源管理器...")
	dataSourceManager.Stop()
	log.Println("✅ 数据源管理器已停止")

	// 步骤 3: 关闭数据库连接 (确保所有写入完成)
	log.Println("💾 关闭数据库连接...")
	if err := database.Close(); err != nil {
		log.Printf("❌ 关闭数据库失败: %v", err)
	} else {
		log.Println("✅ 数据库已安全关闭，所有数据已持久化")
	}

	fmt.Println()
	fmt.Println("👋 感谢使用AI交易系统！")
}
