# 📄 config.json.example 說明文檔

> **注意**: 標準 JSON 不支持 `//` 註釋，所以本文檔補充說明配置範例的每個欄位用途。

## 如何使用

```bash
# 1. 複製範例文件
cp config.json.example config.json

# 2. 根據下方說明編輯 config.json
nano config.json

# 3. 啟動系統
./nofx
```

---

## 配置欄位說明

### 基本設定

```json
"admin_mode": true
```
- **說明**: 管理員模式，跳過登入驗證
- **建議**: 
  - 開發環境: `true` (方便測試)
  - 生產環境: `false` (需要登入)

---

### 杠桿配置

```json
"leverage": {
  "btc_eth_leverage": 5,
  "altcoin_leverage": 5
}
```
- **說明**: 
  - `btc_eth_leverage`: BTC/ETH 的杠桿倍數
  - `altcoin_leverage`: 山寨幣的杠桿倍數
- **建議**: 
  - 新手: 3-5x (安全)
  - 有經驗: 5-10x (平衡)
  - ⚠️ 高風險: >10x (容易爆倉)

---

### 交易幣種

```json
"use_default_coins": true,
"default_coins": [
  "BTCUSDT",
  "ETHUSDT",
  "SOLUSDT",
  ...
]
```
- **說明**: 
  - `use_default_coins: true` → 使用內建幣種列表
  - `use_default_coins: false` → 使用外部 API 獲取幣種
- **建議**: 
  - 新手: 保持 `true` (使用預設的主流幣)
  - 進階: 設為 `false` 並配置 `coin_pool_api_url`

---

### 外部數據源

```json
"coin_pool_api_url": "",
"oi_top_api_url": ""
```
- **說明**: 
  - `coin_pool_api_url`: 自定義幣種池 API
  - `oi_top_api_url`: 持倉量排行 API
- **何時使用**: 
  - 空字符串 (`""`) → 使用內建數據
  - 填入 URL → 使用外部 API (進階用戶)

---

### 風險控制

```json
"max_daily_loss": 10.0,
"max_drawdown": 20.0,
"stop_trading_minutes": 60
```
- **說明**: 
  - `max_daily_loss`: 單日最大虧損百分比 (觸發後停止交易)
  - `max_drawdown`: 最大回撤百分比
  - `stop_trading_minutes`: 觸發風控後暫停交易的時間 (分鐘)
- **建議**: 
  - 保守: `max_daily_loss: 5.0`
  - 中等: `max_daily_loss: 10.0` (預設)
  - 激進: `max_daily_loss: 20.0`

---

### JWT 密鑰

```json
"jwt_secret": "Qk0kAa+d0iIEzXVHXbNbm+UaN3RNabmWtH8rDWZ5OPf..."
```
- **說明**: 用於用戶認證的密鑰
- **⚠️ 安全警告**: 
  - 生產環境必須更換！
  - 使用以下命令生成新密鑰:
    ```bash
    openssl rand -base64 64
    ```

---

### 新聞源配置 (可選功能)

```json
"news": [
  {
    "provider": "telegram",
    "telegram": {
      "proxyurl": "http://127.0.0.1:18080"
    },
    "channels": [
      {
        "id": "ChannelPANews",
        "name": "PANews"
      }
    ]
  }
]
```

**⚠️ 重要提示**: 目前新聞源功能還比較初級，建議使用時刪除或保持預設值

#### 欄位說明:

**`proxyurl`**:
- **用途**: Telegram 代理地址
- **何時需要**: 
  - ✅ 中國大陸伺服器: 需要配置代理
  - ❌ 國外伺服器: 留空或刪除此行

**`channels.id`**:
- **用途**: Telegram 頻道 ID
- **如何獲取**: 
  - 例如頻道網址是 `t.me/ChannelPANews`
  - 則 `id` 為 `"ChannelPANews"` (去掉 t.me/)

**`channels.name`**:
- **用途**: 頻道顯示名稱 (僅用於識別)
- **建議**: 填入易識別的名稱

#### 示例配置:

```json
// 中國大陸伺服器 (需要代理)
"telegram": {
  "proxyurl": "http://127.0.0.1:18080"
}

// 國外伺服器 (無需代理)
"telegram": {
  "proxyurl": ""
}

// 或直接刪除 telegram 整個配置塊
"news": []
```

#### 推薦的 Telegram 頻道:

| 頻道 ID | 頻道名稱 | 內容類型 |
|---------|---------|---------|
| `ChannelPANews` | PANews | 加密貨幣新聞 |
| `cointelegraph` | Cointelegraph | 區塊鏈資訊 |
| `BitcoinMagazine` | Bitcoin Magazine | 比特幣專題 |

---

## 完整範例

### 示例 1: 保守型配置 (新手推薦)

```json
{
  "admin_mode": false,
  "leverage": {
    "btc_eth_leverage": 3,
    "altcoin_leverage": 3
  },
  "use_default_coins": true,
  "default_coins": ["BTCUSDT", "ETHUSDT"],
  "coin_pool_api_url": "",
  "oi_top_api_url": "",
  "api_server_port": 8080,
  "max_daily_loss": 5.0,
  "max_drawdown": 10.0,
  "stop_trading_minutes": 120,
  "jwt_secret": "YOUR_NEW_GENERATED_SECRET_HERE",
  "news": []
}
```

### 示例 2: 激進型配置 (經驗用戶)

```json
{
  "admin_mode": true,
  "leverage": {
    "btc_eth_leverage": 10,
    "altcoin_leverage": 5
  },
  "use_default_coins": true,
  "default_coins": [
    "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT", 
    "XRPUSDT", "DOGEUSDT", "ADAUSDT", "HYPEUSDT"
  ],
  "coin_pool_api_url": "",
  "oi_top_api_url": "",
  "api_server_port": 8080,
  "max_daily_loss": 15.0,
  "max_drawdown": 30.0,
  "stop_trading_minutes": 30,
  "jwt_secret": "YOUR_NEW_GENERATED_SECRET_HERE",
  "news": []
}
```

---

## 常見問題

**Q: 為什麼 JSON 文件不能有註釋？**
A: 標準 JSON 格式不支持 `//` 或 `/* */` 註釋。如果需要註釋，請參閱本文檔。

**Q: 如何驗證 JSON 格式正確？**
A: 使用以下命令:
```bash
python3 -c "import json; json.load(open('config.json')); print('✅ 格式正確')"
```

**Q: 如果我只想交易 BTC 怎麼辦？**
A: 修改 `default_coins` 為:
```json
"default_coins": ["BTCUSDT"]
```

---

## 相關文檔

- [README.md](README.md) - 完整使用說明
- [CONFIG_SECURITY_GUIDE.md](CONFIG_SECURITY_GUIDE.md) - 安全配置指南
- [ENCRYPTION_DEPLOYMENT.md](docs/ENCRYPTION_DEPLOYMENT.md) - 加密部署

---

**記住**: 配置文件包含敏感信息，請勿提交到 Git！將 `config.json` 加入 `.gitignore`。
