# GCP KMS 配置指南

## 快速開始

### 1. 環境變量配置

```bash
# 使用 GCP KMS
export NOFX_KMS_PROVIDER=gcp
export NOFX_GCP_KMS_KEY_NAME="projects/my-project/locations/us/keyRings/nofx/cryptoKeys/nofx-key"
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json

# 使用 Aliyun KMS
export NOFX_KMS_PROVIDER=aliyun
export NOFX_ALIYUN_REGION_ID=cn-hangzhou
export NOFX_ALIYUN_ACCESS_KEY_ID=LTAI...
export NOFX_ALIYUN_ACCESS_KEY_SECRET=...
export NOFX_ALIYUN_KEY_ID=...

# 僅使用本地加密
export NOFX_KMS_PROVIDER=none
export NOFX_MASTER_KEY=$(openssl rand -base64 32)
```

### 2. 代碼集成

```go
import "nofx/crypto"

// 從環境變量加載配置
config := crypto.LoadConfigFromEnv()

// 創建多雲 KMS 管理器
kmsManager, err := crypto.NewMultiCloudKMSManager(config)
if err != nil {
    log.Fatal(err)
}
defer kmsManager.Close()

// 使用
encrypted, _ := kmsManager.Encrypt("sensitive data")
plaintext, _ := kmsManager.Decrypt(encrypted)
```

## GCP KMS 設置

### 創建密鑰

```bash
# 創建密鑰環
gcloud kms keyrings create nofx-keyring --location=us

# 創建密鑰
gcloud kms keys create nofx-key \
  --keyring=nofx-keyring \
  --location=us \
  --purpose=encryption
```

### 服務賬戶權限

```bash
# 創建服務賬戶
gcloud iam service-accounts create nofx-kms

# 授予權限
gcloud kms keys add-iam-policy-binding nofx-key \
  --keyring=nofx-keyring \
  --location=us \
  --member="serviceAccount:nofx-kms@PROJECT.iam.gserviceaccount.com" \
  --role="roles/cloudkms.cryptoKeyEncrypterDecrypter"
```

## 成本對比

| 方案 | 100 用戶 | 1,000 用戶 | 10,000 用戶 |
|-----|---------|-----------|------------|
| 本地加密 | $0 | $0 | $0 |
| Aliyun KMS | ¥30/月 | ¥30/月 | ¥30/月 |
| GCP KMS | $0.21/月 | $1.02/月 | $9.18/月 |

## 故障轉移

系統會自動回退到本地加密：

```
GCP KMS 失敗 → 本地加密
Aliyun KMS 失敗 → 本地加密
```

確保始終設置 `NOFX_MASTER_KEY` 作為回退。
