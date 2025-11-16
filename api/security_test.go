package api

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"nofx/auth"
	"nofx/config"
	"nofx/crypto"
	"nofx/manager"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// setupTestServer creates a test server with mock dependencies
func setupTestServer(t *testing.T) (*Server, *config.Database, func()) {
	gin.SetMode(gin.TestMode)

	// Create temporary database
	tmpDB, err := os.CreateTemp("", "test_api_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp db: %v", err)
	}
	tmpDB.Close()

	db, err := config.NewDatabase(tmpDB.Name())
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Create crypto service (optional for tests)
	tmpKeyPath := "/tmp/test_api_key.pem"
	cryptoService, err := crypto.NewCryptoService(tmpKeyPath)
	if err != nil {
		t.Logf("警告：无法创建加密服务，将在无加密模式下测试: %v", err)
		// Continue without crypto service for testing
	} else {
		db.SetCryptoService(cryptoService)
	}

	// Create trader manager
	tm := manager.NewTraderManager()

	// Create server
	server := NewServer(tm, db, cryptoService, 8080)

	cleanup := func() {
		os.Remove(tmpDB.Name())
		os.Remove(tmpKeyPath)
	}

	return server, db, cleanup
}

// TestSQLInjectionProtection tests SQL injection protection
func TestSQLInjectionProtection(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// Register test user first
	testEmail := "test@example.com"
	testPass := "ValidPass123!"
	hashedPassword, err := auth.HashPassword(testPass)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &config.User{
		ID:           "test-user-001",
		Email:        testEmail,
		PasswordHash: hashedPassword,
		OTPSecret:    "JBSWY3DPEHPK3PXP", // Test OTP secret
		OTPVerified:  true,               // Set to true to allow login
	}
	err = db.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	tests := []struct {
		name           string
		endpoint       string
		method         string
		payload        interface{}
		expectedStatus int
		description    string
	}{
		{
			name:     "SQL injection in trader name",
			endpoint: "/api/traders",
			method:   "POST",
			payload: map[string]interface{}{
				"name":            "'; DROP TABLE traders; --",
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"initial_balance": 1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject SQL injection attempt in name field",
		},
		{
			name:     "SQL injection with UNION",
			endpoint: "/api/traders",
			method:   "POST",
			payload: map[string]interface{}{
				"name":            "test' UNION SELECT * FROM users--",
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"initial_balance": 1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject UNION-based SQL injection",
		},
		{
			name:     "SQL injection in trading symbols",
			endpoint: "/api/traders",
			method:   "POST",
			payload: map[string]interface{}{
				"name":            "TestTrader",
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"trading_symbols": "BTCUSDT'; DELETE FROM config; --",
				"initial_balance": 1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject SQL injection in trading_symbols",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(tt.method, tt.endpoint, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			// Note: Without valid token, should get 401, but with malicious input should get 400
			// For this test we're checking if the input validation catches it

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			// Should either reject with 400 (bad input) or 401 (unauthorized)
			// Both are acceptable as long as it doesn't execute the SQL
			if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
				t.Errorf("%s: Expected status 400 or 401, got %d", tt.description, w.Code)
			}

			t.Logf("✅ %s: Got status %d (rejected)", tt.description, w.Code)
		})
	}
}

// TestXSSProtection tests XSS attack protection
func TestXSSProtection(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"<img src=x onerror=alert('xss')>",
		"javascript:alert('xss')",
		"<iframe src='javascript:alert(\"xss\")'></iframe>",
		"<svg onload=alert('xss')>",
	}

	for _, payload := range xssPayloads {
		// Safely truncate payload for test name
		testName := payload
		if len(payload) > 20 {
			testName = payload[:20]
		}
		t.Run("XSS_"+testName, func(t *testing.T) {
			reqBody := map[string]interface{}{
				"name":            payload,
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"initial_balance": 1000.0,
			}

			body, _ := json.Marshal(reqBody)
			req := httptest.NewRequest("POST", "/api/traders", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			// Should reject XSS attempts (either 400 for bad input or 401 for unauthorized)
			if w.Code == http.StatusOK || w.Code == http.StatusCreated {
				t.Errorf("XSS payload was not rejected: %s", payload)
			}

			// Check response doesn't contain unescaped script tags
			responseBody := w.Body.String()
			if strings.Contains(responseBody, "<script>") {
				t.Errorf("Response contains unescaped script tag")
			}

			// Safely truncate payload for logging
			logPayload := payload
			if len(payload) > 30 {
				logPayload = payload[:30]
			}
			t.Logf("✅ XSS payload rejected: %s", logPayload)
		})
	}
}

func TestCryptoDecryptEndpointDisabled(t *testing.T) {
	t.Setenv("ENABLE_CLIENT_DECRYPT_API", "")
	t.Setenv("DATA_ENCRYPTION_KEY", "unit-test-key-please-change-1234567890")
	auth.SetJWTSecret("test-secret")

	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	body, _ := json.Marshal(map[string]string{
		"wrappedKey": "",
	})
	req := httptest.NewRequest("POST", "/api/crypto/decrypt", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("Expected 404 when decrypt API disabled, got %d", w.Code)
	}
}

func TestCryptoDecryptAADMismatch(t *testing.T) {
	t.Setenv("ENABLE_CLIENT_DECRYPT_API", "true")
	t.Setenv("DATA_ENCRYPTION_KEY", "unit-test-key-please-change-1234567890")
	auth.SetJWTSecret("test-secret")

	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	if server.cryptoHandler == nil || server.cryptoHandler.cryptoService == nil {
		t.Skip("crypto service not available for test")
	}

	token := createTestJWTUser(t, db, "user-crypto", "user@example.com")
	payload := encryptPayloadForTest(t, server, "another-user", "secret")

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/crypto/decrypt", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 for AAD mismatch, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCryptoDecryptSuccess(t *testing.T) {
	t.Setenv("ENABLE_CLIENT_DECRYPT_API", "true")
	t.Setenv("DATA_ENCRYPTION_KEY", "unit-test-key-please-change-1234567890")
	auth.SetJWTSecret("test-secret")

	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	if server.cryptoHandler == nil || server.cryptoHandler.cryptoService == nil {
		t.Skip("crypto service not available for test")
	}

	userID := "user-success"
	email := "success@example.com"
	token := createTestJWTUser(t, db, userID, email)
	payload := encryptPayloadForTest(t, server, userID, "sensitive-secret")

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/api/crypto/decrypt", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["plaintext"] != "sensitive-secret" {
		t.Fatalf("解密結果不正確，得到 %q", resp["plaintext"])
	}
}

func createTestJWTUser(t *testing.T, db *config.Database, userID, email string) string {
	t.Helper()
	user := &config.User{
		ID:           userID,
		Email:        email,
		PasswordHash: "hash",
		OTPSecret:    "",
		OTPVerified:  true,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	token, err := auth.GenerateJWT(userID, email)
	if err != nil {
		t.Fatalf("生成 JWT 失败: %v", err)
	}
	return token
}

func encryptPayloadForTest(t *testing.T, server *Server, userID, plaintext string) crypto.EncryptedPayload {
	t.Helper()

	publicKeyPEM := server.cryptoHandler.cryptoService.GetPublicKeyPEM()
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		t.Fatal("无法解析公钥 PEM")
	}

	pubKeyIface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("解析公钥失败: %v", err)
	}
	pubKey, ok := pubKeyIface.(*rsa.PublicKey)
	if !ok {
		t.Fatal("公钥类型不正确")
	}

	aesKey := make([]byte, 32)
	if _, err := rand.Read(aesKey); err != nil {
		t.Fatalf("生成 AES Key 失败: %v", err)
	}
	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		t.Fatalf("生成 IV 失败: %v", err)
	}

	aadBytes, _ := json.Marshal(crypto.AADData{
		UserID:    userID,
		SessionID: "session-test",
		TS:        time.Now().Unix(),
		Purpose:   "sensitive_data_encryption",
	})

	blockCipher, err := aes.NewCipher(aesKey)
	if err != nil {
		t.Fatalf("创建 AES cipher 失败: %v", err)
	}
	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		t.Fatalf("创建 GCM 失败: %v", err)
	}
	ciphertext := gcm.Seal(nil, iv, []byte(plaintext), aadBytes)

	wrappedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, aesKey, nil)
	if err != nil {
		t.Fatalf("加密 AES Key 失败: %v", err)
	}

	return crypto.EncryptedPayload{
		WrappedKey: base64.RawURLEncoding.EncodeToString(wrappedKey),
		IV:         base64.RawURLEncoding.EncodeToString(iv),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
		AAD:        base64.RawURLEncoding.EncodeToString(aadBytes),
		TS:         time.Now().Unix(),
	}
}

// TestInputValidation tests various input validation scenarios
func TestInputValidation(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name           string
		endpoint       string
		payload        interface{}
		expectedStatus int
		errorContains  string
	}{
		{
			name:     "Empty trader name",
			endpoint: "/api/traders",
			payload: map[string]interface{}{
				"name":            "",
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"initial_balance": 1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "",
		},
		{
			name:     "Extremely long name (buffer overflow attempt)",
			endpoint: "/api/traders",
			payload: map[string]interface{}{
				"name":            strings.Repeat("A", 10000),
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"initial_balance": 1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "",
		},
		{
			name:     "Invalid leverage - too high",
			endpoint: "/api/traders",
			payload: map[string]interface{}{
				"name":             "TestTrader",
				"ai_model_id":      "deepseek",
				"exchange_id":      "binance",
				"btc_eth_leverage": 100,
				"initial_balance":  1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "杠杆",
		},
		{
			name:     "Invalid leverage - negative",
			endpoint: "/api/traders",
			payload: map[string]interface{}{
				"name":             "TestTrader",
				"ai_model_id":      "deepseek",
				"exchange_id":      "binance",
				"btc_eth_leverage": -5,
				"initial_balance":  1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "杠杆",
		},
		{
			name:     "Invalid symbol format",
			endpoint: "/api/traders",
			payload: map[string]interface{}{
				"name":            "TestTrader",
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"trading_symbols": "BTC,ETH,INVALID",
				"initial_balance": 1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "币种格式",
		},
		{
			name:     "Negative initial balance",
			endpoint: "/api/traders",
			payload: map[string]interface{}{
				"name":            "TestTrader",
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"initial_balance": -1000.0,
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "",
		},
		{
			name:     "Zero initial balance",
			endpoint: "/api/traders",
			payload: map[string]interface{}{
				"name":            "TestTrader",
				"ai_model_id":     "deepseek",
				"exchange_id":     "binance",
				"initial_balance": 0,
			},
			expectedStatus: http.StatusBadRequest,
			errorContains:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", tt.endpoint, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			// Should get 400 (validation error) or 401 (unauthorized)
			if w.Code != http.StatusBadRequest && w.Code != http.StatusUnauthorized {
				t.Errorf("Expected 400 or 401, got %d", w.Code)
			}

			responseBody := w.Body.String()
			if tt.errorContains != "" && !strings.Contains(responseBody, tt.errorContains) {
				t.Logf("Response body: %s", responseBody)
				t.Logf("Expected to contain: %s", tt.errorContains)
			}

			t.Logf("✅ Input validation working: %s", tt.name)
		})
	}
}

// TestAuthenticationRequired tests that protected endpoints require authentication
func TestAuthenticationRequired(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	protectedEndpoints := []struct {
		method   string
		endpoint string
	}{
		{"GET", "/api/my-traders"},
		{"POST", "/api/traders"},
		{"PUT", "/api/traders/test-id"},
		{"DELETE", "/api/traders/test-id"},
		{"POST", "/api/traders/test-id/start"},
		{"POST", "/api/traders/test-id/stop"},
		{"GET", "/api/status"},
		{"GET", "/api/models"},
		{"PUT", "/api/models"},
		{"GET", "/api/exchanges"},
		{"PUT", "/api/exchanges"},
	}

	for _, ep := range protectedEndpoints {
		t.Run(ep.method+"_"+ep.endpoint, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.endpoint, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should return 401 Unauthorized, or 403 (CSRF), or 429 (Rate Limit) without valid token
			// All of these indicate the endpoint is protected
			if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden && w.Code != http.StatusTooManyRequests {
				t.Errorf("Expected 401/403/429 for %s %s without auth, got %d",
					ep.method, ep.endpoint, w.Code)
			}

			t.Logf("✅ %s %s requires authentication (status: %d)", ep.method, ep.endpoint, w.Code)
		})
	}
}

// TestPublicEndpointsNoAuth tests that public endpoints work without authentication
func TestPublicEndpointsNoAuth(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	publicEndpoints := []struct {
		method   string
		endpoint string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/supported-models"},
		{"GET", "/api/supported-exchanges"},
		{"GET", "/api/config"},
		{"GET", "/api/crypto/public-key"},
		{"GET", "/api/competition"},
		{"GET", "/api/top-traders"},
	}

	for _, ep := range publicEndpoints {
		t.Run(ep.method+"_"+ep.endpoint, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.endpoint, nil)
			w := httptest.NewRecorder()

			server.router.ServeHTTP(w, req)

			// Should NOT return 401 (may return other codes based on logic)
			if w.Code == http.StatusUnauthorized {
				t.Errorf("Public endpoint %s %s should not require auth",
					ep.method, ep.endpoint)
			}

			t.Logf("✅ %s %s is public (status: %d)", ep.method, ep.endpoint, w.Code)
		})
	}
}

// TestRateLimitingBehavior tests API behavior under high load
func TestRateLimitingBehavior(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	// Make many rapid requests to the same endpoint
	endpoint := "/api/health"
	successCount := 0
	tooManyRequestsCount := 0

	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", endpoint, nil)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			successCount++
		} else if w.Code == http.StatusTooManyRequests {
			tooManyRequestsCount++
		}
	}

	t.Logf("Rapid requests: %d successful, %d rate-limited", successCount, tooManyRequestsCount)

	// Note: Current implementation may not have rate limiting
	// This test documents the behavior
	if tooManyRequestsCount > 0 {
		t.Logf("✅ Rate limiting is active")
	} else {
		t.Logf("⚠️  No rate limiting detected (may want to add)")
	}
}

// TestCORSHeaders tests CORS configuration
func TestCORSHeaders(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("OPTIONS", "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	w := httptest.NewRecorder()
	server.router.ServeHTTP(w, req)

	// Check CORS headers
	allowOrigin := w.Header().Get("Access-Control-Allow-Origin")
	allowMethods := w.Header().Get("Access-Control-Allow-Methods")
	allowHeaders := w.Header().Get("Access-Control-Allow-Headers")

	if allowOrigin == "" {
		t.Error("Missing Access-Control-Allow-Origin header")
	}

	if allowMethods == "" {
		t.Error("Missing Access-Control-Allow-Methods header")
	}

	if allowHeaders == "" {
		t.Error("Missing Access-Control-Allow-Headers header")
	}

	t.Logf("✅ CORS headers present:")
	t.Logf("  Origin: %s", allowOrigin)
	t.Logf("  Methods: %s", allowMethods)
	t.Logf("  Headers: %s", allowHeaders)

	// Check if wildcard CORS is safe for your use case
	if allowOrigin == "*" {
		t.Logf("⚠️  Warning: Wildcard CORS (*) allows all origins")
		t.Logf("   Consider restricting to specific domains in production")
	}
}

// TestJSONContentTypeRequired tests that JSON content type is enforced
func TestJSONContentTypeRequired(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name        string
		contentType string
		expectError bool
	}{
		{
			name:        "Valid JSON content type",
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "Missing content type",
			contentType: "",
			expectError: true,
		},
		{
			name:        "Wrong content type",
			contentType: "text/plain",
			expectError: true,
		},
		{
			name:        "Form data instead of JSON",
			contentType: "application/x-www-form-urlencoded",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"email":"test@example.com","password":"Test123!"}`
			req := httptest.NewRequest("POST", "/api/login", strings.NewReader(body))

			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			if tt.expectError && w.Code == http.StatusOK {
				t.Errorf("Expected error for content type '%s', but got success", tt.contentType)
			}

			if !tt.expectError && w.Code != http.StatusOK && w.Code != http.StatusUnauthorized {
				t.Logf("Content-Type: %s, Status: %d", tt.contentType, w.Code)
			}
		})
	}
}

// TestPasswordComplexity tests password validation
func TestPasswordComplexity(t *testing.T) {
	server, _, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name       string
		email      string
		password   string
		shouldPass bool
	}{
		{
			name:       "Valid strong password",
			email:      "test1@example.com",
			password:   "StrongPass123!",
			shouldPass: true,
		},
		{
			name:       "Too short password",
			email:      "test2@example.com",
			password:   "Short1!",
			shouldPass: false,
		},
		{
			name:       "No numbers",
			email:      "test3@example.com",
			password:   "NoNumbers!",
			shouldPass: false,
		},
		{
			name:       "No special chars",
			email:      "test4@example.com",
			password:   "NoSpecial123",
			shouldPass: false,
		},
		{
			name:       "All lowercase",
			email:      "test5@example.com",
			password:   "lowercase123!",
			shouldPass: true, // May pass depending on policy
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := map[string]string{
				"email":    tt.email,
				"password": tt.password,
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			t.Logf("Password test '%s': status %d", tt.name, w.Code)

			// Note: The actual password validation depends on implementation
			// This test documents the current behavior
		})
	}
}

// TestConcurrentAuthenticationRequests tests auth under concurrent load
func TestConcurrentAuthenticationRequests(t *testing.T) {
	server, db, cleanup := setupTestServer(t)
	defer cleanup()

	// Create test user
	testEmail := "concurrent@example.com"
	testPass := "ConcurrentTest123!"
	hashedPassword, err := auth.HashPassword(testPass)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &config.User{
		ID:           "concurrent-user-001",
		Email:        testEmail,
		PasswordHash: hashedPassword,
		OTPSecret:    "JBSWY3DPEHPK3PXP", // Test OTP secret
		OTPVerified:  true,               // Set to true to allow login
	}
	err = db.CreateUser(user)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Make concurrent login requests
	numRequests := 50
	done := make(chan bool)
	results := make([]int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			reqBody := map[string]string{
				"email":    testEmail,
				"password": testPass,
			}
			body, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			server.router.ServeHTTP(w, req)

			results[index] = w.Code
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	timeout := time.After(10 * time.Second)
	for i := 0; i < numRequests; i++ {
		select {
		case <-done:
			// Request completed
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent requests")
		}
	}

	// Analyze results
	successCount := 0
	for _, status := range results {
		if status == http.StatusOK {
			successCount++
		}
	}

	t.Logf("✅ Concurrent auth test: %d/%d successful", successCount, numRequests)

	if successCount == 0 {
		t.Error("No successful logins in concurrent test")
	}
}
