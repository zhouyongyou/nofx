package auth

import (
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// JWTSecret JWT密钥，将从配置中动态设置
var JWTSecret []byte

// tokenBlacklist 用于登出后的token黑名单（仅内存，按过期时间清理）
var tokenBlacklist = struct {
	sync.RWMutex
	items map[string]time.Time
}{items: make(map[string]time.Time)}

// refreshTokenBlacklist Refresh Token 黑名单
var refreshTokenBlacklist = struct {
	sync.RWMutex
	items map[string]time.Time
}{items: make(map[string]time.Time)}

// maxBlacklistEntries 黑名单最大容量阈值
const maxBlacklistEntries = 100_000

// OTPIssuer OTP发行者名称
const OTPIssuer = "nofxAI"

// SetJWTSecret 设置JWT密钥
func SetJWTSecret(secret string) {
	JWTSecret = []byte(secret)
}

// BlacklistToken 将token加入黑名单直到过期
func BlacklistToken(token string, exp time.Time) {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	tokenBlacklist.items[token] = exp

	// 如果超过容量阈值，则进行一次过期清理；若仍超限，记录警告日志
	if len(tokenBlacklist.items) > maxBlacklistEntries {
		now := time.Now()
		for t, e := range tokenBlacklist.items {
			if now.After(e) {
				delete(tokenBlacklist.items, t)
			}
		}
		if len(tokenBlacklist.items) > maxBlacklistEntries {
			log.Printf("auth: token blacklist size (%d) exceeds limit (%d) after sweep; consider reducing JWT TTL or using a shared persistent store",
				len(tokenBlacklist.items), maxBlacklistEntries)
		}
	}
}

// IsTokenBlacklisted 检查token是否在黑名单中（过期自动清理）
func IsTokenBlacklisted(token string) bool {
	tokenBlacklist.Lock()
	defer tokenBlacklist.Unlock()
	if exp, ok := tokenBlacklist.items[token]; ok {
		if time.Now().After(exp) {
			delete(tokenBlacklist.items, token)
			return false
		}
		return true
	}
	return false
}

// BlacklistRefreshToken 将 Refresh Token 加入黑名单
func BlacklistRefreshToken(token string, exp time.Time) {
	refreshTokenBlacklist.Lock()
	defer refreshTokenBlacklist.Unlock()
	refreshTokenBlacklist.items[token] = exp

	// 清理过期条目
	if len(refreshTokenBlacklist.items) > maxBlacklistEntries {
		now := time.Now()
		for t, e := range refreshTokenBlacklist.items {
			if now.After(e) {
				delete(refreshTokenBlacklist.items, t)
			}
		}
	}
}

// IsRefreshTokenBlacklisted 检查 Refresh Token 是否在黑名单中
func IsRefreshTokenBlacklisted(token string) bool {
	refreshTokenBlacklist.Lock()
	defer refreshTokenBlacklist.Unlock()
	if exp, ok := refreshTokenBlacklist.items[token]; ok {
		if time.Now().After(exp) {
			delete(refreshTokenBlacklist.items, token)
			return false
		}
		return true
	}
	return false
}

// Claims JWT声明（Access Token）
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// RefreshClaims Refresh Token 声明
type RefreshClaims struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	TokenType string `json:"token_type"` // 固定为 "refresh"
	jwt.RegisteredClaims
}

// TokenPair Access Token 和 Refresh Token 对
type TokenPair struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int64  `json:"expires_in"`         // Access Token 过期时间（秒）
	RefreshExpiresIn int64  `json:"refresh_expires_in"` // Refresh Token 过期时间（秒）
}

// HashPassword 哈希密码
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateOTPSecret 生成OTP密钥
func GenerateOTPSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      OTPIssuer,
		AccountName: uuid.New().String(),
	})
	if err != nil {
		return "", err
	}

	return key.Secret(), nil
}

// VerifyOTP 验证OTP码
func VerifyOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateJWT 生成JWT token（舊版本，保持向後兼容）
func GenerateJWT(userID, email string) (string, error) {
	// 安全检查：确保JWT密钥已设置
	if len(JWTSecret) == 0 {
		return "", fmt.Errorf("JWT密钥未设置，无法生成token")
	}

	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)), // 24小时过期
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "nofxAI",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

// GenerateTokenPair 生成 Access Token 和 Refresh Token 对
// Access Token: 15 分钟有效期（短期）
// Refresh Token: 7 天有效期（长期）
func GenerateTokenPair(userID, email string) (*TokenPair, error) {
	// 安全检查：确保JWT密钥已设置
	if len(JWTSecret) == 0 {
		return nil, fmt.Errorf("JWT密钥未设置，无法生成token")
	}

	now := time.Now()
	accessTokenExpiry := 15 * time.Minute
	refreshTokenExpiry := 7 * 24 * time.Hour

	// 生成 Access Token
	accessClaims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "nofxAI",
			ID:        uuid.New().String(), // 唯一标识符
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("生成 Access Token 失败: %w", err)
	}

	// 生成 Refresh Token
	refreshClaims := RefreshClaims{
		UserID:    userID,
		Email:     email,
		TokenType: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(refreshTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "nofxAI",
			ID:        uuid.New().String(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("生成 Refresh Token 失败: %w", err)
	}

	return &TokenPair{
		AccessToken:      accessTokenString,
		RefreshToken:     refreshTokenString,
		ExpiresIn:        int64(accessTokenExpiry.Seconds()),
		RefreshExpiresIn: int64(refreshTokenExpiry.Seconds()),
	}, nil
}

// ValidateRefreshToken 验证 Refresh Token
func ValidateRefreshToken(tokenString string) (*RefreshClaims, error) {
	// 检查是否在黑名单中
	if IsRefreshTokenBlacklisted(tokenString) {
		return nil, fmt.Errorf("Refresh Token 已被撤销")
	}

	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("解析 Refresh Token 失败: %w", err)
	}

	if claims, ok := token.Claims.(*RefreshClaims); ok && token.Valid {
		// 验证 token_type
		if claims.TokenType != "refresh" {
			return nil, fmt.Errorf("无效的 Token 类型")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("无效的 Refresh Token")
}

// RefreshAccessToken 使用 Refresh Token 刷新 Access Token
// 返回新的 Access Token 和新的 Refresh Token（可选：实现 Refresh Token 轮换）
func RefreshAccessToken(refreshTokenString string) (*TokenPair, error) {
	// 验证 Refresh Token
	claims, err := ValidateRefreshToken(refreshTokenString)
	if err != nil {
		return nil, err
	}

	// 生成新的 Token 对
	tokenPair, err := GenerateTokenPair(claims.UserID, claims.Email)
	if err != nil {
		return nil, err
	}

	// 可选：将旧的 Refresh Token 加入黑名单（实现 Refresh Token 轮换）
	// 这样可以防止 Refresh Token 被盗用后持续使用
	if claims.ExpiresAt != nil {
		BlacklistRefreshToken(refreshTokenString, claims.ExpiresAt.Time)
		log.Printf("✅ [AUTH] 旧 Refresh Token 已撤销，用户: %s", claims.Email)
	}

	return tokenPair, nil
}

// ValidateJWT 验证JWT token
func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("无效的token")
}

// GetOTPQRCodeURL 获取OTP二维码URL
func GetOTPQRCodeURL(secret, email string) string {
	return fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", OTPIssuer, email, secret, OTPIssuer)
}
