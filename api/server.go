package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"nofx/auth"
	"nofx/config"
	"nofx/crypto"
	"nofx/decision"
	"nofx/hook"
	"nofx/manager"
	"nofx/middleware"
	"nofx/trader"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
)

// Server HTTP API服务器
type Server struct {
	router        *gin.Engine
	httpServer    *http.Server
	traderManager *manager.TraderManager
	database      *config.Database
	cryptoHandler *CryptoHandler
	port          int
}

// NewServer 创建API服务器
func NewServer(traderManager *manager.TraderManager, database *config.Database, cryptoService *crypto.CryptoService, port int) *Server {
	// 设置为Release模式（减少日志输出）
	gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// 反向代理支持：信任 X-Forwarded-For 和 X-Real-IP 头
	// 当部署在 Nginx/Caddy/Traefik 等反向代理后面时启用
	trustProxy := strings.EqualFold(os.Getenv("TRUST_PROXY"), "true")
	if trustProxy {
		// 设置信任的代理，获取真实客户端 IP
		// 使用 gin 的 SetTrustedProxies 方法
		router.SetTrustedProxies([]string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
		log.Println("🔄 [Proxy] 已启用反向代理支持 (TRUST_PROXY=true)")
		log.Println("    信任的代理网段: 127.0.0.1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16")
	} else {
		// 默认不信任任何代理
		router.SetTrustedProxies(nil)
	}

	// 配置允许的 CORS 来源
	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:5173",
	}
	if frontendURL := os.Getenv("FRONTEND_URL"); frontendURL != "" {
		allowedOrigins = append(allowedOrigins, frontendURL)
	}
	if corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); corsOrigins != "" {
		additionalOrigins := strings.Split(corsOrigins, ",")
		for _, origin := range additionalOrigins {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}

	// 生产环境 CORS 配置检查
	isDevelopment := os.Getenv("ENVIRONMENT") != "production"
	corsConfigured := os.Getenv("CORS_ALLOWED_ORIGINS") != "" || os.Getenv("FRONTEND_URL") != ""
	disableCORS := strings.EqualFold(os.Getenv("DISABLE_CORS"), "true")

	if !isDevelopment && !corsConfigured && !disableCORS {
		log.Println("")
		log.Println("╔═══════════════════════════════════════════════════════════════════╗")
		log.Println("║  ⚠️  警告：生產模式下未配置 CORS！                                ║")
		log.Println("╟───────────────────────────────────────────────────────────────────╢")
		log.Println("║  當前狀態：                                                        ║")
		log.Println("║    • ENVIRONMENT=production（生產模式）                           ║")
		log.Println("║    • CORS_ALLOWED_ORIGINS 未設置                                  ║")
		log.Println("║                                                                   ║")
		log.Println("║  預期行為：                                                        ║")
		log.Println("║    ✅ localhost:3000, localhost:5173 可正常訪問                   ║")
		log.Println("║    ❌ 其他所有來源將被 403 拒絕（包括域名、公網 IP）              ║")
		log.Println("║                                                                   ║")
		log.Println("║  解決方案（選擇其一）：                                            ║")
		log.Println("║                                                                   ║")
		log.Println("║  1️⃣  配置允許的前端域名（推薦用於生產環境）：                     ║")
		log.Println("║      在 .env 中添加：                                             ║")
		log.Println("║      CORS_ALLOWED_ORIGINS=https://yourdomain.com                 ║")
		log.Println("║                                                                   ║")
		log.Println("║  2️⃣  切換回開發模式（不設置 ENVIRONMENT 或設為其他值）：           ║")
		log.Println("║      移除或註釋 .env 中的：                                       ║")
		log.Println("║      # ENVIRONMENT=production                                    ║")
		log.Println("║                                                                   ║")
		log.Println("║  3️⃣  完全禁用 CORS（僅限安全的內網環境）：                        ║")
		log.Println("║      在 .env 中添加：                                             ║")
		log.Println("║      DISABLE_CORS=true                                           ║")
		log.Println("║                                                                   ║")
		log.Println("║  修改後需重啟容器：                                                ║")
		log.Println("║      docker-compose restart                                      ║")
		log.Println("╚═══════════════════════════════════════════════════════════════════╝")
		log.Println("")
	} else if isDevelopment {
		log.Println("🔧 [CORS] 開發模式啟動：自動允許 localhost、.local 域名和私有 IP")
		if len(allowedOrigins) > 2 {
			log.Printf("    已配置額外白名單：%v", allowedOrigins[2:])
		}
	} else if disableCORS {
		log.Println("⚠️  [CORS] CORS 檢查已完全禁用 (DISABLE_CORS=true)")
	} else {
		log.Println("🔒 [CORS] 生產模式啟動：嚴格執行白名單")
		log.Printf("    允許的來源：%v", allowedOrigins)
	}

	// 启用 CORS（白名单模式）
	router.Use(corsMiddleware(allowedOrigins))

	// 启用全局速率限制 (每秒 50 个请求)
	// 注意：前端頁面加載時會並發發送 7-8 個請求，10 req/s 太嚴格
	globalLimiter := middleware.NewIPRateLimiter(rate.Limit(50), 50)
	router.Use(middleware.RateLimitMiddleware(globalLimiter))

	// CSRF 保护（Double Submit Cookie 模式）- 可通过环境变量控制
	// 开发阶段默认关闭以避免频繁 403 错误，生产环境建议启用
	enableCSRF := os.Getenv("ENABLE_CSRF")
	if enableCSRF == "true" {
		log.Println("✅ [CSRF] CSRF 保护已启用")
		csrfConfig := middleware.DefaultCSRFConfig()
		// 生产环境应启用 HTTPS-only Cookie
		if os.Getenv("ENVIRONMENT") == "production" {
			csrfConfig.CookieSecure = true
		}
		router.Use(middleware.CSRFMiddleware(csrfConfig))
	} else {
		log.Println("⚠️  [CSRF] CSRF 保护已禁用（开发模式）")
		log.Println("    提示：生产环境请设置 ENABLE_CSRF=true")
	}

	// 控制是否允許客戶端解密 API（預設關閉）
	enableClientDecrypt := strings.EqualFold(os.Getenv("ENABLE_CLIENT_DECRYPT_API"), "true")
	if enableClientDecrypt {
		log.Println("🔐 [Crypto] ENABLE_CLIENT_DECRYPT_API=true，/api/crypto/decrypt 需要 JWT 且會驗證 AAD")
	} else {
		log.Println("🔐 [Crypto] 客戶端解密 API 已禁用（ENABLE_CLIENT_DECRYPT_API未開啟）")
	}

	// 创建加密处理器
	cryptoHandler := NewCryptoHandler(cryptoService, enableClientDecrypt)

	s := &Server{
		router:        router,
		traderManager: traderManager,
		database:      database,
		cryptoHandler: cryptoHandler,
		port:          port,
	}

	// 设置路由
	s.setupRoutes()

	return s
}

// corsMiddleware CORS中间件（智能模式：开发环境自动允许私有网络）
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	// 检查是否完全禁用 CORS（用于内网环境或开发环境）
	disableCORS := strings.EqualFold(os.Getenv("DISABLE_CORS"), "true")

	// 检测是否为开发环境（默认为开发环境）
	isDevelopment := os.Getenv("ENVIRONMENT") != "production"

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// 如果禁用了 CORS，允许所有请求
		if disableCORS {
			if origin != "" {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
			}
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusOK)
				return
			}
			c.Next()
			return
		}

		// 正常 CORS 检查流程
		allowed := false

		// 1. 检查白名单
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// 2. 开发模式：检查是否为私有网络来源
		if !allowed && isDevelopment && origin != "" {
			if isPrivateNetworkOrigin(origin) {
				allowed = true
				log.Printf("🔓 [CORS] 开发模式自动允许: %s (私有网络/localhost/.local)", origin)
			}
		}

		// 3. 设置 CORS 响应头
		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
		} else if origin != "" {
			// 开发模式：只记录警告，但仍然允许请求（避免阻断开发）
			if isDevelopment {
				log.Printf("⚠️  [CORS] 开发模式警告：未识别的来源 %s", origin)
				log.Printf("    提示：如需在生产环境使用，请添加到 .env: CORS_ALLOWED_ORIGINS=%s", origin)
				// 开发模式下仍然设置 CORS 头，避免阻断
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
				c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
			} else {
				// 生产模式：严格拒绝
				log.Printf("🚫 [CORS] 生产模式拒绝来源: %s", origin)
				log.Printf("    配置方法：在 .env 添加 CORS_ALLOWED_ORIGINS=%s", origin)
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":   "Origin not allowed",
					"origin":  origin,
					"help":    "請在 .env 文件中添加此來源到 CORS_ALLOWED_ORIGINS",
					"example": fmt.Sprintf("CORS_ALLOWED_ORIGINS=%s", origin),
					"docs":    "重啟容器後生效：docker-compose restart",
				})
				return
			}
		}

		// 处理 OPTIONS 预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// isPrivateNetworkOrigin 检查是否为私有网络来源
// 支持以下类型：
// - localhost (localhost, 127.0.0.1, ::1)
// - .local 域名 (mDNS/Bonjour，如 myserver.local)
// - RFC 1918 私有 IP：10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
func isPrivateNetworkOrigin(origin string) bool {
	// 解析 origin URL (格式: http://192.168.1.100:3000 或 http://myserver.local:3000)
	parts := strings.Split(origin, "://")
	if len(parts) != 2 {
		return false
	}

	hostPort := parts[1]
	host := strings.Split(hostPort, ":")[0]

	// 1. 检查 localhost 变体
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return true
	}

	// 2. 检查 .local 域名 (mDNS)
	if strings.HasSuffix(host, ".local") {
		return true
	}

	// 3. 尝试解析为 IP 地址
	ip := net.ParseIP(host)
	if ip == nil {
		// 不是 IP 地址也不是已知的本地域名，可能是内网域名
		// 为了安全，这里返回 false，让用户手动添加到白名单
		return false
	}

	// 4. 检查是否为 loopback IP (127.0.0.0/8)
	if ip.IsLoopback() {
		return true
	}

	// 5. 检查 RFC 1918 私有 IP 地址
	privateIPBlocks := []*net.IPNet{
		// 10.0.0.0/8
		{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)},
		// 172.16.0.0/12
		{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)},
		// 192.168.0.0/16
		{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)},
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}

	return false
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// API路由组
	api := s.router.Group("/api")
	{
		// 健康检查
		api.Any("/health", s.handleHealth)

		// 管理员登录（管理员模式下使用，公共）

		// 系统支持的模型和交易所（无需认证）
		api.GET("/supported-models", s.handleGetSupportedModels)
		api.GET("/supported-exchanges", s.handleGetSupportedExchanges)

		// 系统配置（无需认证，用于前端判断是否管理员模式/注册是否开启）
		api.GET("/config", s.handleGetSystemConfig)

		// 加密相关接口（无需认证）
		api.GET("/crypto/public-key", s.cryptoHandler.HandleGetPublicKey)

		// CSRF Token 获取（无需认证）
		api.GET("/csrf-token", s.handleGetCSRFToken)

		// 系统提示词模板管理（无需认证）
		api.GET("/prompt-templates", s.handleGetPromptTemplates)
		api.GET("/prompt-templates/:name", s.handleGetPromptTemplate)

		// 公开的竞赛数据（无需认证）
		api.GET("/traders", s.handlePublicTraderList)
		api.GET("/competition", s.handlePublicCompetition)
		api.GET("/top-traders", s.handleTopTraders)
		api.GET("/equity-history", s.handleEquityHistory)
		api.POST("/equity-history-batch", s.handleEquityHistoryBatch)
		api.GET("/traders/:id/public-config", s.handleGetPublicTraderConfig)

		// 认证相关路由（应用严格速率限制，防止暴力破解）
		authGroup := api.Group("/", middleware.AuthRateLimitMiddleware())
		{
			authGroup.POST("/register", s.handleRegister)
			authGroup.POST("/login", s.handleLogin)
			authGroup.POST("/verify-otp", s.handleVerifyOTP)
			authGroup.POST("/complete-registration", s.handleCompleteRegistration)
			authGroup.POST("/reset-password", s.handleResetPassword)
			authGroup.POST("/refresh-token", s.handleRefreshToken)
		}

		// 需要认证的路由
		protected := api.Group("/", s.authMiddleware())
		{
			// 注销（加入黑名单）
			protected.POST("/logout", s.handleLogout)

			// 僅在顯式啟用時開放解密端點（需要JWT身份）
			if s.cryptoHandler.AllowDecryptEndpoint() {
				protected.POST("/crypto/decrypt", s.cryptoHandler.HandleDecryptSensitiveData)
			}

			// 服务器IP查询（需要认证，用于白名单配置）
			protected.GET("/server-ip", s.handleGetServerIP)

			// AI交易员管理
			protected.GET("/my-traders", s.handleTraderList)
			protected.GET("/traders/:id/config", s.handleGetTraderConfig)
			protected.POST("/traders", s.handleCreateTrader)
			protected.PUT("/traders/:id", s.handleUpdateTrader)
			protected.DELETE("/traders/:id", s.handleDeleteTrader)
			protected.POST("/traders/:id/start", s.handleStartTrader)
			protected.POST("/traders/:id/stop", s.handleStopTrader)
			protected.PUT("/traders/:id/prompt", s.handleUpdateTraderPrompt)

			// AI模型配置
			protected.GET("/models", s.handleGetModelConfigs)
			protected.PUT("/models", s.handleUpdateModelConfigs)

			// 交易所配置
			protected.GET("/exchanges", s.handleGetExchangeConfigs)
			protected.PUT("/exchanges", s.handleUpdateExchangeConfigs)

			// 用户信号源配置
			protected.GET("/user/signal-sources", s.handleGetUserSignalSource)
			protected.POST("/user/signal-sources", s.handleSaveUserSignalSource)

			// 提示词模板管理（需要认证）
			protected.POST("/prompt-templates", s.handleCreatePromptTemplate)
			protected.PUT("/prompt-templates/:name", s.handleUpdatePromptTemplate)
			protected.DELETE("/prompt-templates/:name", s.handleDeletePromptTemplate)
			protected.POST("/prompt-templates/reload", s.handleReloadPromptTemplates)
			// 指定trader的数据（使用query参数 ?trader_id=xxx）
			protected.GET("/status", s.handleStatus)
			protected.GET("/account", s.handleAccount)
			protected.GET("/positions", s.handlePositions)
			protected.GET("/decisions", s.handleDecisions)
			protected.GET("/decisions/latest", s.handleLatestDecisions)
			protected.GET("/statistics", s.handleStatistics)
			protected.GET("/performance", s.handlePerformance)
		}
	}
}

// handleHealth 健康检查
func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   c.Request.Context().Value("time"),
	})
}

// handleGetCSRFToken 获取 CSRF Token
// 前端调用此接口获取 CSRF Token，用于后续 POST/PUT/DELETE 请求
func (s *Server) handleGetCSRFToken(c *gin.Context) {
	csrfConfig := middleware.DefaultCSRFConfig()
	token := middleware.GetCSRFToken(c, csrfConfig)

	c.JSON(http.StatusOK, gin.H{
		"csrf_token":  token,
		"header_name": csrfConfig.HeaderName,
		"note":        "Please include this token in the X-CSRF-Token header for POST/PUT/DELETE requests",
	})
}

// handleGetSystemConfig 获取系统配置（客户端需要知道的配置）
func (s *Server) handleGetSystemConfig(c *gin.Context) {
	// 获取默认币种
	defaultCoinsStr, _ := s.database.GetSystemConfig("default_coins")
	var defaultCoins []string
	if defaultCoinsStr != "" {
		json.Unmarshal([]byte(defaultCoinsStr), &defaultCoins)
	}
	if len(defaultCoins) == 0 {
		// 使用硬编码的默认币种
		defaultCoins = []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"}
	}

	// 获取杠杆配置
	btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage")
	altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage")

	btcEthLeverage := 5
	if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
		btcEthLeverage = val
	}

	altcoinLeverage := 5
	if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
		altcoinLeverage = val
	}

	// 获取内测模式配置
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	betaMode := betaModeStr == "true"

	regEnabledStr, err := s.database.GetSystemConfig("registration_enabled")
	registrationEnabled := true
	if err == nil {
		registrationEnabled = strings.ToLower(regEnabledStr) != "false"
	}

	c.JSON(http.StatusOK, gin.H{
		"beta_mode":            betaMode,
		"default_coins":        defaultCoins,
		"btc_eth_leverage":     btcEthLeverage,
		"altcoin_leverage":     altcoinLeverage,
		"registration_enabled": registrationEnabled,
	})
}

// handleGetServerIP 获取服务器IP地址（用于白名单配置）
func (s *Server) handleGetServerIP(c *gin.Context) {

	// 首先尝试从Hook获取用户专用IP
	userIP := hook.HookExec[hook.IpResult](hook.GETIP, c.GetString("user_id"))
	if userIP != nil && userIP.Error() == nil {
		c.JSON(http.StatusOK, gin.H{
			"public_ip": userIP.GetResult(),
			"message":   "请将此IP地址添加到白名单中",
		})
		return
	}

	// 尝试通过第三方API获取公网IP
	publicIP := getPublicIPFromAPI()

	// 如果第三方API失败，从网络接口获取第一个公网IP
	if publicIP == "" {
		publicIP = getPublicIPFromInterface()
	}

	// 如果还是没有获取到，返回错误
	if publicIP == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取公网IP地址"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"public_ip": publicIP,
		"message":   "请将此IP地址添加到白名单中",
	})
}

// getPublicIPFromAPI 通过第三方API获取公网IP
func getPublicIPFromAPI() string {
	// 尝试多个公网IP查询服务
	services := []string{
		"https://api.ipify.org?format=text",
		"https://icanhazip.com",
		"https://ifconfig.me",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body := make([]byte, 128)
			n, err := resp.Body.Read(body)
			if err != nil && err.Error() != "EOF" {
				continue
			}

			ip := strings.TrimSpace(string(body[:n]))
			// 验证是否为有效的IP地址
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return ""
}

// getPublicIPFromInterface 从网络接口获取第一个公网IP
func getPublicIPFromInterface() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// 跳过未启用的接口和回环接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			// 只考虑IPv4地址
			if ip.To4() != nil {
				ipStr := ip.String()
				// 排除私有IP地址范围
				if !isPrivateIP(ip) {
					return ipStr
				}
			}
		}
	}

	return ""
}

// isPrivateIP 判断是否为私有IP地址
func isPrivateIP(ip net.IP) bool {
	// 私有IP地址范围：
	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

// getTraderFromQuery 从query参数获取trader
func (s *Server) getTraderFromQuery(c *gin.Context) (*manager.TraderManager, string, error) {
	userID := c.GetString("user_id")
	traderID := c.Query("trader_id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	if traderID == "" {
		// 如果没有指定trader_id，返回该用户的第一个trader
		ids := s.traderManager.GetTraderIDs()
		if len(ids) == 0 {
			return nil, "", fmt.Errorf("没有可用的trader")
		}

		// 获取用户的交易员列表，优先返回用户自己的交易员
		userTraders, err := s.database.GetTraders(userID)
		if err == nil && len(userTraders) > 0 {
			traderID = userTraders[0].ID
		} else {
			traderID = ids[0]
		}
	}

	return s.traderManager, traderID, nil
}

// AI交易员管理相关结构体
type CreateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"` // 系统提示词模板名称
	IsCrossMargin        *bool   `json:"is_cross_margin"`        // 指针类型，nil表示使用默认值true
	UseCoinPool          bool    `json:"use_coin_pool"`
	UseOITop             bool    `json:"use_oi_top"`
	TakerFeeRate         float64 `json:"taker_fee_rate"`        // Taker fee rate, default 0.0004 (0.04%)
	MakerFeeRate         float64 `json:"maker_fee_rate"`        // Maker fee rate, default 0.0002 (0.02%)
	OrderStrategy        string  `json:"order_strategy"`        // Order strategy: market_only, conservative_hybrid, limit_only
	LimitPriceOffset     float64 `json:"limit_price_offset"`    // Limit price offset percentage, default -0.03 (-0.03%)
	LimitTimeoutSeconds  int     `json:"limit_timeout_seconds"` // Limit order timeout in seconds, default 60
	Timeframes           string  `json:"timeframes"`            // 时间线选择 (逗号分隔，例如: "1m,4h,1d")
}

type ModelConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Enabled      bool   `json:"enabled"`
	APIKey       string `json:"apiKey,omitempty"`
	CustomAPIURL string `json:"customApiUrl,omitempty"`
}

// SafeModelConfig 安全的模型配置结构（不包含敏感信息）
type SafeModelConfig struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Enabled         bool   `json:"enabled"`
	CustomAPIURL    string `json:"customApiUrl"`    // 自定义API URL（通常不敏感）
	CustomModelName string `json:"customModelName"` // 自定义模型名（不敏感）
}

type ExchangeConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "cex" or "dex"
	Enabled   bool   `json:"enabled"`
	APIKey    string `json:"apiKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Testnet   bool   `json:"testnet,omitempty"`
}

// SafeExchangeConfig 安全的交易所配置结构（不包含敏感信息）
type SafeExchangeConfig struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Type                  string `json:"type"` // "cex" or "dex"
	Enabled               bool   `json:"enabled"`
	Testnet               bool   `json:"testnet,omitempty"`
	HyperliquidWalletAddr string `json:"hyperliquidWalletAddr"` // Hyperliquid钱包地址（不敏感）
	AsterUser             string `json:"asterUser"`             // Aster用户名（不敏感）
	AsterSigner           string `json:"asterSigner"`           // Aster签名者（不敏感）
}

type UpdateModelConfigRequest struct {
	Models map[string]struct {
		Enabled         bool   `json:"enabled"`
		APIKey          string `json:"api_key"`
		CustomAPIURL    string `json:"custom_api_url"`
		CustomModelName string `json:"custom_model_name"`
	} `json:"models"`
}

type UpdateExchangeConfigRequest struct {
	Exchanges map[string]struct {
		Enabled               bool   `json:"enabled"`
		APIKey                string `json:"api_key"`
		SecretKey             string `json:"secret_key"`
		Testnet               bool   `json:"testnet"`
		HyperliquidWalletAddr string `json:"hyperliquid_wallet_addr"`
		AsterUser             string `json:"aster_user"`
		AsterSigner           string `json:"aster_signer"`
		AsterPrivateKey       string `json:"aster_private_key"`
	} `json:"exchanges"`
}

// queryExchangeBalance 查詢交易所實際餘額
// 根據交易所類型創建臨時 trader 並查詢當前總資產
func (s *Server) queryExchangeBalance(userID, exchangeID string, exchangeCfg *config.ExchangeConfig) (float64, error) {
	// 根據交易所類型創建臨時 trader
	var tempTrader trader.Trader
	var err error

	switch exchangeID {
	case "binance":
		// 使用默认订单策略（查询余额不需要实际下单）
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID, "market_only", -0.03, 60)
	case "hyperliquid":
		tempTrader, err = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey, // private key
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		tempTrader, err = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
		)
	default:
		return 0, fmt.Errorf("不支持的交易所類型: %s", exchangeID)
	}

	if err != nil {
		return 0, fmt.Errorf("創建臨時 trader 失敗: %w", err)
	}

	if tempTrader == nil {
		return 0, fmt.Errorf("tempTrader 為 nil")
	}

	// 查詢實際餘額
	balanceInfo, err := tempTrader.GetBalance()
	if err != nil {
		return 0, fmt.Errorf("查詢交易所余額失敗: %w", err)
	}

	// 使用統一的工具函數解析總資產
	totalEquity, success := trader.ParseTotalEquity(balanceInfo, "✓")
	if !success {
		return 0, fmt.Errorf("無法從餘額信息中提取總資產")
	}

	return totalEquity, nil
}

// handleCreateTrader 创建新的AI交易员
func (s *Server) handleCreateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	var err error // Declare err for later use
	var req CreateTraderRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 校验杠杆值
	if req.BTCETHLeverage < 0 || req.BTCETHLeverage > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "BTC/ETH杠杆必须在1-50倍之间"})
		return
	}
	if req.AltcoinLeverage < 0 || req.AltcoinLeverage > 20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "山寨币杠杆必须在1-20倍之间"})
		return
	}

	// 校验交易币种格式
	if req.TradingSymbols != "" {
		symbols := strings.Split(req.TradingSymbols, ",")
		for _, symbol := range symbols {
			symbol = strings.TrimSpace(symbol)
			if symbol != "" && !strings.HasSuffix(strings.ToUpper(symbol), "USDT") {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的币种格式: %s，必须以USDT结尾", symbol)})
				return
			}
		}
	}

	// ✅ 检查交易员名称是否重复
	existingTraders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("检查交易员名称失败: %v", err)})
		return
	}
	for _, existing := range existingTraders {
		if existing.Name == req.Name {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("交易员名称 '%s' 已存在，请使用其他名称", req.Name)})
			return
		}
	}

	// 生成交易员ID (使用 UUID 确保唯一性，解决 Issue #893)
	// 保留前缀以便调试和日志追踪
	traderID := fmt.Sprintf("%s_%s_%s", req.ExchangeID, req.AIModelID, uuid.New().String())

	// 设置默认值
	isCrossMargin := true // 默认为全仓模式
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值（从系统配置获取）
	btcEthLeverage := 5
	altcoinLeverage := 5
	if req.BTCETHLeverage > 0 {
		btcEthLeverage = req.BTCETHLeverage
	} else {
		// 从系统配置获取默认值
		if btcEthLeverageStr, _ := s.database.GetSystemConfig("btc_eth_leverage"); btcEthLeverageStr != "" {
			if val, err := strconv.Atoi(btcEthLeverageStr); err == nil && val > 0 {
				btcEthLeverage = val
			}
		}
	}
	if req.AltcoinLeverage > 0 {
		altcoinLeverage = req.AltcoinLeverage
	} else {
		// 从系统配置获取默认值
		if altcoinLeverageStr, _ := s.database.GetSystemConfig("altcoin_leverage"); altcoinLeverageStr != "" {
			if val, err := strconv.Atoi(altcoinLeverageStr); err == nil && val > 0 {
				altcoinLeverage = val
			}
		}
	}

	// 设置系统提示词模板默认值
	systemPromptTemplate := "default"
	if req.SystemPromptTemplate != "" {
		systemPromptTemplate = req.SystemPromptTemplate
	}

	// 设置扫描间隔默认值
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = 2 // 默认2分钟
	} else if scanIntervalMinutes < 1 {
		scanIntervalMinutes = 1 // 最低1分钟，不允许小于1分钟
	}

	// ✅ Fix #787, #807, #790: Respect user-specified initial balance
	// ✅ Fix P&L calculation: Use total equity instead of available balance when auto-querying
	actualBalance := req.InitialBalance // Default: use user input

	// Only auto-query from exchange when user input <= 0
	if actualBalance <= 0 {
		log.Printf("ℹ️ User didn't specify initial balance (%.2f), querying from exchange...", actualBalance)

		exchanges, exchangeErr := s.database.GetExchanges(userID)
		if exchangeErr != nil {
			log.Printf("⚠️ 获取交易所配置失败，使用默认值 100 USDT: %v", exchangeErr)
			actualBalance = 100.0
		} else {
			// 查找匹配的交易所配置
			var exchangeCfg *config.ExchangeConfig
			for _, ex := range exchanges {
				if ex.ExchangeID == req.ExchangeID {
					exchangeCfg = ex
					break
				}
			}

			if exchangeCfg == nil {
				log.Printf("⚠️ 未找到交易所 %s 的配置，使用默认值 100 USDT", req.ExchangeID)
				actualBalance = 100.0
			} else if !exchangeCfg.Enabled {
				log.Printf("⚠️ 交易所 %s 未启用，使用默认值 100 USDT", req.ExchangeID)
				actualBalance = 100.0
			} else {
				// 🔧 计算Total Equity = Wallet Balance + Unrealized Profit
				// 这是账户的真实净值，用作Initial Balance的基准
				// 使用輔助函數查詢交易所余額
				balance, queryErr := s.queryExchangeBalance(userID, req.ExchangeID, exchangeCfg)
				if queryErr != nil {
					log.Printf("⚠️ 查詢余額失敗，使用默認值 100 USDT: %v", queryErr)
					actualBalance = 100.0
				} else {
					actualBalance = balance
					log.Printf("✅ 查询到交易所实际净值: %.2f USDT (用户输入: %.2f)",
						actualBalance, req.InitialBalance)
				}
			}
		}
	} else {
		log.Printf("✓ 使用用户指定的初始余额: %.2f USDT", actualBalance)
	}

	// 设置默认费率
	takerFeeRate := req.TakerFeeRate
	makerFeeRate := req.MakerFeeRate

	// 如果用户未设置，使用默认值
	if takerFeeRate == 0 {
		takerFeeRate = 0.0004 // Binance 标准 Taker 费率
	}
	if makerFeeRate == 0 {
		makerFeeRate = 0.0002 // Binance 标准 Maker 费率
	}

	// 添加费率范围验证
	if takerFeeRate < 0 || takerFeeRate > 0.01 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Taker费率必须在0-1%之间"})
		return
	}
	if makerFeeRate < 0 || makerFeeRate > 0.01 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maker费率必须在0-1%之间"})
		return
	}

	log.Printf("✓ 费率配置: Taker=%.4f (%.2f%%), Maker=%.4f (%.2f%%)",
		takerFeeRate, takerFeeRate*100, makerFeeRate, makerFeeRate*100)

	// 设置时间线默认值
	timeframes := req.Timeframes
	if timeframes == "" {
		timeframes = "4h" // 默认只勾选4小时线
	}

	// 设置订单策略默认值
	orderStrategy := req.OrderStrategy
	if orderStrategy == "" {
		orderStrategy = "conservative_hybrid" // 默认使用保守混合策略
	}

	// 设置限价偏移默认值
	limitPriceOffset := req.LimitPriceOffset
	if limitPriceOffset == 0 {
		limitPriceOffset = -0.03 // 默认 -0.03%
	}

	// 设置限价超时默认值
	limitTimeoutSeconds := req.LimitTimeoutSeconds
	if limitTimeoutSeconds == 0 {
		limitTimeoutSeconds = 60 // 默认 60 秒
	}

	// 查询 AI Model 和 Exchange 的自增 ID
	log.Printf("🔍 [DEBUG] 步骤7: 查询用户 %s 的 AI 模型配置 (请求的 AI 模型: %s)...", userID, req.AIModelID)
	aiModels, err := s.database.GetAIModels(userID)
	if err != nil {
		log.Printf("❌ [DEBUG] 查询 AI 模型失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取AI模型配置失败"})
		return
	}
	log.Printf("✅ [DEBUG] 找到 %d 个 AI 模型配置", len(aiModels))

	var aiModelIntID int
	var aiModelFound bool
	for _, model := range aiModels {
		log.Printf("🔍 [DEBUG] 检查 AI 模型: ID=%d, ModelID=%s (寻找: %s)", model.ID, model.ModelID, req.AIModelID)
		if model.ModelID == req.AIModelID {
			aiModelIntID = model.ID
			aiModelFound = true
			log.Printf("✅ [DEBUG] 找到匹配的 AI 模型: ID=%d", aiModelIntID)
			break
		}
	}
	if !aiModelFound {
		log.Printf("❌ [DEBUG] 未找到 AI 模型 '%s'，可用的模型：", req.AIModelID)
		for _, model := range aiModels {
			log.Printf("   - ModelID=%s", model.ModelID)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("AI模型 %s 不存在", req.AIModelID)})
		return
	}

	log.Printf("🔍 [DEBUG] 步骤8: 查询用户 %s 的交易所配置 (请求的交易所: %s)...", userID, req.ExchangeID)
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("❌ [DEBUG] 查询交易所失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易所配置失败"})
		return
	}
	log.Printf("✅ [DEBUG] 找到 %d 个交易所配置", len(exchanges))

	var exchangeIntID int
	var exchangeFound bool
	for _, exchange := range exchanges {
		log.Printf("🔍 [DEBUG] 检查交易所: ID=%d, ExchangeID=%s (寻找: %s)", exchange.ID, exchange.ExchangeID, req.ExchangeID)
		if exchange.ExchangeID == req.ExchangeID {
			exchangeIntID = exchange.ID
			exchangeFound = true
			log.Printf("✅ [DEBUG] 找到匹配的交易所: ID=%d", exchangeIntID)
			break
		}
	}
	if !exchangeFound {
		log.Printf("❌ [DEBUG] 未找到交易所 '%s'，可用的交易所：", req.ExchangeID)
		for _, exchange := range exchanges {
			log.Printf("   - ExchangeID=%s", exchange.ExchangeID)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("交易所 %s 不存在", req.ExchangeID)})
		return
	}

	// 创建交易员配置（数据库实体）
	log.Printf("🔍 [DEBUG] 步骤9: 构建交易员配置对象...")
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            aiModelIntID,  // 使用查询到的自增 ID
		ExchangeID:           exchangeIntID, // 使用查询到的自增 ID
		InitialBalance:       actualBalance, // 使用实际查询的余额
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseCoinPool:          req.UseCoinPool,
		UseOITop:             req.UseOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		TakerFeeRate:         takerFeeRate,        // 添加 Taker 费率
		MakerFeeRate:         makerFeeRate,        // 添加 Maker 费率
		OrderStrategy:        orderStrategy,       // 添加订单策略
		LimitPriceOffset:     limitPriceOffset,    // 添加限价偏移
		LimitTimeoutSeconds:  limitTimeoutSeconds, // 添加限价超时
		Timeframes:           timeframes,          // 添加时间线选择
		IsRunning:            false,
	}
	log.Printf("✅ [DEBUG] 交易员配置对象已构建: ID=%s, AIModelID=%d, ExchangeID=%d", traderID, aiModelIntID, exchangeIntID)

	// 保存到数据库
	log.Printf("🔍 [DEBUG] 步骤10: 保存交易员到数据库...")
	err = s.database.CreateTrader(trader)
	if err != nil {
		log.Printf("❌ [DEBUG] 数据库 CreateTrader 失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建交易员失败: %v", err)})
		return
	}
	log.Printf("✅ [DEBUG] 交易员已成功保存到数据库")

	// 立即将新交易员加载到TraderManager中
	err = s.traderManager.LoadTraderByID(s.database, userID, traderID)
	if err != nil {
		log.Printf("⚠️ 加载交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易员已经成功创建到数据库
	}

	log.Printf("✓ 创建交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusCreated, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"is_running":  false,
	})
}

// UpdateTraderRequest 更新交易员请求
type UpdateTraderRequest struct {
	Name                 string  `json:"name" binding:"required"`
	AIModelID            string  `json:"ai_model_id" binding:"required"`
	ExchangeID           string  `json:"exchange_id" binding:"required"`
	InitialBalance       float64 `json:"initial_balance"`
	ScanIntervalMinutes  int     `json:"scan_interval_minutes"`
	BTCETHLeverage       int     `json:"btc_eth_leverage"`
	AltcoinLeverage      int     `json:"altcoin_leverage"`
	TradingSymbols       string  `json:"trading_symbols"`
	CustomPrompt         string  `json:"custom_prompt"`
	OverrideBasePrompt   bool    `json:"override_base_prompt"`
	SystemPromptTemplate string  `json:"system_prompt_template"`
	IsCrossMargin        *bool   `json:"is_cross_margin"`
	UseCoinPool          *bool   `json:"use_coin_pool"`
	UseOITop             *bool   `json:"use_oi_top"`
	TakerFeeRate         float64 `json:"taker_fee_rate"`        // Taker fee rate
	MakerFeeRate         float64 `json:"maker_fee_rate"`        // Maker fee rate
	OrderStrategy        string  `json:"order_strategy"`        // Order strategy
	LimitPriceOffset     float64 `json:"limit_price_offset"`    // Limit price offset
	LimitTimeoutSeconds  int     `json:"limit_timeout_seconds"` // Limit timeout in seconds
	Timeframes           string  `json:"timeframes"`            // Timeframes selection
}

// handleUpdateTrader 更新交易员配置
func (s *Server) handleUpdateTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	var req UpdateTraderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查交易员是否存在且属于当前用户
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易员列表失败"})
		return
	}

	var existingTrader *config.TraderRecord
	for _, trader := range traders {
		if trader.ID == traderID {
			existingTrader = trader
			break
		}
	}

	if existingTrader == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 设置默认值
	isCrossMargin := existingTrader.IsCrossMargin // 保持原值
	if req.IsCrossMargin != nil {
		isCrossMargin = *req.IsCrossMargin
	}

	// 设置杠杆默认值
	btcEthLeverage := req.BTCETHLeverage
	altcoinLeverage := req.AltcoinLeverage
	if btcEthLeverage <= 0 {
		btcEthLeverage = existingTrader.BTCETHLeverage // 保持原值
	}
	if altcoinLeverage <= 0 {
		altcoinLeverage = existingTrader.AltcoinLeverage // 保持原值
	}

	// 设置扫描间隔，允许更新
	scanIntervalMinutes := req.ScanIntervalMinutes
	if scanIntervalMinutes <= 0 {
		scanIntervalMinutes = existingTrader.ScanIntervalMinutes // 保持原值
	} else if scanIntervalMinutes < 1 {
		scanIntervalMinutes = 1 // 最低1分钟，不允许小于1分钟
	}

	// 设置提示词模板，允许更新
	systemPromptTemplate := req.SystemPromptTemplate
	if systemPromptTemplate == "" {
		systemPromptTemplate = existingTrader.SystemPromptTemplate // 如果请求中没有提供，保持原值
	}

	// 设置信号源开关
	useCoinPool := existingTrader.UseCoinPool
	if req.UseCoinPool != nil {
		useCoinPool = *req.UseCoinPool
	}

	useOITop := existingTrader.UseOITop
	if req.UseOITop != nil {
		useOITop = *req.UseOITop
	}

	// 设置费率，允许更新
	takerFeeRate := req.TakerFeeRate
	makerFeeRate := req.MakerFeeRate

	// 如果用户未提供或为0，保持原有配置
	if takerFeeRate == 0 {
		if existingTrader.TakerFeeRate > 0 {
			takerFeeRate = existingTrader.TakerFeeRate // 保持原值
		} else {
			takerFeeRate = 0.0004 // 使用默认值
		}
	}
	if makerFeeRate == 0 {
		if existingTrader.MakerFeeRate > 0 {
			makerFeeRate = existingTrader.MakerFeeRate // 保持原值
		} else {
			makerFeeRate = 0.0002 // 使用默认值
		}
	}

	// 验证费率范围
	if takerFeeRate < 0 || takerFeeRate > 0.01 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Taker费率必须在0-1%之间"})
		return
	}
	if makerFeeRate < 0 || makerFeeRate > 0.01 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maker费率必须在0-1%之间"})
		return
	}

	// 记录费率变化
	if takerFeeRate != existingTrader.TakerFeeRate || makerFeeRate != existingTrader.MakerFeeRate {
		log.Printf("✓ 更新费率配置: Taker %.4f→%.4f, Maker %.4f→%.4f",
			existingTrader.TakerFeeRate, takerFeeRate,
			existingTrader.MakerFeeRate, makerFeeRate)
	}

	// 设置订单策略，允许更新
	orderStrategy := req.OrderStrategy
	if orderStrategy == "" {
		if existingTrader.OrderStrategy != "" {
			orderStrategy = existingTrader.OrderStrategy // 保持原值
		} else {
			orderStrategy = "conservative_hybrid" // 使用默认值
		}
	}

	// 设置限价偏移，允许更新
	limitPriceOffset := req.LimitPriceOffset
	if limitPriceOffset == 0 {
		if existingTrader.LimitPriceOffset != 0 {
			limitPriceOffset = existingTrader.LimitPriceOffset // 保持原值
		} else {
			limitPriceOffset = -0.03 // 使用默认值
		}
	}

	// 设置限价超时，允许更新
	limitTimeoutSeconds := req.LimitTimeoutSeconds
	if limitTimeoutSeconds == 0 {
		if existingTrader.LimitTimeoutSeconds > 0 {
			limitTimeoutSeconds = existingTrader.LimitTimeoutSeconds // 保持原值
		} else {
			limitTimeoutSeconds = 60 // 使用默认值
		}
	}

	// 设置时间线选择，允许更新
	timeframes := req.Timeframes
	if timeframes == "" {
		if existingTrader.Timeframes != "" {
			timeframes = existingTrader.Timeframes // 保持原值
		} else {
			timeframes = "4h" // 使用默认值
		}
	}

	// 查询 AI Model 和 Exchange 的自增 ID
	aiModels, err := s.database.GetAIModels(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取AI模型配置失败"})
		return
	}

	var aiModelIntID int
	var aiModelFound bool
	for _, model := range aiModels {
		if model.ModelID == req.AIModelID {
			aiModelIntID = model.ID
			aiModelFound = true
			break
		}
	}
	if !aiModelFound {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("AI模型 %s 不存在", req.AIModelID)})
		return
	}

	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易所配置失败"})
		return
	}

	var exchangeIntID int
	var exchangeFound bool
	for _, exchange := range exchanges {
		if exchange.ExchangeID == req.ExchangeID {
			exchangeIntID = exchange.ID
			exchangeFound = true
			break
		}
	}
	if !exchangeFound {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("交易所 %s 不存在", req.ExchangeID)})
		return
	}

	// 更新交易员配置
	trader := &config.TraderRecord{
		ID:                   traderID,
		UserID:               userID,
		Name:                 req.Name,
		AIModelID:            aiModelIntID,  // 使用查询到的自增 ID
		ExchangeID:           exchangeIntID, // 使用查询到的自增 ID
		InitialBalance:       req.InitialBalance,
		BTCETHLeverage:       btcEthLeverage,
		AltcoinLeverage:      altcoinLeverage,
		TradingSymbols:       req.TradingSymbols,
		UseCoinPool:          useCoinPool,
		UseOITop:             useOITop,
		CustomPrompt:         req.CustomPrompt,
		OverrideBasePrompt:   req.OverrideBasePrompt,
		SystemPromptTemplate: systemPromptTemplate,
		IsCrossMargin:        isCrossMargin,
		ScanIntervalMinutes:  scanIntervalMinutes,
		TakerFeeRate:         takerFeeRate,             // 添加 Taker 费率
		MakerFeeRate:         makerFeeRate,             // 添加 Maker 费率
		OrderStrategy:        orderStrategy,            // 添加订单策略
		LimitPriceOffset:     limitPriceOffset,         // 添加限价偏移
		LimitTimeoutSeconds:  limitTimeoutSeconds,      // 添加限价超时
		Timeframes:           timeframes,               // 添加时间线选择
		IsRunning:            existingTrader.IsRunning, // 保持原值
	}

	// 更新数据库
	err = s.database.UpdateTrader(trader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易员失败: %v", err)})
		return
	}

	// 如果请求中包含initial_balance且与现有值不同，单独更新它
	// UpdateTrader不会更新initial_balance，需要使用专门的方法
	if req.InitialBalance > 0 && math.Abs(req.InitialBalance-existingTrader.InitialBalance) > 0.1 {
		err = s.database.UpdateTraderInitialBalance(userID, traderID, req.InitialBalance)
		if err != nil {
			log.Printf("⚠️ 更新初始余额失败: %v", err)
			// 不返回错误，因为主要配置已更新成功
		} else {
			log.Printf("✓ 初始余额已更新: %.2f -> %.2f", existingTrader.InitialBalance, req.InitialBalance)
		}
	}

	// 🔄 从内存中移除旧的trader实例，以便重新加载最新配置
	_ = s.traderManager.RemoveTrader(traderID) // 忽略錯誤，trader可能不在內存中

	// 重新加载交易员到内存
	err = s.traderManager.LoadTraderByID(s.database, userID, traderID)
	if err != nil {
		log.Printf("⚠️ 重新加载交易员到内存失败: %v", err)
	}

	log.Printf("✓ 更新交易员成功: %s (模型: %s, 交易所: %s)", req.Name, req.AIModelID, req.ExchangeID)

	c.JSON(http.StatusOK, gin.H{
		"trader_id":   traderID,
		"trader_name": req.Name,
		"ai_model":    req.AIModelID,
		"message":     "交易员更新成功",
	})
}

// handleDeleteTrader 删除交易员
func (s *Server) handleDeleteTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	// ✅ 步骤1：先从内存中停止并移除交易员（RemoveTrader会处理停止逻辑和竞赛缓存清除）
	if removeErr := s.traderManager.RemoveTrader(traderID); removeErr != nil {
		// 交易员不在内存中也不是错误，可能已经被移除或从未加载
		log.Printf("⚠️ 从内存中移除交易员时出现警告: %v", removeErr)
	}

	// ✅ 步骤2：最后才从数据库删除
	err = s.database.DeleteTrader(userID, traderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除交易员失败: %v", err)})
		return
	}

	log.Printf("✓ 交易员已完全删除: %s", traderID)
	c.JSON(http.StatusOK, gin.H{"message": "交易员已删除"})
}

// handleStartTrader 启动交易员
func (s *Server) handleStartTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 确保用户的交易员已加载到内存中（修复 404 问题）
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	// 校验交易员是否属于当前用户
	traderRecord, _, _, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	// 获取模板名称
	templateName := traderRecord.SystemPromptTemplate

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 检查交易员是否已经在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已在运行中"})
		return
	}

	// 重新加载系统提示词模板（确保使用最新的硬盘文件）
	s.reloadPromptTemplatesWithLog(templateName)

	// 启动交易员
	go func() {
		log.Printf("▶️  启动交易员 %s (%s)", traderID, trader.GetName())
		if err := trader.Run(); err != nil {
			log.Printf("❌ 交易员 %s 运行错误: %v", trader.GetName(), err)
		}
	}()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(userID, traderID, true)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("✓ 交易员 %s 已启动", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已启动"})
}

// handleStopTrader 停止交易员
func (s *Server) handleStopTrader(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	// 校验交易员是否属于当前用户
	_, _, _, err = s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在或无访问权限"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 检查交易员是否正在运行
	status := trader.GetStatus()
	if isRunning, ok := status["is_running"].(bool); ok && !isRunning {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员已停止"})
		return
	}

	// 停止交易员
	trader.Stop()

	// 更新数据库中的运行状态
	err = s.database.UpdateTraderStatus(userID, traderID, false)
	if err != nil {
		log.Printf("⚠️  更新交易员状态失败: %v", err)
	}

	log.Printf("⏹  交易员 %s 已停止", trader.GetName())
	c.JSON(http.StatusOK, gin.H{"message": "交易员已停止"})
}

// handleUpdateTraderPrompt 更新交易员自定义Prompt
func (s *Server) handleUpdateTraderPrompt(c *gin.Context) {
	traderID := c.Param("id")
	userID := c.GetString("user_id")

	// 确保用户的交易员已加载到内存中（修复 404 问题）
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	var req struct {
		CustomPrompt       string `json:"custom_prompt"`
		OverrideBasePrompt bool   `json:"override_base_prompt"`
	}

	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": bindErr.Error()})
		return
	}

	// 更新数据库
	err = s.database.UpdateTraderCustomPrompt(userID, traderID, req.CustomPrompt, req.OverrideBasePrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新自定义prompt失败: %v", err)})
		return
	}

	// 如果trader在内存中，更新其custom prompt和override设置
	trader, err := s.traderManager.GetTrader(traderID)
	if err == nil {
		trader.SetCustomPrompt(req.CustomPrompt)
		trader.SetOverrideBasePrompt(req.OverrideBasePrompt)
		log.Printf("✓ 已更新交易员 %s 的自定义prompt (覆盖基础=%v)", trader.GetName(), req.OverrideBasePrompt)
	}

	c.JSON(http.StatusOK, gin.H{"message": "自定义prompt已更新"})
}

// handleSyncBalance 同步交易所余额到initial_balance（选项B：手动同步 + 选项C：智能检测）
func (s *Server) handleSyncBalance(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	// 确保用户的交易员已加载到内存中（修复 404 问题）
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	log.Printf("🔄 用户 %s 请求同步交易员 %s 的余额", userID, traderID)

	// 从数据库获取交易员配置（包含交易所信息）
	traderConfig, _, exchangeCfg, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	if exchangeCfg == nil || !exchangeCfg.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易所未配置或未启用"})
		return
	}

	// 创建临时 trader 查询余额
	var tempTrader trader.Trader
	var createErr error

	switch exchangeCfg.ExchangeID {
	case "binance":
		// 使用默认订单策略（查询余额不需要实际下单）
		tempTrader = trader.NewFuturesTrader(exchangeCfg.APIKey, exchangeCfg.SecretKey, userID, "market_only", -0.03, 60)
	case "hyperliquid":
		tempTrader, createErr = trader.NewHyperliquidTrader(
			exchangeCfg.APIKey,
			exchangeCfg.HyperliquidWalletAddr,
			exchangeCfg.Testnet,
		)
	case "aster":
		tempTrader, createErr = trader.NewAsterTrader(
			exchangeCfg.AsterUser,
			exchangeCfg.AsterSigner,
			exchangeCfg.AsterPrivateKey,
		)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的交易所类型"})
		return
	}

	if createErr != nil {
		log.Printf("⚠️ 创建临时 trader 失败: %v", createErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("连接交易所失败: %v", createErr)})
		return
	}

	// 查询实际余额
	balanceInfo, balanceErr := tempTrader.GetBalance()
	if balanceErr != nil {
		log.Printf("⚠️ 查询交易所余额失败: %v", balanceErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("查询余额失败: %v", balanceErr)})
		return
	}

	// ✅ 使用总资产（total equity）而不是可用余额
	// 总资产 = 钱包余额 + 未实现盈亏，这样才能正确计算总盈亏
	var actualBalance float64
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0

	if wallet, ok := balanceInfo["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}
	if unrealized, ok := balanceInfo["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}

	// 总资产 = 钱包余额 + 未实现盈亏
	totalEquity := totalWalletBalance + totalUnrealizedProfit

	if totalEquity > 0 {
		actualBalance = totalEquity
		log.Printf("✓ 查询到交易所总资产余额: %.2f USDT (钱包: %.2f + 未实现: %.2f)",
			actualBalance, totalWalletBalance, totalUnrealizedProfit)
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取总资产余额"})
		return
	}

	oldBalance := traderConfig.InitialBalance

	// ✅ 选项C：智能检测余额变化
	changePercent := ((actualBalance - oldBalance) / oldBalance) * 100
	changeType := "增加"
	if changePercent < 0 {
		changeType = "减少"
	}

	log.Printf("✓ 查询到交易所实际余额: %.2f USDT (当前配置: %.2f USDT, 变化: %.2f%%)",
		actualBalance, oldBalance, changePercent)

	// 更新数据库中的 initial_balance
	err = s.database.UpdateTraderInitialBalance(userID, traderID, actualBalance)
	if err != nil {
		log.Printf("❌ 更新initial_balance失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新余额失败"})
		return
	}

	// 重新加载交易员到内存
	err = s.traderManager.LoadTraderByID(s.database, userID, traderID)
	if err != nil {
		log.Printf("⚠️ 重新加载交易员到内存失败: %v", err)
	}

	log.Printf("✅ 已同步余额: %.2f → %.2f USDT (%s %.2f%%)", oldBalance, actualBalance, changeType, changePercent)

	c.JSON(http.StatusOK, gin.H{
		"message":        "余额同步成功",
		"old_balance":    oldBalance,
		"new_balance":    actualBalance,
		"change_percent": changePercent,
		"change_type":    changeType,
	})
}

// handleGetModelConfigs 获取AI模型配置
func (s *Server) handleGetModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的AI模型配置", userID)
	models, err := s.database.GetAIModels(userID)
	if err != nil {
		log.Printf("❌ 获取AI模型配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取AI模型配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个AI模型配置", len(models))

	// 转换为安全的响应结构，移除敏感信息
	safeModels := make([]SafeModelConfig, len(models))
	for i, model := range models {
		safeModels[i] = SafeModelConfig{
			ID:              model.ModelID, // 返回 model_id（例如 "deepseek"）而不是自增 ID
			Name:            model.Name,
			Provider:        model.Provider,
			Enabled:         model.Enabled,
			CustomAPIURL:    model.CustomAPIURL,
			CustomModelName: model.CustomModelName,
		}
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleUpdateModelConfigs 更新AI模型配置（仅支持加密数据）
func (s *Server) handleUpdateModelConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// 读取原始请求体
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}

	// 解析加密的 payload
	var encryptedPayload crypto.EncryptedPayload
	if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
		log.Printf("❌ 解析加密载荷失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误，必须使用加密传输"})
		return
	}

	// 验证是否为加密数据
	if encryptedPayload.WrappedKey == "" {
		log.Printf("❌ 检测到非加密请求 (UserID: %s)", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "此接口仅支持加密传输，请使用加密客户端",
			"code":    "ENCRYPTION_REQUIRED",
			"message": "Encrypted transmission is required for security reasons",
		})
		return
	}

	// 解密数据
	decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
	if err != nil {
		log.Printf("❌ 解密模型配置失败 (UserID: %s): %v", userID, err)
		// 根据错误类型提供更具体的错误信息
		errMsg := "解密数据失败"
		if strings.Contains(err.Error(), "timestamp") {
			errMsg = "时间戳验证失败：请检查系统时间是否正确"
		} else if strings.Contains(err.Error(), "unwrap") || strings.Contains(err.Error(), "RSA") {
			errMsg = "密钥解密失败：请刷新页面重试"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	// 解析解密后的数据
	var req UpdateModelConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		log.Printf("❌ 解析解密数据失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析解密数据失败"})
		return
	}
	log.Printf("🔓 已解密模型配置数据 (UserID: %s)", userID)

	// 更新每个模型的配置
	for modelID, modelData := range req.Models {
		err := s.database.UpdateAIModel(userID, modelID, modelData.Enabled, modelData.APIKey, modelData.CustomAPIURL, modelData.CustomModelName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新模型 %s 失败: %v", modelID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为模型配置已经成功更新到数据库
	}

	log.Printf("✓ AI模型配置已更新: %+v", SanitizeModelConfigForLog(req.Models))
	c.JSON(http.StatusOK, gin.H{"message": "模型配置已更新"})
}

// handleGetExchangeConfigs 获取交易所配置
func (s *Server) handleGetExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")
	log.Printf("🔍 查询用户 %s 的交易所配置", userID)
	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		log.Printf("❌ 获取交易所配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易所配置失败: %v", err)})
		return
	}
	log.Printf("✅ 找到 %d 个交易所配置", len(exchanges))

	// 转换为安全的响应结构，移除敏感信息
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ExchangeID, // 返回 exchange_id（例如 "binance"）
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: exchange.HyperliquidWalletAddr,
			AsterUser:             exchange.AsterUser,
			AsterSigner:           exchange.AsterSigner,
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// handleUpdateExchangeConfigs 更新交易所配置（仅支持加密数据）
func (s *Server) handleUpdateExchangeConfigs(c *gin.Context) {
	userID := c.GetString("user_id")

	// 读取原始请求体
	bodyBytes, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}

	// 解析加密的 payload
	var encryptedPayload crypto.EncryptedPayload
	if err := json.Unmarshal(bodyBytes, &encryptedPayload); err != nil {
		log.Printf("❌ 解析加密载荷失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误，必须使用加密传输"})
		return
	}

	// 验证是否为加密数据
	if encryptedPayload.WrappedKey == "" {
		log.Printf("❌ 检测到非加密请求 (UserID: %s)", userID)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "此接口仅支持加密传输，请使用加密客户端",
			"code":    "ENCRYPTION_REQUIRED",
			"message": "Encrypted transmission is required for security reasons",
		})
		return
	}

	// 解密数据
	decrypted, err := s.cryptoHandler.cryptoService.DecryptSensitiveData(&encryptedPayload)
	if err != nil {
		log.Printf("❌ 解密交易所配置失败 (UserID: %s): %v", userID, err)
		// 根据错误类型提供更具体的错误信息
		errMsg := "解密数据失败"
		if strings.Contains(err.Error(), "timestamp") {
			errMsg = "时间戳验证失败：请检查系统时间是否正确"
		} else if strings.Contains(err.Error(), "unwrap") || strings.Contains(err.Error(), "RSA") {
			errMsg = "密钥解密失败：请刷新页面重试"
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	// 解析解密后的数据
	var req UpdateExchangeConfigRequest
	if err := json.Unmarshal([]byte(decrypted), &req); err != nil {
		log.Printf("❌ 解析解密数据失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "解析解密数据失败"})
		return
	}
	log.Printf("🔓 已解密交易所配置数据 (UserID: %s)", userID)

	// 更新每个交易所的配置
	for exchangeID, exchangeData := range req.Exchanges {
		err := s.database.UpdateExchange(userID, exchangeID, exchangeData.Enabled, exchangeData.APIKey, exchangeData.SecretKey, exchangeData.Testnet, exchangeData.HyperliquidWalletAddr, exchangeData.AsterUser, exchangeData.AsterSigner, exchangeData.AsterPrivateKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新交易所 %s 失败: %v", exchangeID, err)})
			return
		}
	}

	// 重新加载该用户的所有交易员，使新配置立即生效
	err = s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 重新加载用户交易员到内存失败: %v", err)
		// 这里不返回错误，因为交易所配置已经成功更新到数据库
	}

	log.Printf("✓ 交易所配置已更新: %+v", SanitizeExchangeConfigForLog(req.Exchanges))
	c.JSON(http.StatusOK, gin.H{"message": "交易所配置已更新"})
}

// handleGetUserSignalSource 获取用户信号源配置
func (s *Server) handleGetUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	source, err := s.database.GetUserSignalSource(userID)
	if err != nil {
		// 如果配置不存在，返回空配置而不是404错误
		c.JSON(http.StatusOK, gin.H{
			"coin_pool_url": "",
			"oi_top_url":    "",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"coin_pool_url": source.CoinPoolURL,
		"oi_top_url":    source.OITopURL,
	})
}

// handleSaveUserSignalSource 保存用户信号源配置
func (s *Server) handleSaveUserSignalSource(c *gin.Context) {
	userID := c.GetString("user_id")
	var req struct {
		CoinPoolURL string `json:"coin_pool_url"`
		OITopURL    string `json:"oi_top_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := s.database.CreateUserSignalSource(userID, req.CoinPoolURL, req.OITopURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("保存用户信号源配置失败: %v", err)})
		return
	}

	log.Printf("✓ 用户信号源配置已保存: user=%s, coin_pool=%s, oi_top=%s", userID, req.CoinPoolURL, req.OITopURL)
	c.JSON(http.StatusOK, gin.H{"message": "用户信号源配置已保存"})
}

// handleTraderList trader列表
func (s *Server) handleTraderList(c *gin.Context) {
	userID := c.GetString("user_id")
	traders, err := s.database.GetTraders(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取交易员列表失败: %v", err)})
		return
	}

	// 获取用户的所有 AI 模型和交易所配置，用于将整数 ID 映射到字符串 ID
	aiModels, err := s.database.GetAIModels(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取AI模型配置失败"})
		return
	}

	exchanges, err := s.database.GetExchanges(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取交易所配置失败"})
		return
	}

	// 创建映射：整数 ID -> 字符串 ModelID/ExchangeID
	aiModelMap := make(map[int]string)
	for _, model := range aiModels {
		aiModelMap[model.ID] = model.ModelID
	}

	exchangeMap := make(map[int]string)
	for _, exchange := range exchanges {
		exchangeMap[exchange.ID] = exchange.ExchangeID
	}

	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		// 获取实时运行状态
		isRunning := trader.IsRunning
		if at, err := s.traderManager.GetTrader(trader.ID); err == nil {
			status := at.GetStatus()
			if running, ok := status["is_running"].(bool); ok {
				isRunning = running
			}
		}

		// 返回 AI 模型的 ModelID（如 "deepseek", "qwen-chat"），而不是整数 ID
		// 前端需要使用 .includes() 方法来检查模型类型
		aiModelID := aiModelMap[trader.AIModelID]
		if aiModelID == "" {
			aiModelID = "unknown" // 如果找不到，返回默认值
		}

		// 返回交易所的 ExchangeID（如 "binance", "hyperliquid"），而不是整数 ID
		exchangeID := exchangeMap[trader.ExchangeID]
		if exchangeID == "" {
			exchangeID = "unknown" // 如果找不到，返回默认值
		}

		result = append(result, map[string]interface{}{
			"trader_id":              trader.ID,
			"trader_name":            trader.Name,
			"ai_model":               aiModelID,
			"exchange_id":            exchangeID,
			"is_running":             isRunning,
			"initial_balance":        trader.InitialBalance,
			"system_prompt_template": trader.SystemPromptTemplate,
			"scan_interval_minutes":  trader.ScanIntervalMinutes,
			"btc_eth_leverage":       trader.BTCETHLeverage,
			"altcoin_leverage":       trader.AltcoinLeverage,
			"trading_symbols":        trader.TradingSymbols,
			"custom_prompt":          trader.CustomPrompt,
			"override_base_prompt":   trader.OverrideBasePrompt,
			"is_cross_margin":        trader.IsCrossMargin,
			"use_coin_pool":          trader.UseCoinPool,
			"use_oi_top":             trader.UseOITop,
			"taker_fee_rate":         trader.TakerFeeRate,
			"maker_fee_rate":         trader.MakerFeeRate,
			"order_strategy":         trader.OrderStrategy,
			"limit_price_offset":     trader.LimitPriceOffset,
			"limit_timeout_seconds":  trader.LimitTimeoutSeconds,
			"timeframes":             trader.Timeframes,
		})
	}

	c.JSON(http.StatusOK, result)
}

// handleGetTraderConfig 获取交易员详细配置
func (s *Server) handleGetTraderConfig(c *gin.Context) {
	userID := c.GetString("user_id")
	traderID := c.Param("id")

	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	// 确保用户的交易员已加载到内存中（修复 404 问题）
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	traderConfig, aiModel, exchange, err := s.database.GetTraderConfig(userID, traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("获取交易员配置失败: %v", err)})
		return
	}

	// 获取实时运行状态
	isRunning := traderConfig.IsRunning
	if at, err := s.traderManager.GetTrader(traderID); err == nil {
		status := at.GetStatus()
		if running, ok := status["is_running"].(bool); ok {
			isRunning = running
		}
	}

	// 返回 AI 模型的 ModelID（如 "deepseek", "qwen-chat"），而不是整数 ID
	// 前端需要使用 .includes() 方法来检查模型类型
	aiModelID := aiModel.ModelID

	// 返回交易所的 ExchangeID（如 "binance", "hyperliquid"），而不是整数 ID
	exchangeID := exchange.ExchangeID

	result := map[string]interface{}{
		"trader_id":              traderConfig.ID,
		"trader_name":            traderConfig.Name,
		"ai_model":               aiModelID,
		"exchange_id":            exchangeID,
		"initial_balance":        traderConfig.InitialBalance,
		"scan_interval_minutes":  traderConfig.ScanIntervalMinutes,
		"btc_eth_leverage":       traderConfig.BTCETHLeverage,
		"altcoin_leverage":       traderConfig.AltcoinLeverage,
		"trading_symbols":        traderConfig.TradingSymbols,
		"custom_prompt":          traderConfig.CustomPrompt,
		"override_base_prompt":   traderConfig.OverrideBasePrompt,
		"system_prompt_template": traderConfig.SystemPromptTemplate,
		"is_cross_margin":        traderConfig.IsCrossMargin,
		"use_coin_pool":          traderConfig.UseCoinPool,
		"use_oi_top":             traderConfig.UseOITop,
		"is_running":             isRunning,
		"taker_fee_rate":         traderConfig.TakerFeeRate,
		"maker_fee_rate":         traderConfig.MakerFeeRate,
		"order_strategy":         traderConfig.OrderStrategy,
		"limit_price_offset":     traderConfig.LimitPriceOffset,
		"limit_timeout_seconds":  traderConfig.LimitTimeoutSeconds,
		"timeframes":             traderConfig.Timeframes,
	}

	c.JSON(http.StatusOK, result)
}

// handleStatus 系统状态
func (s *Server) handleStatus(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	status := trader.GetStatus()
	c.JSON(http.StatusOK, status)
}

// handleAccount 账户信息
func (s *Server) handleAccount(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📊 收到账户信息请求 [%s]", trader.GetName())
	account, err := trader.GetAccountInfo()
	if err != nil {
		log.Printf("❌ 获取账户信息失败 [%s]: %v", trader.GetName(), err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取账户信息失败: %v", err),
		})
		return
	}

	log.Printf("✓ 返回账户信息 [%s]: 净值=%.2f, 可用=%.2f, 盈亏=%.2f (%.2f%%)",
		trader.GetName(),
		account["total_equity"],
		account["available_balance"],
		account["total_pnl"],
		account["total_pnl_pct"])
	c.JSON(http.StatusOK, account)
}

// handlePositions 持仓列表
func (s *Server) handlePositions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	positions, err := trader.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取持仓列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, positions)
}

// handleDecisions 决策日志列表
func (s *Server) handleDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取所有历史决策记录（无限制）
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, records)
}

// handleLatestDecisions 最新决策日志（最近5条，最新的在前）
func (s *Server) handleLatestDecisions(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 从 query 参数读取 limit，默认 5，最大 50
	limit := 5
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	records, err := trader.GetDecisionLogger().GetLatestRecords(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取决策日志失败: %v", err),
		})
		return
	}

	// 反转数组，让最新的在前面（用于列表显示）
	// GetLatestRecords返回的是从旧到新（用于图表），这里需要从新到旧
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	c.JSON(http.StatusOK, records)
}

// handleStatistics 统计信息
func (s *Server) handleStatistics(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	stats, err := trader.GetDecisionLogger().GetStatistics()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取统计信息失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleCompetition 竞赛总览（对比所有trader）
func (s *Server) handleCompetition(c *gin.Context) {
	userID := c.GetString("user_id")

	// 确保用户的交易员已加载到内存中
	err := s.traderManager.LoadUserTraders(s.database, userID)
	if err != nil {
		log.Printf("⚠️ 加载用户 %s 的交易员失败: %v", userID, err)
	}

	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleEquityHistory 收益率历史数据
func (s *Server) handleEquityHistory(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 获取尽可能多的历史数据（几天的数据）
	// 每3分钟一个周期：10000条 = 约20天的数据
	records, err := trader.GetDecisionLogger().GetLatestRecords(10000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取历史数据失败: %v", err),
		})
		return
	}

	// 构建收益率历史数据点
	type EquityPoint struct {
		Timestamp        string  `json:"timestamp"`
		TotalEquity      float64 `json:"total_equity"`      // 账户净值（wallet + unrealized）
		AvailableBalance float64 `json:"available_balance"` // 可用余额
		TotalPnL         float64 `json:"total_pnl"`         // 总盈亏（相对初始余额）
		TotalPnLPct      float64 `json:"total_pnl_pct"`     // 总盈亏百分比
		PositionCount    int     `json:"position_count"`    // 持仓数量
		MarginUsedPct    float64 `json:"margin_used_pct"`   // 保证金使用率
		CycleNumber      int     `json:"cycle_number"`
	}

	// 从AutoTrader获取当前初始余额（用作旧数据的fallback）
	base := 0.0
	if status := trader.GetStatus(); status != nil {
		if ib, ok := status["initial_balance"].(float64); ok && ib > 0 {
			base = ib
		}
	}

	// 如果还是无法获取，返回错误
	if base == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取初始余额",
		})
		return
	}

	var history []EquityPoint
	for _, record := range records {
		// TotalBalance字段实际存储的是TotalEquity
		// totalEquity := record.AccountState.TotalBalance
		// TotalUnrealizedProfit字段实际存储的是TotalPnL（相对初始余额）
		// totalPnL := record.AccountState.TotalUnrealizedProfit
		walletBalance := record.AccountState.TotalBalance
		unrealizedPnL := record.AccountState.TotalUnrealizedProfit
		totalEquity := walletBalance + unrealizedPnL

		// 🔄 使用历史记录中保存的initial_balance（如果有）
		// 这样可以保持历史PNL%的准确性，即使用户后来更新了initial_balance
		if record.AccountState.InitialBalance > 0 {
			base = record.AccountState.InitialBalance
		}

		totalPnL := totalEquity - base
		// 计算盈亏百分比
		totalPnLPct := 0.0
		if base > 0 {
			totalPnLPct = (totalPnL / base) * 100
		}

		history = append(history, EquityPoint{
			Timestamp:        record.Timestamp.Format("2006-01-02 15:04:05"),
			TotalEquity:      totalEquity,
			AvailableBalance: record.AccountState.AvailableBalance,
			TotalPnL:         totalPnL,
			TotalPnLPct:      totalPnLPct,
			PositionCount:    record.AccountState.PositionCount,
			MarginUsedPct:    record.AccountState.MarginUsedPct,
			CycleNumber:      record.CycleNumber,
		})
	}

	c.JSON(http.StatusOK, history)
}

// handlePerformance AI历史表现分析（用于展示AI学习和反思）
func (s *Server) handlePerformance(c *gin.Context) {
	_, traderID, err := s.getTraderFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 分析最近100个周期的交易表现（避免长期持仓的交易记录丢失）
	// 假设每3分钟一个周期，100个周期 = 5小时，足够覆盖大部分交易
	performance, err := trader.GetDecisionLogger().AnalyzePerformance(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("分析历史表现失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, performance)
}

// authMiddleware JWT认证中间件
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
			c.Abort()
			return
		}

		// 检查Bearer token格式
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// 黑名单检查
		if auth.IsTokenBlacklisted(tokenString) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token已失效，请重新登录"})
			c.Abort()
			return
		}

		// 验证JWT token
		claims, err := auth.ValidateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token: " + err.Error()})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}

// handleLogout 将当前token加入黑名单
func (s *Server) handleLogout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "缺少Authorization头"})
		return
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的Authorization格式"})
		return
	}
	tokenString := parts[1]
	claims, err := auth.ValidateJWT(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的token"})
		return
	}
	var exp time.Time
	if claims.ExpiresAt != nil {
		exp = claims.ExpiresAt.Time
	} else {
		exp = time.Now().Add(24 * time.Hour)
	}
	auth.BlacklistToken(tokenString, exp)
	c.JSON(http.StatusOK, gin.H{"message": "已登出"})
}

// handleRegister 处理用户注册请求
func (s *Server) handleRegister(c *gin.Context) {
	clientIP := c.ClientIP()
	log.Printf("📝 [Register] 收到注册请求 (IP: %s, X-Forwarded-For: %s, X-Real-IP: %s)",
		clientIP, c.GetHeader("X-Forwarded-For"), c.GetHeader("X-Real-IP"))

	regEnabled := true
	if regStr, err := s.database.GetSystemConfig("registration_enabled"); err == nil {
		regEnabled = strings.ToLower(regStr) != "false"
	}
	if !regEnabled {
		log.Printf("⚠️ [Register] 注册已关闭 (IP: %s)", clientIP)
		c.JSON(http.StatusForbidden, gin.H{"error": "注册已关闭"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6"`
		BetaCode string `json:"beta_code"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ [Register] 请求格式错误 (IP: %s): %v", clientIP, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("📧 [Register] 处理邮箱: %s (IP: %s)", req.Email, clientIP)

	// 检查是否开启了内测模式
	betaModeStr, _ := s.database.GetSystemConfig("beta_mode")
	log.Printf("🔒 [Register] beta_mode=%s", betaModeStr)
	if betaModeStr == "true" {
		// 内测模式下必须提供有效的内测码
		if req.BetaCode == "" {
			log.Printf("⚠️ [Register] 内测模式但未提供内测码 (Email: %s)", req.Email)
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测期间，注册需要提供内测码"})
			return
		}

		// 验证内测码
		isValid, err := s.database.ValidateBetaCode(req.BetaCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "验证内测码失败"})
			return
		}
		if !isValid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "内测码无效或已被使用"})
			return
		}
	}

	// 检查邮箱是否已存在
	existingUser, err := s.database.GetUserByEmail(req.Email)
	if err == nil {
		// 如果用户未完成OTP验证，允许重新获取OTP（支持中断后恢复注册）
		if !existingUser.OTPVerified {
			qrCodeURL := auth.GetOTPQRCodeURL(existingUser.OTPSecret, req.Email)
			c.JSON(http.StatusOK, gin.H{
				"user_id":     existingUser.ID,
				"email":       req.Email,
				"otp_secret":  existingUser.OTPSecret,
				"qr_code_url": qrCodeURL,
				"message":     "检测到未完成的注册，请继续完成OTP设置",
			})
			return
		}
		// 用户已完成验证，拒绝重复注册
		c.JSON(http.StatusConflict, gin.H{"error": "邮箱已被注册"})
		return
	}

	// 生成密码哈希
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 生成OTP密钥
	otpSecret, err := auth.GenerateOTPSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OTP密钥生成失败"})
		return
	}

	// 创建用户（未验证OTP状态）
	userID := uuid.New().String()
	user := &config.User{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: passwordHash,
		OTPSecret:    otpSecret,
		OTPVerified:  false,
	}

	err = s.database.CreateUser(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建用户失败: " + err.Error()})
		return
	}

	// 如果是内测模式，标记内测码为已使用
	betaModeStr2, _ := s.database.GetSystemConfig("beta_mode")
	if betaModeStr2 == "true" && req.BetaCode != "" {
		err := s.database.UseBetaCode(req.BetaCode, req.Email)
		if err != nil {
			log.Printf("⚠️ 标记内测码为已使用失败: %v", err)
			// 这里不返回错误，因为用户已经创建成功
		} else {
			log.Printf("✓ 内测码 %s 已被用户 %s 使用", req.BetaCode, req.Email)
		}
	}

	// 返回OTP设置信息
	qrCodeURL := auth.GetOTPQRCodeURL(otpSecret, req.Email)
	c.JSON(http.StatusOK, gin.H{
		"user_id":     userID,
		"email":       req.Email,
		"otp_secret":  otpSecret,
		"qr_code_url": qrCodeURL,
		"message":     "请使用Google Authenticator扫描二维码并验证OTP",
	})
}

// handleCompleteRegistration 完成注册（验证OTP）
func (s *Server) handleCompleteRegistration(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP验证码错误"})
		return
	}

	// 更新用户OTP验证状态
	err = s.database.UpdateUserOTPVerified(req.UserID, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新用户状态失败"})
		return
	}

	// 生成 Access/Refresh Token
	tokenPair, err := auth.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	// 初始化用户的默认模型和交易所配置
	err = s.initUserDefaultConfigs(user.ID)
	if err != nil {
		log.Printf("初始化用户默认配置失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":              tokenPair.AccessToken, // 向後兼容舊版前端
		"access_token":       tokenPair.AccessToken,
		"refresh_token":      tokenPair.RefreshToken,
		"expires_in":         tokenPair.ExpiresIn,
		"refresh_expires_in": tokenPair.RefreshExpiresIn,
		"user_id":            user.ID,
		"email":              user.Email,
		"message":            "注册完成",
	})
}

// handleLogin 处理用户登录请求
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 验证密码
	if !auth.CheckPassword(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "邮箱或密码错误"})
		return
	}

	// 检查OTP是否已验证
	if !user.OTPVerified {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":              "账户未完成OTP设置",
			"user_id":            user.ID,
			"requires_otp_setup": true,
		})
		return
	}

	// 返回需要OTP验证的状态
	c.JSON(http.StatusOK, gin.H{
		"user_id":      user.ID,
		"email":        user.Email,
		"message":      "请输入Google Authenticator验证码",
		"requires_otp": true,
	})
}

// handleVerifyOTP 验证OTP并完成登录
func (s *Server) handleVerifyOTP(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		OTPCode string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取用户信息
	user, err := s.database.GetUserByID(req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// 验证OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误"})
		return
	}

	// 生成新的 Token Pair（Access + Refresh）
	tokenPair, err := auth.GenerateTokenPair(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成token失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":              tokenPair.AccessToken, // 向後兼容舊版前端
		"access_token":       tokenPair.AccessToken,
		"refresh_token":      tokenPair.RefreshToken,
		"expires_in":         tokenPair.ExpiresIn,
		"refresh_expires_in": tokenPair.RefreshExpiresIn,
		"user_id":            user.ID,
		"email":              user.Email,
		"message":            "登录成功",
	})
}

// handleRefreshToken 刷新访问令牌（使用 Refresh Token 获取新的 Token Pair）
func (s *Server) handleRefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 refresh_token 参数"})
		return
	}

	// 调用 auth.RefreshAccessToken 刷新令牌（自动进行 Token Rotation）
	tokenPair, err := auth.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		log.Printf("❌ [AUTH] Refresh Token 刷新失败: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Refresh Token 无效或已过期"})
		return
	}

	log.Printf("✓ [AUTH] Token 刷新成功")

	c.JSON(http.StatusOK, gin.H{
		"access_token":       tokenPair.AccessToken,
		"refresh_token":      tokenPair.RefreshToken,
		"expires_in":         tokenPair.ExpiresIn,
		"refresh_expires_in": tokenPair.RefreshExpiresIn,
		"token_type":         "Bearer",
		"message":            "Token 刷新成功",
	})
}

// handleResetPassword 重置密码（通过邮箱 + OTP 验证）
func (s *Server) handleResetPassword(c *gin.Context) {
	var req struct {
		Email       string `json:"email" binding:"required,email"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
		OTPCode     string `json:"otp_code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询用户
	user, err := s.database.GetUserByEmail(req.Email)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "邮箱不存在"})
		return
	}

	// 验证 OTP
	if !auth.VerifyOTP(user.OTPSecret, req.OTPCode) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google Authenticator 验证码错误"})
		return
	}

	// 生成新密码哈希
	newPasswordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}

	// 更新密码
	err = s.database.UpdateUserPassword(user.ID, newPasswordHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码更新失败"})
		return
	}

	log.Printf("✓ 用户 %s 密码已重置", user.Email)
	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功，请使用新密码登录"})
}

// initUserDefaultConfigs 为新用户初始化默认的模型和交易所配置
func (s *Server) initUserDefaultConfigs(userID string) error {
	// 注释掉自动创建默认配置，让用户手动添加
	// 这样新用户注册后不会自动有配置项
	log.Printf("用户 %s 注册完成，等待手动配置AI模型和交易所", userID)
	return nil
}

// handleGetSupportedModels 获取系统支持的AI模型列表
func (s *Server) handleGetSupportedModels(c *gin.Context) {
	// 返回系统支持的AI模型（从default用户获取）
	models, err := s.database.GetAIModels("default")
	if err != nil {
		log.Printf("❌ 获取支持的AI模型失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的AI模型失败"})
		return
	}

	// 转换为安全的响应结构，移除敏感信息
	safeModels := make([]SafeModelConfig, len(models))
	for i, model := range models {
		safeModels[i] = SafeModelConfig{
			ID:              model.ModelID, // 返回 model_id（例如 "deepseek"）而不是自增 ID
			Name:            model.Name,
			Provider:        model.Provider,
			Enabled:         model.Enabled,
			CustomAPIURL:    model.CustomAPIURL,
			CustomModelName: model.CustomModelName,
		}
	}

	c.JSON(http.StatusOK, safeModels)
}

// handleGetSupportedExchanges 获取系统支持的交易所列表
func (s *Server) handleGetSupportedExchanges(c *gin.Context) {
	// 返回系统支持的交易所（从default用户获取）
	exchanges, err := s.database.GetExchanges("default")
	if err != nil {
		log.Printf("❌ 获取支持的交易所失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取支持的交易所失败"})
		return
	}

	// 转换为安全的响应结构，移除敏感信息
	safeExchanges := make([]SafeExchangeConfig, len(exchanges))
	for i, exchange := range exchanges {
		safeExchanges[i] = SafeExchangeConfig{
			ID:                    exchange.ExchangeID, // 返回 exchange_id（例如 "binance"）
			Name:                  exchange.Name,
			Type:                  exchange.Type,
			Enabled:               exchange.Enabled,
			Testnet:               exchange.Testnet,
			HyperliquidWalletAddr: "", // 默认配置不包含钱包地址
			AsterUser:             "", // 默认配置不包含用户信息
			AsterSigner:           "",
		}
	}

	c.JSON(http.StatusOK, safeExchanges)
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🌐 API服务器启动在 http://localhost%s", addr)
	log.Printf("📊 API文档:")
	log.Printf("  • GET  /api/health           - 健康检查")
	log.Printf("  • GET  /api/traders          - 公开的AI交易员排行榜前50名（无需认证）")
	log.Printf("  • GET  /api/competition      - 公开的竞赛数据（无需认证）")
	log.Printf("  • GET  /api/top-traders      - 前5名交易员数据（无需认证，表现对比用）")
	log.Printf("  • GET  /api/equity-history?trader_id=xxx - 公开的收益率历史数据（无需认证，竞赛用）")
	log.Printf("  • GET  /api/equity-history-batch?trader_ids=a,b,c - 批量获取历史数据（无需认证，表现对比优化）")
	log.Printf("  • GET  /api/traders/:id/public-config - 公开的交易员配置（无需认证，不含敏感信息）")
	log.Printf("  • POST /api/traders          - 创建新的AI交易员")
	log.Printf("  • DELETE /api/traders/:id    - 删除AI交易员")
	log.Printf("  • POST /api/traders/:id/start - 启动AI交易员")
	log.Printf("  • POST /api/traders/:id/stop  - 停止AI交易员")
	log.Printf("  • GET  /api/models           - 获取AI模型配置")
	log.Printf("  • PUT  /api/models           - 更新AI模型配置")
	log.Printf("  • GET  /api/exchanges        - 获取交易所配置")
	log.Printf("  • PUT  /api/exchanges        - 更新交易所配置")
	log.Printf("  • GET  /api/status?trader_id=xxx     - 指定trader的系统状态")
	log.Printf("  • GET  /api/account?trader_id=xxx    - 指定trader的账户信息")
	log.Printf("  • GET  /api/positions?trader_id=xxx  - 指定trader的持仓列表")
	log.Printf("  • GET  /api/decisions?trader_id=xxx  - 指定trader的决策日志")
	log.Printf("  • GET  /api/decisions/latest?trader_id=xxx - 指定trader的最新决策")
	log.Printf("  • GET  /api/statistics?trader_id=xxx - 指定trader的统计信息")
	log.Printf("  • GET  /api/performance?trader_id=xxx - 指定trader的AI学习表现分析")
	log.Println()

	// 创建 http.Server 以支持 graceful shutdown
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown 优雅关闭 API 服务器
func (s *Server) Shutdown() error {
	if s.httpServer == nil {
		return nil
	}

	// 设置 5 秒超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// handleGetPromptTemplates 获取所有系统提示词模板列表
func (s *Server) handleGetPromptTemplates(c *gin.Context) {
	// 导入 decision 包
	templates := decision.GetAllPromptTemplates()

	// 转换为响应格式
	response := make([]map[string]interface{}, 0, len(templates))
	for _, tmpl := range templates {
		response = append(response, map[string]interface{}{
			"name":         tmpl.Name,
			"display_name": tmpl.DisplayName,
			"description":  tmpl.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"templates": response,
	})
}

// handleGetPromptTemplate 获取指定名称的提示词模板内容
func (s *Server) handleGetPromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	template, err := decision.GetPromptTemplate(templateName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateName)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    template.Name,
		"content": template.Content,
	})
}

// handlePublicTraderList 获取公开的交易员列表（无需认证）
func (s *Server) handlePublicTraderList(c *gin.Context) {
	// 从所有用户获取交易员信息
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取交易员列表失败: %v", err),
		})
		return
	}

	// 获取traders数组
	tradersData, exists := competition["traders"]
	if !exists {
		c.JSON(http.StatusOK, []map[string]interface{}{})
		return
	}

	traders, ok := tradersData.([]map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "交易员数据格式错误",
		})
		return
	}

	// 返回交易员基本信息，过滤敏感信息
	result := make([]map[string]interface{}, 0, len(traders))
	for _, trader := range traders {
		result = append(result, map[string]interface{}{
			"trader_id":              trader["trader_id"],
			"trader_name":            trader["trader_name"],
			"ai_model":               trader["ai_model"],
			"exchange":               trader["exchange"],
			"is_running":             trader["is_running"],
			"total_equity":           trader["total_equity"],
			"total_pnl":              trader["total_pnl"],
			"total_pnl_pct":          trader["total_pnl_pct"],
			"position_count":         trader["position_count"],
			"margin_used_pct":        trader["margin_used_pct"],
			"system_prompt_template": trader["system_prompt_template"],
		})
	}

	c.JSON(http.StatusOK, result)
}

// handlePublicCompetition 获取公开的竞赛数据（无需认证）
func (s *Server) handlePublicCompetition(c *gin.Context) {
	competition, err := s.traderManager.GetCompetitionData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取竞赛数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, competition)
}

// handleTopTraders 获取前5名交易员数据（无需认证，用于表现对比）
func (s *Server) handleTopTraders(c *gin.Context) {
	topTraders, err := s.traderManager.GetTopTradersData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取前10名交易员数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, topTraders)
}

// handleEquityHistoryBatch 批量获取多个交易员的收益率历史数据（无需认证，用于表现对比）
func (s *Server) handleEquityHistoryBatch(c *gin.Context) {
	var requestBody struct {
		TraderIDs []string `json:"trader_ids"`
	}

	// 尝试解析POST请求的JSON body
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		// 如果JSON解析失败，尝试从query参数获取（兼容GET请求）
		traderIDsParam := c.Query("trader_ids")
		if traderIDsParam == "" {
			// 如果没有指定trader_ids，则返回前5名的历史数据
			topTraders, err := s.traderManager.GetTopTradersData()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("获取前5名交易员失败: %v", err),
				})
				return
			}

			traders, ok := topTraders["traders"].([]map[string]interface{})
			if !ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "交易员数据格式错误"})
				return
			}

			// 提取trader IDs
			traderIDs := make([]string, 0, len(traders))
			for _, trader := range traders {
				if traderID, ok := trader["trader_id"].(string); ok {
					traderIDs = append(traderIDs, traderID)
				}
			}

			result := s.getEquityHistoryForTraders(traderIDs)
			c.JSON(http.StatusOK, result)
			return
		}

		// 解析逗号分隔的trader IDs
		requestBody.TraderIDs = strings.Split(traderIDsParam, ",")
		for i := range requestBody.TraderIDs {
			requestBody.TraderIDs[i] = strings.TrimSpace(requestBody.TraderIDs[i])
		}
	}

	// 限制最多20个交易员，防止请求过大
	if len(requestBody.TraderIDs) > 20 {
		requestBody.TraderIDs = requestBody.TraderIDs[:20]
	}

	result := s.getEquityHistoryForTraders(requestBody.TraderIDs)
	c.JSON(http.StatusOK, result)
}

// getEquityHistoryForTraders 获取多个交易员的历史数据
func (s *Server) getEquityHistoryForTraders(traderIDs []string) map[string]interface{} {
	result := make(map[string]interface{})
	histories := make(map[string]interface{})
	errors := make(map[string]string)

	for _, traderID := range traderIDs {
		if traderID == "" {
			continue
		}

		trader, err := s.traderManager.GetTrader(traderID)
		if err != nil {
			errors[traderID] = "交易员不存在"
			continue
		}

		// 获取历史数据（用于对比展示，限制数据量）
		records, err := trader.GetDecisionLogger().GetLatestRecords(500)
		if err != nil {
			errors[traderID] = fmt.Sprintf("获取历史数据失败: %v", err)
			continue
		}

		// 构建收益率历史数据
		history := make([]map[string]interface{}, 0, len(records))
		for _, record := range records {
			// 计算总权益（余额+未实现盈亏）
			totalEquity := record.AccountState.TotalBalance + record.AccountState.TotalUnrealizedProfit

			history = append(history, map[string]interface{}{
				"timestamp":    record.Timestamp,
				"total_equity": totalEquity,
				"total_pnl":    record.AccountState.TotalUnrealizedProfit,
				"balance":      record.AccountState.TotalBalance,
			})
		}

		histories[traderID] = history
	}

	result["histories"] = histories
	result["count"] = len(histories)
	if len(errors) > 0 {
		result["errors"] = errors
	}

	return result
}

// handleGetPublicTraderConfig 获取公开的交易员配置信息（无需认证，不包含敏感信息）
func (s *Server) handleGetPublicTraderConfig(c *gin.Context) {
	traderID := c.Param("id")
	if traderID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "交易员ID不能为空"})
		return
	}

	trader, err := s.traderManager.GetTrader(traderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "交易员不存在"})
		return
	}

	// 获取交易员的状态信息
	status := trader.GetStatus()

	// 只返回公开的配置信息，不包含API密钥等敏感数据
	result := map[string]interface{}{
		"trader_id":   trader.GetID(),
		"trader_name": trader.GetName(),
		"ai_model":    trader.GetAIModel(),
		"exchange":    trader.GetExchange(),
		"is_running":  status["is_running"],
		"ai_provider": status["ai_provider"],
		"start_time":  status["start_time"],
	}

	c.JSON(http.StatusOK, result)
}

// reloadPromptTemplatesWithLog 重新加载提示词模板并记录日志
func (s *Server) reloadPromptTemplatesWithLog(templateName string) {
	if err := decision.ReloadPromptTemplates(); err != nil {
		log.Printf("⚠️  重新加载提示词模板失败: %v", err)
		return
	}

	if templateName == "" {
		log.Printf("✓ 已重新加载系统提示词模板 [当前使用: default (未指定，使用默认)]")
	} else {
		log.Printf("✓ 已重新加载系统提示词模板 [当前使用: %s]", templateName)
	}
}

// handleCreatePromptTemplate 创建新的提示词模板
func (s *Server) handleCreatePromptTemplate(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 检查模板是否已存在
	if decision.TemplateExists(req.Name) {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("模板已存在: %s", req.Name)})
		return
	}

	// 保存模板
	if err := decision.SavePromptTemplate(req.Name, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("创建模板失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模板创建成功",
		"name":    req.Name,
	})
}

// handleUpdatePromptTemplate 更新提示词模板
func (s *Server) handleUpdatePromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// 检查模板是否存在
	if !decision.TemplateExists(templateName) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateName)})
		return
	}

	// 更新模板
	if err := decision.SavePromptTemplate(templateName, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新模板失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模板更新成功",
		"name":    templateName,
	})
}

// handleDeletePromptTemplate 删除提示词模板
func (s *Server) handleDeletePromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	// 删除模板
	if err := decision.DeletePromptTemplate(templateName); err != nil {
		if strings.Contains(err.Error(), "不能删除系统模板") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else if strings.Contains(err.Error(), "模板不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("删除模板失败: %v", err)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模板删除成功",
	})
}

// handleReloadPromptTemplates 重新加载所有提示词模板
func (s *Server) handleReloadPromptTemplates(c *gin.Context) {
	if err := decision.ReloadPromptTemplates(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("重新加载失败: %v", err),
		})
		return
	}

	templates := decision.GetAllPromptTemplates()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "重新加载成功",
		"count":   len(templates),
	})
}
