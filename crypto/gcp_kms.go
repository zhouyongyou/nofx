package crypto

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

// GCPKMSManager Google Cloud KMS 管理器
type GCPKMSManager struct {
	client        *kms.KeyManagementClient
	keyName       string // 完整的密鑰資源名稱
	ctx           context.Context
	localFallback *EncryptionManager // 本地加密回退
}

// NewGCPKMSManager 創建 GCP KMS 管理器
// keyName 格式: projects/PROJECT_ID/locations/LOCATION/keyRings/KEY_RING/cryptoKeys/KEY_NAME
func NewGCPKMSManager(keyName string) (*GCPKMSManager, error) {
	ctx := context.Background()

	// 創建 KMS 客戶端
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("創建 GCP KMS 客戶端失敗: %w", err)
	}

	// 初始化本地回退加密
	localFallback, err := GetEncryptionManager()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("初始化本地加密回退失敗: %w", err)
	}

	log.Printf("✓ GCP KMS 管理器初始化成功: %s", keyName)

	return &GCPKMSManager{
		client:        client,
		keyName:       keyName,
		ctx:           ctx,
		localFallback: localFallback,
	}, nil
}

// Encrypt 使用 GCP KMS 加密數據
func (m *GCPKMSManager) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	// 構建加密請求
	req := &kmspb.EncryptRequest{
		Name:      m.keyName,
		Plaintext: []byte(plaintext),
	}

	// 調用 GCP KMS 加密
	result, err := m.client.Encrypt(m.ctx, req)
	if err != nil {
		log.Printf("⚠️ GCP KMS 加密失敗，使用本地加密回退: %v", err)
		// 回退到本地加密
		return m.localFallback.EncryptForDatabase(plaintext)
	}

	// Base64 編碼密文
	ciphertext := base64.StdEncoding.EncodeToString(result.Ciphertext)
	return ciphertext, nil
}

// Decrypt 使用 GCP KMS 解密數據
func (m *GCPKMSManager) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Base64 解碼
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		// 可能是本地加密的數據，嘗試本地解密
		return m.localFallback.DecryptFromDatabase(ciphertext)
	}

	// 構建解密請求
	req := &kmspb.DecryptRequest{
		Name:       m.keyName,
		Ciphertext: ciphertextBytes,
	}

	// 調用 GCP KMS 解密
	result, err := m.client.Decrypt(m.ctx, req)
	if err != nil {
		log.Printf("⚠️ GCP KMS 解密失敗，嘗試本地解密: %v", err)
		// 回退到本地解密
		return m.localFallback.DecryptFromDatabase(ciphertext)
	}

	return string(result.Plaintext), nil
}

// Close 關閉 KMS 客戶端
func (m *GCPKMSManager) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}
