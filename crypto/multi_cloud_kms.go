package crypto

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// CloudKMSProvider 雲端 KMS 提供商
type CloudKMSProvider string

const (
	CloudKMSNone   CloudKMSProvider = "none"   // 不使用雲端 KMS
	CloudKMSAliyun CloudKMSProvider = "aliyun" // 阿里雲 KMS
	CloudKMSGCP    CloudKMSProvider = "gcp"    // Google Cloud KMS
)

// CloudKMSConfig 雲端 KMS 配置
type CloudKMSConfig struct {
	Provider CloudKMSProvider

	// Aliyun KMS 配置
	AliyunRegionID        string
	AliyunAccessKeyID     string
	AliyunAccessKeySecret string
	AliyunKeyID           string

	// GCP KMS 配置
	GCPKeyName string // projects/PROJECT_ID/locations/LOCATION/keyRings/KEY_RING/cryptoKeys/KEY_NAME
}

// MultiCloudKMSManager 多雲 KMS 管理器
type MultiCloudKMSManager struct {
	provider      CloudKMSProvider
	aliyunKMS     *AliyunKMSManager
	gcpKMS        *GCPKMSManager
	localFallback *EncryptionManager
}

// NewMultiCloudKMSManager 創建多雲 KMS 管理器
func NewMultiCloudKMSManager(config *CloudKMSConfig) (*MultiCloudKMSManager, error) {
	// 初始化本地加密（必須，作為回退）
	localFallback, err := GetEncryptionManager()
	if err != nil {
		return nil, fmt.Errorf("初始化本地加密失敗: %w", err)
	}

	manager := &MultiCloudKMSManager{
		provider:      config.Provider,
		localFallback: localFallback,
	}

	// 根據提供商初始化對應的 KMS
	switch config.Provider {
	case CloudKMSAliyun:
		aliyunKMS, err := NewAliyunKMSManager(
			config.AliyunRegionID,
			config.AliyunAccessKeyID,
			config.AliyunAccessKeySecret,
			config.AliyunKeyID,
		)
		if err != nil {
			log.Printf("⚠️ Aliyun KMS 初始化失敗，使用本地加密: %v", err)
			manager.provider = CloudKMSNone
		} else {
			manager.aliyunKMS = aliyunKMS
			log.Println("✓ 多雲 KMS 管理器：Aliyun KMS")
		}

	case CloudKMSGCP:
		gcpKMS, err := NewGCPKMSManager(config.GCPKeyName)
		if err != nil {
			log.Printf("⚠️ GCP KMS 初始化失敗，使用本地加密: %v", err)
			manager.provider = CloudKMSNone
		} else {
			manager.gcpKMS = gcpKMS
			log.Println("✓ 多雲 KMS 管理器：GCP KMS")
		}

	case CloudKMSNone:
		log.Println("✓ 多雲 KMS 管理器：僅本地加密")

	default:
		return nil, fmt.Errorf("不支持的 KMS 提供商: %s", config.Provider)
	}

	return manager, nil
}

// Encrypt 加密數據
func (m *MultiCloudKMSManager) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	switch m.provider {
	case CloudKMSAliyun:
		if m.aliyunKMS != nil {
			return m.aliyunKMS.Encrypt(plaintext)
		}

	case CloudKMSGCP:
		if m.gcpKMS != nil {
			return m.gcpKMS.Encrypt(plaintext)
		}
	}

	// 回退到本地加密
	return m.localFallback.EncryptForDatabase(plaintext)
}

// Decrypt 解密數據
func (m *MultiCloudKMSManager) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// 自動檢測加密方式並嘗試解密
	switch m.provider {
	case CloudKMSAliyun:
		if m.aliyunKMS != nil {
			plaintext, err := m.aliyunKMS.Decrypt(ciphertext)
			if err == nil {
				return plaintext, nil
			}
		}

	case CloudKMSGCP:
		if m.gcpKMS != nil {
			plaintext, err := m.gcpKMS.Decrypt(ciphertext)
			if err == nil {
				return plaintext, nil
			}
		}
	}

	// 回退到本地解密
	return m.localFallback.DecryptFromDatabase(ciphertext)
}

// Close 關閉 KMS 管理器
func (m *MultiCloudKMSManager) Close() error {
	if m.aliyunKMS != nil {
		return m.aliyunKMS.Close()
	}
	if m.gcpKMS != nil {
		return m.gcpKMS.Close()
	}
	return nil
}

// GetProvider 獲取當前使用的 KMS 提供商
func (m *MultiCloudKMSManager) GetProvider() CloudKMSProvider {
	return m.provider
}

// LoadConfigFromEnv 從環境變量加載 KMS 配置
func LoadConfigFromEnv() *CloudKMSConfig {
	config := &CloudKMSConfig{}

	// 檢測 KMS 提供商
	provider := strings.ToLower(os.Getenv("NOFX_KMS_PROVIDER"))

	switch provider {
	case "gcp", "google":
		config.Provider = CloudKMSGCP
		config.GCPKeyName = os.Getenv("NOFX_GCP_KMS_KEY_NAME")
		if config.GCPKeyName == "" {
			log.Println("⚠️ GCP KMS 配置不完整，回退到本地加密")
			config.Provider = CloudKMSNone
		}

	case "aliyun", "alicloud":
		config.Provider = CloudKMSAliyun
		config.AliyunRegionID = os.Getenv("NOFX_ALIYUN_REGION_ID")
		config.AliyunAccessKeyID = os.Getenv("NOFX_ALIYUN_ACCESS_KEY_ID")
		config.AliyunAccessKeySecret = os.Getenv("NOFX_ALIYUN_ACCESS_KEY_SECRET")
		config.AliyunKeyID = os.Getenv("NOFX_ALIYUN_KEY_ID")

		if config.AliyunRegionID == "" || config.AliyunAccessKeyID == "" ||
			config.AliyunAccessKeySecret == "" || config.AliyunKeyID == "" {
			log.Println("⚠️ Aliyun KMS 配置不完整，回退到本地加密")
			config.Provider = CloudKMSNone
		}

	default:
		// 默認使用本地加密
		config.Provider = CloudKMSNone
	}

	return config
}
