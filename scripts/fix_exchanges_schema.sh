#!/bin/bash

set -e

DB_FILE="config.db"

echo "🔍 開始修復 exchanges 表結構..."
echo ""

# 備份數據庫
BACKUP_FILE="${DB_FILE}.backup_$(date +%Y%m%d_%H%M%S)"
echo "💾 備份數據庫到 $BACKUP_FILE ..."
cp "$DB_FILE" "$BACKUP_FILE"
echo "✅ 備份完成"
echo ""

# 檢查當前表結構
echo "🔍 當前 exchanges 表結構："
sqlite3 "$DB_FILE" "PRAGMA table_info(exchanges);" | head -5
echo ""

# 檢查是否已經有 exchange_id 列
HAS_EXCHANGE_ID=$(sqlite3 "$DB_FILE" "PRAGMA table_info(exchanges);" | grep -c "exchange_id" || true)

if [ "$HAS_EXCHANGE_ID" -gt 0 ]; then
    echo "✅ exchanges 表已經有 exchange_id 列，無需修復"
    exit 0
fi

echo "⚠️  exchanges 表缺少 exchange_id 列，開始遷移..."
echo ""

# 執行遷移
sqlite3 "$DB_FILE" <<'EOF'
BEGIN TRANSACTION;

-- 1. 創建新表（有 exchange_id 列）
CREATE TABLE exchanges_fixed (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    exchange_id TEXT NOT NULL,
    user_id TEXT NOT NULL DEFAULT 'default',
    display_name TEXT DEFAULT '',
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    enabled BOOLEAN DEFAULT 0,
    api_key TEXT DEFAULT '',
    secret_key TEXT DEFAULT '',
    testnet BOOLEAN DEFAULT 0,
    hyperliquid_wallet_addr TEXT DEFAULT '',
    aster_user TEXT DEFAULT '',
    aster_signer TEXT DEFAULT '',
    aster_private_key TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- 2. 遷移數據（舊表的 id 列 -> 新表的 exchange_id 列）
INSERT INTO exchanges_fixed (
    exchange_id, user_id, name, type, enabled, api_key, secret_key, testnet,
    hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
    created_at, updated_at
)
SELECT
    id as exchange_id,  -- 舊表的 id 就是 exchange_id（如 "binance"）
    user_id, name, type, enabled, api_key, secret_key, testnet,
    hyperliquid_wallet_addr, aster_user, aster_signer, aster_private_key,
    created_at, updated_at
FROM exchanges;

-- 3. 刪除舊表
DROP TABLE exchanges;

-- 4. 重命名新表
ALTER TABLE exchanges_fixed RENAME TO exchanges;

COMMIT;
EOF

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ 遷移成功！"
    echo ""
    echo "🔍 新的 exchanges 表結構："
    sqlite3 "$DB_FILE" "PRAGMA table_info(exchanges);"
    echo ""
    echo "📊 數據統計："
    echo "  - 交易所數量: $(sqlite3 "$DB_FILE" "SELECT COUNT(*) FROM exchanges;")"
    sqlite3 "$DB_FILE" "SELECT '  - ID=' || id || ', exchange_id=' || exchange_id || ', name=' || name FROM exchanges;" 2>/dev/null || true
else
    echo ""
    echo "❌ 遷移失敗！正在恢復備份..."
    cp "$BACKUP_FILE" "$DB_FILE"
    echo "✅ 已恢復備份"
    exit 1
fi

echo ""
echo "✅ 修復完成！"
