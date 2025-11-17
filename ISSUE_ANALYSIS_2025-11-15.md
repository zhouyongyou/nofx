# 問題分析報告 - 2025-11-15

## 用戶提出的三個問題

### ❓ 問題 1：雲部署時 CORS 會是阻礙嗎？

**現狀分析**：

目前的 CORS middleware 配置：
```go
// api/server.go:122
isDevelopment := os.Getenv("ENVIRONMENT") != "production"
```

**關鍵發現**：
- ✅ **docker-compose.yml 沒有設置 `ENVIRONMENT` 變量**
- ✅ 因此默認 `isDevelopment = true`（開發模式）
- ✅ 開發模式下極度友好：記錄警告但允許所有請求

**雲部署場景分析**：

| 部署方式 | ENVIRONMENT 值 | CORS 行為 | 是否有問題 |
|---------|---------------|-----------|----------|
| Docker Compose（默認） | 未設置 | 開發模式，允許所有 | ✅ 無問題 |
| 用戶手動設 `ENVIRONMENT=production` | production | 嚴格白名單 | ⚠️ 需配置 |
| 雲服務自動注入（AWS/GCP） | production | 嚴格白名單 | ⚠️ 需配置 |

**結論**：
1. **目前用戶部署：不會遇到 CORS 問題**（默認開發模式）
2. **未來生產環境：需要改進**（見下方建議）

---

### ❓ 問題 2：「交易所不存在」能否預防？

**現狀分析**：

前端的「刪除交易所」操作（`web/src/hooks/useTraderActions.ts:442-469`）：
```typescript
handleDeleteExchange = async (exchangeId: string) => {
  // 1. 檢查是否有 trader 在使用
  checkInUse: isExchangeUsedByAnyTrader,

  // 2. 不是真正刪除，而是清空敏感字段
  clearFields: (e) => ({
    ...e,
    apiKey: '',
    secretKey: '',
    enabled: false,  // 禁用而非刪除
  }),

  // 3. 調用 PUT /api/exchanges 更新
}
```

**問題來源**：
- ❌ **數據庫沒有外鍵約束**（SQLite 默認不強制）
- ❌ **手動編輯數據庫**（用戶可能直接刪除 exchanges 記錄）
- ❌ **舊版本遺留數據**（遷移時外鍵映射錯誤）

**當前保護機制**：
- ✅ 前端檢查「是否有 trader 使用」
- ✅ 前端不真正刪除，只是禁用
- ❌ 後端沒有強制外鍵約束
- ❌ 啟動時沒有數據完整性檢查

**結論**：
目前只提供修復工具，**沒有預防機制**（見下方建議）

---

### ❓ 問題 3：AI 模型無法保存配置？

**代碼檢查**：

`config/database.go:1261-1291` 的 `UpdateAIModel` 函數：

```go
// 1. 檢查表結構（兼容新舊版本）
var hasModelIDColumn int
err := d.db.QueryRow(`
    SELECT COUNT(*) FROM pragma_table_info('ai_models')
    WHERE name = 'model_id'
`).Scan(&hasModelIDColumn)

// 2. 先嘗試精確匹配 model_id
err = d.db.QueryRow(`
    SELECT model_id FROM ai_models WHERE user_id = ? AND model_id = ? LIMIT 1
`, userID, id).Scan(&existingModelID)

// 3. 如果找到，更新它
if err == nil {
    _, err = d.db.Exec(`
        UPDATE ai_models SET enabled = ?, api_key = ?, ...
        WHERE model_id = ? AND user_id = ?
    `, enabled, encryptedAPIKey, ..., existingModelID, userID)
}

// 4. 如果沒找到，嘗試通過 provider 查找（兼容舊邏輯）
```

**可能的問題點**：
1. ⚠️ **表結構檢查可能失敗**（pragma_table_info 權限問題）
2. ⚠️ **model_id 匹配失敗**（ID 格式不一致）
3. ⚠️ **加密失敗**（encryptSensitiveData 返回空）
4. ⚠️ **沒有錯誤返回給前端**（可能被靜默忽略）

**需要診斷**：
- 用戶具體報錯信息（前端 console 或後端 log）
- 哪個 AI 模型無法保存（OpenAI? DeepSeek? Custom?）
- 是否所有模型都無法保存，還是特定模型

---

## 🛠️ 建議的改進方案

### 1. CORS 雲部署優化（高優先級）

#### 方案 A：自動檢測前端 URL（推薦）

在啟動時自動添加當前訪問的前端 URL：

```go
// api/server.go 啟動日誌中添加提示
log.Printf("🌐 [CORS] 當前允許的來源:")
for _, origin := range allowedOrigins {
    log.Printf("    • %s", origin)
}
log.Printf("💡 提示：如果您的前端部署在其他地址，請設置環境變量：")
log.Printf("   CORS_ALLOWED_ORIGINS=https://your-frontend-url.com")
```

#### 方案 B：動態學習模式（激進）

```go
// 開發模式下，自動記錄訪問的 Origin 並添加到白名單
if isDevelopment && origin != "" && !isInWhitelist(origin) {
    log.Printf("🔓 [CORS] 自動添加新來源到臨時白名單: %s", origin)
    allowedOrigins = append(allowedOrigins, origin)
}
```

#### 方案 C：改進文檔和部署腳本（最安全）

在 `.env.example` 添加雲部署範例：

```bash
# 雲部署範例
# AWS EC2
# CORS_ALLOWED_ORIGINS=http://ec2-xx-xx-xx-xx.compute.amazonaws.com

# Vercel + Railway
# CORS_ALLOWED_ORIGINS=https://my-app.vercel.app

# 自定義域名
# CORS_ALLOWED_ORIGINS=https://trading.example.com
```

---

### 2. 數據庫完整性預防（中優先級）

#### 方案 A：添加外鍵約束（推薦）

創建遷移腳本 `scripts/add_foreign_key_constraints.sh`：

```sql
-- 啟用外鍵支持
PRAGMA foreign_keys = ON;

-- 重建 traders 表（添加外鍵約束）
CREATE TABLE traders_new (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    exchange_id INTEGER NOT NULL,
    ...,
    FOREIGN KEY (exchange_id) REFERENCES exchanges(id) ON DELETE RESTRICT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 遷移數據（只遷移有效的外鍵引用）
INSERT INTO traders_new
SELECT * FROM traders t
WHERE EXISTS (SELECT 1 FROM exchanges e WHERE e.id = t.exchange_id);

-- 替換舊表
DROP TABLE traders;
ALTER TABLE traders_new RENAME TO traders;
```

#### 方案 B：啟動時完整性檢查（補充）

在 `manager/trader_manager.go` 啟動時添加：

```go
// LoadTradersFromDatabase 開頭添加
func (tm *TraderManager) LoadTradersFromDatabase() error {
    log.Println("🔍 [啟動檢查] 驗證數據庫完整性...")

    // 檢查孤立的 traders
    orphanedCount := tm.database.CheckOrphanedTraders()
    if orphanedCount > 0 {
        log.Printf("⚠️  發現 %d 個引用無效交易所的 trader", orphanedCount)
        log.Printf("    請執行修復腳本: docker exec -it nofx-api-1 bash -c 'cd /app/scripts && ./fix_missing_exchange_references.sh'")
        // 不中斷啟動，但記錄警告
    }

    // 原有的加載邏輯...
}
```

#### 方案 C：後端刪除保護（最安全）

即使前端已有檢查，後端也應該強制執行：

```go
// api/server.go 添加 DELETE /api/exchanges/:id endpoint
func (s *Server) handleDeleteExchange(c *gin.Context) {
    exchangeID := c.Param("id")
    userID := c.GetString("user_id")

    // 檢查是否有 trader 使用
    traders, _ := s.database.GetTradersUsingExchange(userID, exchangeID)
    if len(traders) > 0 {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "無法刪除交易所，有 trader 正在使用",
            "traders": traders,
        })
        return
    }

    // 軟刪除（設置 enabled=false）
    err := s.database.DisableExchange(userID, exchangeID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "交易所已禁用"})
}
```

---

### 3. AI 模型保存問題診斷（緊急）

#### 步驟 1：添加詳細日誌

在 `api/server.go:1691` 添加：

```go
log.Printf("🔧 [AI Model] 開始更新模型 %s (用戶 %s)", modelID, userID)
log.Printf("    enabled=%v, apiKey長度=%d, customURL=%s",
    modelData.Enabled, len(modelData.APIKey), modelData.CustomAPIURL)

err := s.database.UpdateAIModel(userID, modelID, ...)
if err != nil {
    log.Printf("❌ [AI Model] 更新失敗: %v", err)
    c.JSON(http.StatusInternalServerError, gin.H{
        "error": fmt.Sprintf("更新模型失敗: %v", err),
    })
    return
}

log.Printf("✅ [AI Model] 模型 %s 更新成功", modelID)
```

在 `config/database.go:1261` 添加：

```go
func (d *Database) UpdateAIModel(...) error {
    log.Printf("🔍 [DB] UpdateAIModel: userID=%s, id=%s", userID, id)

    // 檢查表結構
    var hasModelIDColumn int
    err := d.db.QueryRow(...).Scan(&hasModelIDColumn)
    if err != nil {
        log.Printf("❌ [DB] 檢查表結構失敗: %v", err)
        return fmt.Errorf("检查ai_models表结构失败: %w", err)
    }
    log.Printf("    hasModelIDColumn=%d", hasModelIDColumn)

    // ... 後續邏輯每步都添加日誌
}
```

#### 步驟 2：前端錯誤處理改進

檢查 `web/src/pages/AITradersPage.tsx` 是否正確處理錯誤：

```typescript
const handleSaveModel = async (modelId: string, data: any) => {
  try {
    const response = await fetch('/api/ai-models', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ... },
      body: JSON.stringify({ models: { [modelId]: data } }),
    });

    if (!response.ok) {
      const error = await response.json();
      console.error('❌ AI模型保存失敗:', error);
      alert(`保存失敗: ${error.error || '未知錯誤'}`);
      return;
    }

    console.log('✅ AI模型保存成功');
  } catch (e) {
    console.error('❌ 網絡錯誤:', e);
    alert(`網絡錯誤: ${e.message}`);
  }
};
```

#### 步驟 3：提供診斷工具

創建 `scripts/diagnose_ai_models.sh`：

```bash
#!/bin/bash
echo "🔍 診斷 AI 模型配置"
docker exec -it nofx-api-1 sqlite3 /data/nofx.db <<EOF
.mode column
.headers on
SELECT * FROM ai_models;
EOF
```

---

## 📋 執行優先級

### 🔥 緊急（本週完成）
1. ✅ **AI 模型保存問題診斷**
   - 添加詳細日誌
   - 用戶提供具體報錯信息
   - 修復根本原因

### ⚡ 高優先級（下週完成）
2. ⚠️ **數據庫完整性預防**
   - 添加外鍵約束遷移腳本
   - 啟動時完整性檢查

3. ⚠️ **CORS 雲部署優化**
   - 改進啟動日誌提示
   - 添加雲部署文檔

### 📌 中優先級（兩週內）
4. 後端刪除保護（雙重驗證）
5. 前端錯誤處理改進

---

## 🎯 總結

| 問題 | 當前狀態 | 是否阻礙用戶 | 建議方案 |
|------|---------|------------|---------|
| 雲部署 CORS | 默認開發模式，不阻礙 | ❌ 否 | 改進文檔和日誌 |
| 交易所不存在 | 已提供修復工具 | ⚠️ 部分（舊數據） | 添加外鍵約束 |
| AI 模型保存 | **需要診斷** | ❓ 未知 | **立即調查** |

**下一步**：請用戶提供 AI 模型保存的具體報錯信息（前端 console 或後端日誌），以便精準修復。
