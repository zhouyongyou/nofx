package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CSRFConfig CSRF 中间件配置
type CSRFConfig struct {
	TokenLength    int           // Token 长度（字节）
	CookieName     string        // Cookie 名称
	HeaderName     string        // Header 名称
	CookiePath     string        // Cookie 路径
	CookieSecure   bool          // 是否仅 HTTPS
	CookieSameSite http.SameSite // SameSite 属性
	CookieDomain   string        // Cookie 域名（用于跨子域共享）
	ExemptPaths    []string      // 豁免路径（不检查 CSRF）
}

// DefaultCSRFConfig 返回默认 CSRF 配置
// 注意：此配置可通过环境变量进行调整
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		TokenLength:    32,
		CookieName:     "csrf_token",
		HeaderName:     "X-CSRF-Token",
		CookiePath:     "/",
		CookieSecure:   false, // 开发环境设为 false，生产环境应为 true
		CookieSameSite: http.SameSiteLaxMode, // 改为 Lax 以支持反向代理环境
		CookieDomain:   "",                   // 空字符串表示当前域名
		ExemptPaths: []string{
			"/api/health",
			"/api/supported-models",
			"/api/supported-exchanges",
			"/api/config",
			"/api/crypto/public-key",
			"/api/prompt-templates",
			"/api/traders",               // 公开的 Trader 列表
			"/api/competition",           // 公开的竞赛数据
			"/api/top-traders",           // 公开的 Top Traders
			"/api/equity-history",        // 公开的权益历史
			"/api/login",                 // 登录端点豁免（首次访问）
			"/api/register",              // 注册端点豁免
			"/api/verify-otp",            // OTP验证端点豁免（已有OTP安全验证）
			"/api/complete-registration", // 完成注册端点豁免（已有OTP安全验证）
			"/api/models",                // 模型配置端点（已有JWT认证+RSA加密）
			"/api/exchanges",             // 交易所配置端点（已有JWT认证+RSA加密）
		},
	}
}

// generateCSRFToken 生成随机 CSRF Token
func generateCSRFToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// CSRFMiddleware CSRF 保护中间件（Double Submit Cookie 模式）
// 工作原理：
// 1. 第一次请求时生成随机 Token，存储在 Cookie 中
// 2. 前端从 Cookie 读取 Token，并在后续请求的 Header 中携带
// 3. 服务器验证 Cookie 中的 Token 和 Header 中的 Token 是否一致
// 4. 由于恶意网站无法读取其他域的 Cookie，因此无法伪造请求
func CSRFMiddleware(config CSRFConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// OPTIONS 请求直接放行（CORS 预检）
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// 检查是否在豁免路径中
		path := c.Request.URL.Path
		for _, exemptPath := range config.ExemptPaths {
			if strings.HasPrefix(path, exemptPath) {
				c.Next()
				return
			}
		}

		// GET 和 HEAD 请求不检查 CSRF（幂等操作）
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
			// 如果 Cookie 中没有 Token，生成一个新的
			_, err := c.Cookie(config.CookieName)
			if err != nil {
				token, genErr := generateCSRFToken(config.TokenLength)
				if genErr != nil {
					log.Printf("❌ [CSRF] 生成 Token 失败: %v", genErr)
					c.Next()
					return
				}

				c.SetSameSite(config.CookieSameSite)
				c.SetCookie(
					config.CookieName,
					token,
					3600*24, // 24 小时
					config.CookiePath,
					"",
					config.CookieSecure,
					true, // HttpOnly
				)
				log.Printf("🔐 [CSRF] 为 IP %s 生成新 Token", c.ClientIP())
			}
			c.Next()
			return
		}

		// POST/PUT/DELETE 等状态变更操作需要验证 CSRF Token
		cookieToken, err := c.Cookie(config.CookieName)
		if err != nil {
			log.Printf("🚨 [CSRF] IP %s 缺少 CSRF Cookie (路径: %s)", c.ClientIP(), path)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "CSRF token missing in cookie",
			})
			c.Abort()
			return
		}

		// 从 Header 中获取 Token
		headerToken := c.GetHeader(config.HeaderName)
		if headerToken == "" {
			log.Printf("🚨 [CSRF] IP %s 缺少 CSRF Header (路径: %s)", c.ClientIP(), path)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "CSRF token missing in header",
			})
			c.Abort()
			return
		}

		// 验证 Token 是否一致
		if cookieToken != headerToken {
			log.Printf("🚨 [CSRF] IP %s Token 不匹配 (路径: %s)", c.ClientIP(), path)
			c.JSON(http.StatusForbidden, gin.H{
				"error": "CSRF token mismatch",
			})
			c.Abort()
			return
		}

		// 验证通过
		log.Printf("✅ [CSRF] IP %s 验证通过 (路径: %s)", c.ClientIP(), path)
		c.Next()
	}
}

// GetCSRFToken 获取当前请求的 CSRF Token（用于 API 响应）
func GetCSRFToken(c *gin.Context, config CSRFConfig) string {
	token, err := c.Cookie(config.CookieName)
	if err != nil {
		// 如果没有 Token，生成一个新的
		newToken, genErr := generateCSRFToken(config.TokenLength)
		if genErr != nil {
			log.Printf("❌ [CSRF] 生成 Token 失败: %v", genErr)
			return ""
		}

		c.SetSameSite(config.CookieSameSite)
		c.SetCookie(
			config.CookieName,
			newToken,
			3600*24,
			config.CookiePath,
			"",
			config.CookieSecure,
			true,
		)
		return newToken
	}
	return token
}
