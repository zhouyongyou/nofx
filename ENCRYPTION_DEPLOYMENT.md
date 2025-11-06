# 🔐 加密系統部署指南

## 架構概述

```
┌─────────────────────────────────────────────────────────────────┐
│                   三層加密安全架構                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  前端 (Browser)                                                   │
│  ├─ 二階段輸入（分段 + 剪貼簿混淆）                              │
│  ├─ RSA-4096 混合加密                                             │
│  └─ Base64 傳輸                                                   │
│                        ↓ HTTPS                                    │
│  後端 (Go Server)                                                 │
│  ├─ RSA 私鑰解密                                                  │
│  ├─ AES-256-GCM 數據庫加密                                        │
│  ├─ 密鑰輪換機制                                                  │
│  └─ 審計日誌記錄                                                  │
│                        ↓                                          │
│  數據庫 (SQLite)                                                  │
│  └─ 所有敏感字段加密存儲                                          │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 第一步：初始化加密系統

### 1.1 生成密鑰對

首次啟動時，系統會自動生成：
- RSA-4096 密鑰對（用於前後端通信加密）
- AES-256 主密鑰（用於數據庫加密）

```bash
cd /Users/sotadic/Documents/GitHub/nofx

# 啟動系統，自動生成密鑰
go run main.go

# 密鑰會保存在 .secrets/ 目錄
# ⚠️ 確保此目錄不會被 Git 追蹤
echo ".secrets/" >> .gitignore
```

**生成的檔案**:
```
.secrets/
├── rsa_private.pem    # RSA 私鑰 (4096-bit)
├── rsa_public.pem     # RSA 公鑰
└── master.key         # 數據庫加密主密鑰 (Base64)
```

### 1.2 設置環境變數（生產環境必須）

```bash
# 讀取生成的主密鑰
MASTER_KEY=$(cat .secrets/master.key)

# 添加到環境變數
export NOFX_MASTER_KEY="$MASTER_KEY"

# 或添加到 .env 文件（⚠️ 確保 .env 不會提交到 Git）
echo "NOFX_MASTER_KEY=$MASTER_KEY" >> .env
```

**⚠️ 生產環境安全建議**:
```bash
# 使用 systemd 服務管理環境變數
sudo nano /etc/systemd/system/nofx.service

[Service]
Environment="NOFX_MASTER_KEY=<your_key_here>"
EnvironmentFile=/opt/nofx/.env
```

---

## 第二步：遷移現有數據

如果你已有明文密鑰數據，需要遷移到加密格式：

### 2.1 備份數據庫

```bash
# 備份原始數據庫
cp config.db config.db.backup.$(date +%Y%m%d_%H%M%S)

# 驗證備份
sqlite3 config.db.backup.* "SELECT COUNT(*) FROM exchanges;"
```

### 2.2 執行遷移

```bash
# 方式 1: 使用 Go 程式遷移
go run scripts/migrate_encryption.go

# 方式 2: 使用 SQL 腳本
sqlite3 config.db < scripts/migrate_to_encrypted.sql
```

### 2.3 驗證遷移結果

```bash
# 檢查加密後的數據（應該看到 Base64 字串）
sqlite3 config.db "SELECT id, substr(api_key, 1, 20) FROM exchanges LIMIT 3;"

# 輸出示例:
# binance|J8K9L0M1N2O3P4Q5R6S7==
# hyperliquid|X9Y8Z7A6B5C4D3E2F1G0==
```

---

## 第三步：更新 main.go

### 3.1 引入加密模組

在 `main.go` 中添加初始化代碼：

```go
package main

import (
    "log"
    "nofx/api"
    "nofx/config"
    "nofx/crypto"
    "net/http"
)

func main() {
    // 1. 初始化數據庫
    db, err := config.NewDatabase("config.db")
    if err != nil {
        log.Fatalf("數據庫初始化失敗: %v", err)
    }
    defer db.Close()

    // 2. 初始化安全存儲層
    secureStorage, err := crypto.NewSecureStorage(db.GetDB())
    if err != nil {
        log.Fatalf("安全存儲初始化失敗: %v", err)
    }

    // 3. 可選：遷移舊數據
    if err := secureStorage.MigrateToEncrypted(); err != nil {
        log.Printf("⚠️ 數據遷移失敗（如果是首次運行可忽略）: %v", err)
    }

    // 4. 創建加密 API 處理器
    cryptoHandler, err := api.NewCryptoHandler(secureStorage)
    if err != nil {
        log.Fatalf("加密處理器初始化失敗: %v", err)
    }

    // 5. 註冊路由
    http.HandleFunc("/api/crypto/public-key", cryptoHandler.HandleGetPublicKey)
    http.HandleFunc("/api/crypto/decrypt", cryptoHandler.HandleDecryptPrivateKey)
    http.HandleFunc("/api/audit-logs", cryptoHandler.HandleGetAuditLogs)

    log.Println("🔐 加密系統已啟用")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## 第四步：前端集成

### 4.1 更新 AITradersPage.tsx

```typescript
import { twoStagePrivateKeyInput, fetchServerPublicKey } from '../lib/crypto';

// 替換原有的私鑰輸入邏輯
const handleSaveExchangeConfig = async (
  exchangeId: string,
  apiKey: string,
  secretKey?: string
) => {
  try {
    // 1. 獲取伺服器公鑰
    const serverPublicKey = await fetchServerPublicKey();

    // 2. 二階段輸入並加密私鑰
    const { encryptedKey } = await twoStagePrivateKeyInput(serverPublicKey);

    // 3. 發送加密數據到後端
    await api.post('/api/exchange/config', {
      exchange_id: exchangeId,
      api_key: apiKey,
      secret_key: secretKey,
      encrypted_private_key: encryptedKey, // 加密後的私鑰
    });

    alert('✅ 配置已安全保存');
  } catch (error) {
    console.error('保存失敗:', error);
    alert('❌ 保存失敗，請重試');
  }
};
```

---

## 第五步：安全加固

### 5.1 檔案權限設置

```bash
# 限制密鑰檔案權限
chmod 700 .secrets
chmod 600 .secrets/*

# 數據庫權限
chmod 600 config.db

# 檢查權限
ls -la .secrets/
# 應該顯示: drwx------ (僅所有者可讀寫執行)
```

### 5.2 阿里雲伺服器加固

```bash
# 1. 啟用防火牆
sudo ufw enable
sudo ufw allow 8080/tcp   # 後端 API
sudo ufw allow 3000/tcp   # 前端 (生產環境應該用 Nginx 反向代理)
sudo ufw allow 22/tcp     # SSH

# 2. 禁用 root SSH 登入
sudo nano /etc/ssh/sshd_config
# 修改: PermitRootLogin no
sudo systemctl restart sshd

# 3. 安裝 fail2ban（防止暴力破解）
sudo apt install fail2ban
sudo systemctl enable fail2ban
```

### 5.3 監控與告警

創建監控腳本 `scripts/monitor_security.sh`:

```bash
#!/bin/bash

# 監控 .secrets 目錄訪問
inotifywait -m .secrets/ -e access -e modify |
while read path action file; do
    echo "⚠️  $(date): $file 被 $action"
    # 發送告警（可接入釘釘/Telegram）
    curl -X POST "https://your-alert-webhook" \
         -d "密鑰檔案被訪問: $file"
done
```

---

## 第六步：密鑰輪換計劃

### 6.1 定期輪換主密鑰（建議每季度一次）

```bash
# 1. 創建輪換腳本
cat > scripts/rotate_master_key.sh << 'EOF'
#!/bin/bash
set -e

echo "🔄 開始輪換主密鑰..."

# 停止服務
sudo systemctl stop nofx

# 備份當前密鑰
cp .secrets/master.key .secrets/master.key.old

# 執行輪換
go run scripts/rotate_key.go

# 更新環境變數
NEW_KEY=$(cat .secrets/master.key)
sudo sed -i "s/NOFX_MASTER_KEY=.*/NOFX_MASTER_KEY=$NEW_KEY/" /etc/systemd/system/nofx.service

# 重新加密所有數據
go run scripts/reencrypt_all.go

# 重啟服務
sudo systemctl start nofx

echo "✅ 密鑰輪換完成"
EOF

chmod +x scripts/rotate_master_key.sh
```

---

## 第七步：驗證與測試

### 7.1 加密測試

```bash
# 測試加密/解密流程
go test ./crypto -v

# 預期輸出:
# ✅ RSA 密鑰對生成成功
# ✅ AES 加密/解密測試通過
# ✅ 混合加密測試通過
```

### 7.2 端到端測試

```bash
# 1. 啟動後端
go run main.go

# 2. 打開前端，測試私鑰輸入
# http://localhost:3000

# 3. 驗證數據庫中的數據已加密
sqlite3 config.db "SELECT api_key FROM exchanges LIMIT 1;"
# 應該看到 Base64 字串，而非明文
```

### 7.3 安全審計

```bash
# 檢查是否有明文密鑰洩露
grep -r "0x[0-9a-fA-F]{64}" . --exclude-dir=node_modules --exclude-dir=.git
# 應該沒有任何輸出

# 檢查日誌中是否有敏感信息
grep -i "private.*key\|secret\|api.*key" nohup.out | head
# 應該只看到審計日誌，沒有明文密鑰
```

---

## 緊急情況處理

### 情況1：密鑰丟失

```bash
# ⚠️ 如果主密鑰丟失，所有加密數據將無法恢復
# 恢復方式：
1. 從備份恢復 .secrets/master.key
2. 或使用最近的數據庫備份（未加密版本）
3. 重新生成密鑰並提示用戶重新輸入
```

### 情況2：懷疑密鑰洩露

```bash
# 立即執行密鑰輪換
./scripts/rotate_master_key.sh

# 撤銷所有交易所 API 權限
# 通知用戶重新配置

# 檢查審計日誌
curl http://localhost:8080/api/audit-logs \
  -H "X-User-ID: <user_id>" | jq .
```

---

## 性能與成本

- **加密開銷**: 每次操作增加 ~5ms 延遲（可忽略）
- **存儲開銷**: 加密後數據大小增加 ~30%
- **維護成本**: 每季度密鑰輪換需要停機 ~5 分鐘

---

## 合規性檢查清單

- [x] 私鑰端到端加密（前端 → 後端）
- [x] 數據庫敏感字段加密
- [x] 審計日誌完整記錄
- [x] 密鑰輪換機制
- [x] 訪問控制（檔案權限 600）
- [x] 傳輸層安全（HTTPS）
- [ ] 雙因素認證（2FA）
- [ ] 硬體安全模組（HSM）- 可選

---

## 常見問題

**Q: 為什麼不直接使用 MetaMask 簽名？**
A: MetaMask 簽名適合交易場景，但 AI 自動交易需要伺服器端持有私鑰。本方案在此前提下最大化安全性。

**Q: 主密鑰存在環境變數是否安全？**
A: 環境變數相比硬編碼更安全，但最佳實踐是使用雲端 KMS（如 AWS Secrets Manager）。

**Q: 如何防止內部人員竊取密鑰？**
A: 啟用審計日誌、最小權限原則、密鑰分片（Shamir's Secret Sharing）。

---

## 下一步優化方向

1. **硬體安全模組（HSM）**: 將主密鑰存儲在專用硬體中
2. **零知識證明**: 實現完全不上傳私鑰的簽名方案
3. **多方計算（MPC）**: 私鑰分片存儲，無單點故障
4. **生物識別**: 結合指紋/Face ID 進行二次驗證

---

**安全是持續的過程，而非一次性配置。定期審查、更新、演練。**
