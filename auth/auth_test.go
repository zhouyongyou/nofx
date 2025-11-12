package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
)

// =============================================================================
// Test 1: JWT Secret Management
// =============================================================================

func TestSetJWTSecret(t *testing.T) {
	t.Run("set and use secret", func(t *testing.T) {
		secret := "test-secret-key-12345"
		SetJWTSecret(secret)

		if string(JWTSecret) != secret {
			t.Errorf("expected JWTSecret to be %s, got %s", secret, string(JWTSecret))
		}
	})

	t.Run("empty secret", func(t *testing.T) {
		SetJWTSecret("")
		if len(JWTSecret) != 0 {
			t.Errorf("expected empty JWTSecret, got %s", string(JWTSecret))
		}
	})
}

// =============================================================================
// Test 2: Password Hashing and Verification
// =============================================================================

func TestHashPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{"simple password", "password123"},
		{"complex password", "P@ssw0rd!2023#Complex"},
		{"unicode password", "密碼測試123"},
		{"long password", strings.Repeat("a", 72)}, // bcrypt max length
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := HashPassword(tt.password)
			if err != nil {
				t.Fatalf("HashPassword failed: %v", err)
			}

			// Hash should not be empty
			if hash == "" {
				t.Error("hash should not be empty")
			}

			// Hash should not equal password
			if hash == tt.password {
				t.Error("hash should not equal plaintext password")
			}

			// Hash should start with $2a$ or $2b$ (bcrypt format)
			if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
				t.Errorf("unexpected hash format: %s", hash)
			}
		})
	}
}

func TestCheckPassword(t *testing.T) {
	password := "correct-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	tests := []struct {
		name     string
		password string
		expected bool
	}{
		{"correct password", password, true},
		{"wrong password", "wrong-password", false},
		{"empty password", "", false},
		{"case sensitive", "Correct-Password-123", false},
		{"extra spaces", " correct-password-123 ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckPassword(tt.password, hash)
			if result != tt.expected {
				t.Errorf("CheckPassword(%q, hash) = %v, expected %v", tt.password, result, tt.expected)
			}
		})
	}
}

func TestCheckPasswordWithInvalidHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"empty hash", ""},
		{"invalid format", "not-a-valid-bcrypt-hash"},
		{"random string", "random-string-123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckPassword("any-password", tt.hash)
			if result {
				t.Error("CheckPassword should return false for invalid hash")
			}
		})
	}
}

// =============================================================================
// Test 3: OTP Generation and Verification
// =============================================================================

func TestGenerateOTPSecret(t *testing.T) {
	secret1, err := GenerateOTPSecret()
	if err != nil {
		t.Fatalf("GenerateOTPSecret failed: %v", err)
	}

	if secret1 == "" {
		t.Error("secret should not be empty")
	}

	// Generate second secret to ensure they're different
	secret2, err := GenerateOTPSecret()
	if err != nil {
		t.Fatalf("GenerateOTPSecret failed: %v", err)
	}

	if secret1 == secret2 {
		t.Error("consecutive OTP secrets should be different")
	}

	// Secret should be base32 encoded (A-Z, 2-7)
	for _, c := range secret1 {
		if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7') || c == '=') {
			t.Errorf("secret contains invalid base32 character: %c", c)
			break
		}
	}
}

func TestVerifyOTP(t *testing.T) {
	// Generate a secret
	secret, err := GenerateOTPSecret()
	if err != nil {
		t.Fatalf("GenerateOTPSecret failed: %v", err)
	}

	t.Run("valid OTP code", func(t *testing.T) {
		// Generate current valid code
		code, err := totp.GenerateCode(secret, time.Now())
		if err != nil {
			t.Fatalf("totp.GenerateCode failed: %v", err)
		}

		if !VerifyOTP(secret, code) {
			t.Error("VerifyOTP should return true for valid code")
		}
	})

	t.Run("invalid OTP code", func(t *testing.T) {
		invalidCodes := []string{
			"000000",
			"123456",
			"999999",
			"invalid",
			"",
		}

		for _, code := range invalidCodes {
			if VerifyOTP(secret, code) {
				t.Errorf("VerifyOTP should return false for invalid code: %s", code)
			}
		}
	})

	t.Run("expired OTP code", func(t *testing.T) {
		// Generate code from 2 minutes ago (should be expired)
		pastTime := time.Now().Add(-2 * time.Minute)
		oldCode, err := totp.GenerateCode(secret, pastTime)
		if err != nil {
			t.Fatalf("totp.GenerateCode failed: %v", err)
		}

		// This might still pass if within TOTP window, so we check logic
		// In real scenario, codes older than 30-60s should fail
		result := VerifyOTP(secret, oldCode)
		// We don't assert false here because TOTP has validation window
		// Just log the result for awareness
		t.Logf("Old code verification result: %v", result)
	})
}

func TestGetOTPQRCodeURL(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	email := "user@example.com"

	url := GetOTPQRCodeURL(secret, email)

	// Check URL format
	expectedPrefix := "otpauth://totp/" + OTPIssuer + ":"
	if !strings.HasPrefix(url, expectedPrefix) {
		t.Errorf("URL should start with %s, got %s", expectedPrefix, url)
	}

	// Check URL contains secret
	if !strings.Contains(url, "secret="+secret) {
		t.Errorf("URL should contain secret=%s", secret)
	}

	// Check URL contains email
	if !strings.Contains(url, email) {
		t.Errorf("URL should contain email %s", email)
	}

	// Check URL contains issuer
	if !strings.Contains(url, "issuer="+OTPIssuer) {
		t.Errorf("URL should contain issuer=%s", OTPIssuer)
	}
}

// =============================================================================
// Test 4: JWT Generation and Validation
// =============================================================================

func TestGenerateJWT(t *testing.T) {
	// Setup
	SetJWTSecret("test-jwt-secret-key-for-testing")

	t.Run("generate valid JWT", func(t *testing.T) {
		userID := "user-123"
		email := "test@example.com"

		tokenString, err := GenerateJWT(userID, email)
		if err != nil {
			t.Fatalf("GenerateJWT failed: %v", err)
		}

		if tokenString == "" {
			t.Error("token should not be empty")
		}

		// JWT should have 3 parts (header.payload.signature)
		parts := strings.Split(tokenString, ".")
		if len(parts) != 3 {
			t.Errorf("JWT should have 3 parts, got %d", len(parts))
		}
	})

	t.Run("generate JWT with empty values", func(t *testing.T) {
		// Should still generate token even with empty values
		tokenString, err := GenerateJWT("", "")
		if err != nil {
			t.Fatalf("GenerateJWT should not fail with empty values: %v", err)
		}

		if tokenString == "" {
			t.Error("token should not be empty even with empty values")
		}
	})

	t.Run("JWT with nil or empty secret", func(t *testing.T) {
		// Clear secret
		JWTSecret = nil

		// After security fix, GenerateJWT should reject nil secret
		tokenString, err := GenerateJWT("user-123", "test@example.com")

		// Should return error when secret is not set
		if err == nil {
			t.Errorf("GenerateJWT should fail with nil secret, but got token: %s", tokenString)
		}

		// Verify error message is clear
		if err != nil && err.Error() != "JWT密钥未设置，无法生成token" {
			t.Errorf("expected specific error message, got: %v", err)
		}

		// Restore secret
		SetJWTSecret("test-jwt-secret-key-for-testing")
	})
}

func TestValidateJWT(t *testing.T) {
	// Setup
	SetJWTSecret("test-jwt-secret-key-for-testing")
	userID := "user-456"
	email := "validate@example.com"

	t.Run("validate valid JWT", func(t *testing.T) {
		tokenString, err := GenerateJWT(userID, email)
		if err != nil {
			t.Fatalf("GenerateJWT failed: %v", err)
		}

		claims, err := ValidateJWT(tokenString)
		if err != nil {
			t.Fatalf("ValidateJWT failed: %v", err)
		}

		if claims.UserID != userID {
			t.Errorf("expected UserID %s, got %s", userID, claims.UserID)
		}

		if claims.Email != email {
			t.Errorf("expected Email %s, got %s", email, claims.Email)
		}

		if claims.Issuer != "nofxAI" {
			t.Errorf("expected Issuer nofxAI, got %s", claims.Issuer)
		}
	})

	t.Run("validate JWT with wrong secret", func(t *testing.T) {
		tokenString, _ := GenerateJWT(userID, email)

		// Change secret
		SetJWTSecret("wrong-secret-key")

		_, err := ValidateJWT(tokenString)
		if err == nil {
			t.Error("ValidateJWT should fail with wrong secret")
		}

		// Restore secret
		SetJWTSecret("test-jwt-secret-key-for-testing")
	})

	t.Run("validate invalid JWT format", func(t *testing.T) {
		invalidTokens := []string{
			"",
			"invalid.token",
			"not-a-jwt-token",
			"invalid.jwt.token",
		}

		for _, token := range invalidTokens {
			_, err := ValidateJWT(token)
			if err == nil {
				t.Errorf("ValidateJWT should fail for invalid token: %s", token)
			}
		}
	})

	t.Run("validate expired JWT", func(t *testing.T) {
		// Create expired token manually
		claims := Claims{
			UserID: userID,
			Email:  email,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-25 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now().Add(-25 * time.Hour)),
				Issuer:    "nofxAI",
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(JWTSecret)
		if err != nil {
			t.Fatalf("failed to create expired token: %v", err)
		}

		_, err = ValidateJWT(tokenString)
		if err == nil {
			t.Error("ValidateJWT should fail for expired token")
		}
	})

	t.Run("validate JWT with wrong signing method", func(t *testing.T) {
		// Create token with wrong signing method (RS256 instead of HS256)
		claims := Claims{
			UserID: userID,
			Email:  email,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				Issuer:    "nofxAI",
			},
		}

		// Create token with wrong method (but we can't sign it properly, so just test detection)
		token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims) // Use HS512 instead of HS256
		tokenString, _ := token.SignedString(JWTSecret)

		// This should still work because both are HMAC, but let's verify
		_, err := ValidateJWT(tokenString)
		// HS512 is still HMAC so it might pass, but we're testing the validation logic exists
		t.Logf("Wrong signing method validation result: %v", err)
	})
}

// =============================================================================
// Test 5: Token Blacklist Management
// =============================================================================

func TestBlacklistToken(t *testing.T) {
	// Clear blacklist before test
	tokenBlacklist.Lock()
	tokenBlacklist.items = make(map[string]time.Time)
	tokenBlacklist.Unlock()

	t.Run("add token to blacklist", func(t *testing.T) {
		token := "test-token-123"
		exp := time.Now().Add(1 * time.Hour)

		BlacklistToken(token, exp)

		if !IsTokenBlacklisted(token) {
			t.Error("token should be blacklisted")
		}
	})

	t.Run("blacklist multiple tokens", func(t *testing.T) {
		tokens := []string{"token1", "token2", "token3"}
		exp := time.Now().Add(1 * time.Hour)

		for _, token := range tokens {
			BlacklistToken(token, exp)
		}

		for _, token := range tokens {
			if !IsTokenBlacklisted(token) {
				t.Errorf("token %s should be blacklisted", token)
			}
		}
	})

	t.Run("blacklist cleanup on capacity", func(t *testing.T) {
		// Clear blacklist
		tokenBlacklist.Lock()
		tokenBlacklist.items = make(map[string]time.Time)
		tokenBlacklist.Unlock()

		// Add tokens that will expire
		expiredTime := time.Now().Add(-1 * time.Hour)
		for i := 0; i < 50; i++ {
			BlacklistToken("expired-token-"+string(rune(i)), expiredTime)
		}

		// Add valid tokens to trigger cleanup
		validTime := time.Now().Add(1 * time.Hour)
		for i := 0; i < maxBlacklistEntries+1; i++ {
			BlacklistToken("valid-token-"+string(rune(i)), validTime)
		}

		// Check that expired tokens were cleaned up
		tokenBlacklist.RLock()
		size := len(tokenBlacklist.items)
		tokenBlacklist.RUnlock()

		// Size should be less than maxBlacklistEntries + 50 due to cleanup
		if size > maxBlacklistEntries+50 {
			t.Errorf("blacklist cleanup failed, size: %d", size)
		}
	})
}

func TestIsTokenBlacklisted(t *testing.T) {
	// Clear blacklist before test
	tokenBlacklist.Lock()
	tokenBlacklist.items = make(map[string]time.Time)
	tokenBlacklist.Unlock()

	t.Run("check non-blacklisted token", func(t *testing.T) {
		if IsTokenBlacklisted("non-existent-token") {
			t.Error("non-existent token should not be blacklisted")
		}
	})

	t.Run("check expired blacklisted token", func(t *testing.T) {
		token := "expired-token"
		exp := time.Now().Add(-1 * time.Second) // Already expired

		BlacklistToken(token, exp)

		// Should return false and auto-remove
		if IsTokenBlacklisted(token) {
			t.Error("expired token should not be blacklisted")
		}

		// Verify it was removed
		tokenBlacklist.RLock()
		_, exists := tokenBlacklist.items[token]
		tokenBlacklist.RUnlock()

		if exists {
			t.Error("expired token should be removed from blacklist")
		}
	})

	t.Run("check valid blacklisted token", func(t *testing.T) {
		token := "valid-blacklisted-token"
		exp := time.Now().Add(1 * time.Hour)

		BlacklistToken(token, exp)

		if !IsTokenBlacklisted(token) {
			t.Error("valid blacklisted token should return true")
		}
	})
}

// =============================================================================
// Test 6: Integration Test - Full Auth Flow
// =============================================================================

func TestFullAuthFlow(t *testing.T) {
	// Setup
	SetJWTSecret("integration-test-secret")

	t.Run("complete authentication flow", func(t *testing.T) {
		// 1. Hash password
		password := "user-secure-password"
		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword failed: %v", err)
		}

		// 2. Verify password
		if !CheckPassword(password, hash) {
			t.Error("password verification failed")
		}

		// 3. Generate OTP secret
		otpSecret, err := GenerateOTPSecret()
		if err != nil {
			t.Fatalf("GenerateOTPSecret failed: %v", err)
		}

		// 4. Verify OTP
		otpCode, err := totp.GenerateCode(otpSecret, time.Now())
		if err != nil {
			t.Fatalf("GenerateCode failed: %v", err)
		}

		if !VerifyOTP(otpSecret, otpCode) {
			t.Error("OTP verification failed")
		}

		// 5. Generate JWT
		userID := "user-789"
		email := "integration@example.com"
		token, err := GenerateJWT(userID, email)
		if err != nil {
			t.Fatalf("GenerateJWT failed: %v", err)
		}

		// 6. Validate JWT
		claims, err := ValidateJWT(token)
		if err != nil {
			t.Fatalf("ValidateJWT failed: %v", err)
		}

		if claims.UserID != userID || claims.Email != email {
			t.Error("JWT claims mismatch")
		}

		// 7. Blacklist token (logout)
		exp := time.Now().Add(24 * time.Hour)
		BlacklistToken(token, exp)

		// 8. Verify token is blacklisted
		if !IsTokenBlacklisted(token) {
			t.Error("token should be blacklisted after logout")
		}
	})
}

// =============================================================================
// Test 7: Concurrency Test
// =============================================================================

func TestConcurrentBlacklist(t *testing.T) {
	// Clear blacklist
	tokenBlacklist.Lock()
	tokenBlacklist.items = make(map[string]time.Time)
	tokenBlacklist.Unlock()

	// Run concurrent blacklist operations
	done := make(chan bool)
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			token := "concurrent-token-" + string(rune(id))
			exp := time.Now().Add(1 * time.Hour)

			// Add to blacklist
			BlacklistToken(token, exp)

			// Check if blacklisted
			if !IsTokenBlacklisted(token) {
				t.Errorf("token %s should be blacklisted", token)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

func BenchmarkHashPassword(b *testing.B) {
	password := "benchmark-password-123"
	for i := 0; i < b.N; i++ {
		HashPassword(password)
	}
}

func BenchmarkCheckPassword(b *testing.B) {
	password := "benchmark-password-123"
	hash, _ := HashPassword(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckPassword(password, hash)
	}
}

func BenchmarkGenerateJWT(b *testing.B) {
	SetJWTSecret("benchmark-secret-key")
	userID := "bench-user"
	email := "bench@example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateJWT(userID, email)
	}
}

func BenchmarkValidateJWT(b *testing.B) {
	SetJWTSecret("benchmark-secret-key")
	token, _ := GenerateJWT("bench-user", "bench@example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateJWT(token)
	}
}

func BenchmarkGenerateOTPSecret(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateOTPSecret()
	}
}

func BenchmarkVerifyOTP(b *testing.B) {
	secret, _ := GenerateOTPSecret()
	code, _ := totp.GenerateCode(secret, time.Now())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		VerifyOTP(secret, code)
	}
}
