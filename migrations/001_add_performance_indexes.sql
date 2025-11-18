-- ============================================================
-- nofx 性能优化索引迁移
-- Version: 1.0.0
-- Date: 2025-11-14
-- Description: 添加性能索引以优化 API 查询
-- ============================================================

-- 开启 WAL 模式检查 (确保已启用)
PRAGMA journal_mode;
-- 预期输出: wal

-- 开始事务
BEGIN TRANSACTION;

-- ============================================================
-- Part 1: AI Models 表索引
-- ============================================================

-- 索引 1: 加速 (user_id, model_id) 复合查询
-- 使用场景: GetAIModels() 内部查找特定 model_id
CREATE INDEX IF NOT EXISTS idx_ai_models_user_model
ON ai_models(user_id, model_id);

-- 索引 2: 加速 user_id 单字段查询
-- 使用场景: GetAIModels(userID) 主查询
CREATE INDEX IF NOT EXISTS idx_ai_models_user
ON ai_models(user_id)
WHERE user_id != 'default'; -- 排除默认配置,只索引用户数据

-- 索引 3: 加速 enabled 字段过滤
-- 使用场景: 查询用户启用的模型
CREATE INDEX IF NOT EXISTS idx_ai_models_enabled
ON ai_models(user_id, enabled)
WHERE enabled = 1; -- 部分索引,仅索引已启用的

-- ============================================================
-- Part 2: Exchanges 表索引
-- ============================================================

-- 索引 4: 加速 (user_id, id) 复合查询
-- 注意: exchanges 表的主键是 (id, user_id)
CREATE INDEX IF NOT EXISTS idx_exchanges_user_id
ON exchanges(user_id, id);

-- 索引 5: 加速 user_id 单字段查询
CREATE INDEX IF NOT EXISTS idx_exchanges_user
ON exchanges(user_id)
WHERE user_id != 'default';

-- 索引 6: 加速 enabled 字段过滤
CREATE INDEX IF NOT EXISTS idx_exchanges_enabled
ON exchanges(user_id, enabled)
WHERE enabled = 1;

-- ============================================================
-- Part 3: Traders 表索引
-- ============================================================

-- 索引 7: 加速 user_id 查询 (GetTraders 主查询)
CREATE INDEX IF NOT EXISTS idx_traders_user
ON traders(user_id);

-- 索引 8: 加速运行中 Trader 查询
-- 使用场景: GetAllTimeframes() 查询所有运行中的 Trader
CREATE INDEX IF NOT EXISTS idx_traders_running
ON traders(is_running)
WHERE is_running = 1;

-- 索引 9: 复合索引 - 查询用户的运行中 Trader
CREATE INDEX IF NOT EXISTS idx_traders_user_running
ON traders(user_id, is_running);

-- 索引 10: 外键优化 - ai_model_id
-- 使用场景: JOIN 查询 Trader 关联的 AI 模型
CREATE INDEX IF NOT EXISTS idx_traders_ai_model
ON traders(ai_model_id);

-- 索引 11: 外键优化 - exchange_id
-- 使用场景: JOIN 查询 Trader 关联的交易所
CREATE INDEX IF NOT EXISTS idx_traders_exchange
ON traders(exchange_id);

-- ============================================================
-- Part 4: Users 表 (已有 UNIQUE email,无需额外索引)
-- ============================================================

-- ============================================================
-- Part 5: 验证索引创建
-- ============================================================

-- 提交事务
COMMIT;

-- 列出所有索引
SELECT
    name,
    tbl_name,
    sql
FROM sqlite_master
WHERE type = 'index'
  AND name LIKE 'idx_%'
ORDER BY tbl_name, name;

-- 分析表以更新统计信息 (优化查询计划)
ANALYZE ai_models;
ANALYZE exchanges;
ANALYZE traders;
ANALYZE users;

-- 输出摘要
SELECT '✅ Performance indexes created successfully' AS status;
