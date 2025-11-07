# 上游 (NoFxAiOS/nofx) 需要修复的问题报告

**生成时间**: 2025-11-07
**分析分支**: nofxaios/dev
**报告人**: z-dev 分支测试团队

---

## 📋 执行摘要

在 z-dev 分支测试中发现**两个严重的 prompt 计算错误**，经检查上游 `nofxaios/dev` 分支也存在相同问题：

1. **保证金使用率过高** - 导致 Hyperliquid 交易所报错 "Insufficient margin"
2. **risk_usd 计算公式错误** - 重复计算杠杆，导致风险评估错误（Issue #592）

两个问题都会导致交易失败或风控失效，**建议立即修复**。

---

## 🐛 问题 1：保证金使用率过高导致开仓失败

### 问题描述

**受影响文件**（上游）：
- `prompts/adaptive.txt`
- `prompts/default.txt`
- `prompts/Hansen.txt`
- `prompts/nof1.txt`
- `prompts/adaptive_relaxed.txt`（如果存在）

**当前错误配置**：
```
可用保证金 = Available Cash × 0.95（预留5%）
```

### 问题表现

**真实案例**（用户账户 $98.89）：

```
AI 计算：
- 可用保证金 = $98.89 × 0.95 = $93.95
- position_size_usd = $93.95 × 5 = $469.75
- 实际占用 = $93.95 + 手续费 $0.19 = $94.14
- 剩余保证金 = $98.89 - $94.14 = $4.75  ❌

Hyperliquid 报错：
"Insufficient margin to place order. asset=1"
```

**错误截图**：
```
仓位计算：可用保证金$98.89 × 0.95 = $93.95，名义价值$93.95 × 5 = $469.75
ETHUSDT open_long 5x @3296.7700 X
开多仓失败：Insufficient margin to place order. asset=1
净值：98.89 USDT 可用：98.89 USDT
```

### 根本原因

Hyperliquid 要求账户保留**清算保证金缓冲**，5% 的预留不足以满足：

1. **手续费**：~0.04%（Taker）
2. **滑点**：0.01-0.1%
3. **清算保证金**：交易所要求的最小剩余余额（推测 ≥10% 或 ≥$10）

### 建议修复方案

**方案 A**：降低保证金使用率至 88%（推荐）

```diff
- 1. **可用保证金** = Available Cash × 0.95（预留5%给手续费与滑点缓冲）
+ 1. **可用保证金** = Available Cash × 0.88（预留12%给手续费、滑点与清算保证金缓冲）

- - 可用保证金 = $500 × 0.95 = $475
- - position_size_usd = $475 × 5 = $2,375
+ - 可用保证金 = $500 × 0.88 = $440
+ - position_size_usd = $440 × 5 = $2,200
```

**方案 B**：在代码层添加最小剩余余额检查

```go
// trader/auto_trader.go
minimumReserve := math.Max(10.0, availableBalance * 0.10)
if availableBalance - totalRequired < minimumReserve {
    return fmt.Errorf("保证金不足：需保留至少 $%.2f", minimumReserve)
}
```

**推荐**：同时采用方案 A + 方案 B（双重保护）

### 影响评估

**严重程度**: 🔴 **高** - 导致小账户（<$200）完全无法开仓

**影响范围**:
- 所有使用 Hyperliquid 的用户
- 小账户用户无法正常交易
- 可能导致用户流失

**修复后效果**：
```
$98.89 账户修复后：
- 可用保证金 = $98.89 × 0.88 = $87.02
- position_size_usd = $87.02 × 5 = $435.10
- 剩余 = $11.87  ✅ 充足
```

---

## 🐛 问题 2：risk_usd 计算公式错误（Issue #592）

### 问题描述

**相关 Issue**: #592 "[BUG] Adaptive 提示词中公式错误问题"
**提出者**: @Sierccccc

**受影响文件**（上游）：
- `prompts/adaptive.txt` (line 404)
- `prompts/nof1.txt` (line 104)

**当前错误公式**：
```python
# adaptive.txt
risk_usd = |入场价 - 止损价| × 仓位数量 × 杠杆  ❌

# nof1.txt
risk_usd = |Entry Price - Stop Loss| × Position Size × Leverage  ❌
```

### 问题分析

#### 为什么是错的？

**仓位数量已经包含了杠杆效应**：

```
仓位数量 = position_size_usd / 价格
position_size_usd = 保证金 × 杠杆

因此：
仓位数量 = (保证金 × 杠杆) / 价格
```

**再乘一次杠杆 = 重复计算，风险被错误放大「杠杆」倍**

#### 实际案例对比

**场景**：
- 保证金: $100
- 杠杆: 10x
- position_size_usd: $1,000
- 价格: $50,000/BTC
- 仓位数量: 0.02 BTC
- 止损距离: $500/BTC

**正确计算**：
```
risk_usd = $500 × 0.02 = $10 ✅
风险占保证金比例 = $10 / $100 = 10%（合理）
```

**错误计算（当前公式）**：
```
risk_usd = $500 × 0.02 × 10 = $100 ❌
风险占保证金比例 = $100 / $100 = 100%（完全错误）
```

### 问题影响

**严重程度**: 🟡 **中** - 不会导致交易失败，但风控逻辑错误

**可能导致**：

1. **AI 误判风险过大** → 拒绝开仓（错失机会）
2. **风控验证失败** → 风险显示为实际的「杠杆」倍
3. **仓位过小** → AI 为了满足错误的风控要求，开出过小仓位
4. **用户困惑** → risk_usd 显示值与实际风险不符

**实际案例**（来自 Issue #592）：

> "好的，非常感谢给我这个机会重新审视。这是一个极其关键的点，我们必须确保万无一失。
>
> 我已经**重新、并且更加深入地**理解了你在 Prompt 中定义的 `risk_usd` 和其他相关变量。我的结论保持不变，并且我能更清晰地解释为什么这个公式存在风险。"

### 建议修复方案

#### 修改 1：prompts/adaptive.txt (line 404)

```diff
5. **risk_usd** (风险金额)
-   - 计算公式: |入场价 - 止损价| × 仓位数量 × 杠杆
+   - 计算公式: |入场价 - 止损价| × 仓位数量
+   - ⚠️ **不要再乘杠杆**：仓位数量 = position_size_usd / 价格，已包含杠杆效应
   - 必须 ≤ 账户净值 × 风险预算（1.5-2.5%）
```

#### 修改 2：prompts/nof1.txt (line 104)

```diff
4. **risk_usd** (float): Dollar amount at risk (distance from entry to stop loss)
-   - Calculate as: |Entry Price - Stop Loss| × Position Size × Leverage
+   - Calculate as: |Entry Price - Stop Loss| × Position Size (in coins)
+   - ⚠️ **Do NOT multiply by leverage**: Position Size already includes leverage effect
```

### 验证方法

修复后，使用上面的案例验证：

```python
# 测试数据
margin = 100
leverage = 10
position_size_usd = margin * leverage  # 1000
price = 50000
position_size_coins = position_size_usd / price  # 0.02
stop_distance = 500

# 修复后的计算
risk_usd = stop_distance * position_size_coins
print(f"risk_usd = {risk_usd}")  # 应输出: 10.0
print(f"risk% = {risk_usd / margin * 100}%")  # 应输出: 10.0%
```

---

## 📊 修复优先级矩阵

| 问题 | 严重程度 | 影响范围 | 优先级 | 预计工作量 |
|------|---------|---------|--------|-----------|
| **保证金使用率** | 🔴 高 | 所有 Hyperliquid 用户 | **P0** | 15 分钟 |
| **risk_usd 公式** | 🟡 中 | 风控逻辑 | **P1** | 10 分钟 |

---

## 🔄 推荐修复流程

### Step 1: 修复保证金使用率（P0）

1. 修改所有 prompt 文件中的 `0.95` → `0.88`
2. 更新示例计算（$500 和 $98.89 的案例）
3. 测试小账户开仓（< $200）

**受影响文件**：
- prompts/adaptive.txt
- prompts/default.txt
- prompts/Hansen.txt
- prompts/nof1.txt
- prompts/adaptive_relaxed.txt（如果存在）

### Step 2: 修复 risk_usd 公式（P1）

1. 修改 adaptive.txt line 404
2. 修改 nof1.txt line 104
3. 添加警告说明
4. Close Issue #592

**受影响文件**：
- prompts/adaptive.txt (1 处)
- prompts/nof1.txt (1 处)

### Step 3: 回归测试

1. **保证金测试**：
   - 测试账户：$98.89, $200, $500
   - 验证：所有账户都能成功开仓
   - 验证：剩余余额 ≥ $10 或 10%

2. **risk_usd 测试**：
   - 测试案例：保证金 $100, 杠杆 10x, 止损距离 $500
   - 验证：risk_usd ≈ $10（不是 $100）
   - 验证：风险占比 ≈ 10%（不是 100%）

---

## 📝 提交建议

### Commit 1: 保证金使用率修复

```bash
git commit -m "fix(prompts): reduce margin usage from 95% to 88% for Hyperliquid liquidation buffer

## Problem
Users with small accounts (<$200) encounter Hyperliquid error:
\"Insufficient margin to place order. asset=1\"

## Root Cause
5% reserve insufficient for:
- Trading fees (~0.04%)
- Slippage (0.01-0.1%)
- Liquidation margin buffer (Hyperliquid requirement)

## Solution
Reduce margin usage rate from 95% to 88% (reserve 12%)

Example ($98.89 account):
Before: $93.95 margin → $4.75 remaining ❌
After:  $87.02 margin → $11.87 remaining ✅

## Modified Files
- prompts/adaptive.txt
- prompts/default.txt
- prompts/Hansen.txt
- prompts/nof1.txt
- prompts/adaptive_relaxed.txt (if exists)

## Testing
Verified with $98.89 account - successful order placement"
```

### Commit 2: risk_usd 公式修复

```bash
git commit -m "fix(prompts): correct risk_usd formula - remove duplicate leverage multiplication

## Problem (Issue #592)
risk_usd formula incorrectly multiplies leverage twice:
Incorrect: risk_usd = |Entry - Stop| × Position Size × Leverage ❌

## Root Cause
Position Size already includes leverage effect:
- Position Size = position_size_usd / price
- position_size_usd = margin × leverage
- Therefore: Position Size = (margin × leverage) / price

Multiplying leverage again amplifies risk by \"leverage\" times.

## Example
Setup: $100 margin, 10x leverage, 0.02 BTC position, $500 stop distance

Correct:   risk_usd = $500 × 0.02 = $10 ✅ (10% of margin)
Incorrect: risk_usd = $500 × 0.02 × 10 = $100 ❌ (100% of margin - wrong!)

## Solution
Correct formula: risk_usd = |Entry - Stop| × Position Size

Added warnings:
- CN: ⚠️ 不要再乘杠杆：仓位数量已包含杠杆效应
- EN: ⚠️ Do NOT multiply by leverage: Position Size already includes leverage effect

## Modified Files
- prompts/adaptive.txt (line 404)
- prompts/nof1.txt (line 104)

Closes #592"
```

---

## ✅ 验收标准

### 保证金使用率修复

- [ ] 所有 prompt 文件使用 `0.88` 而非 `0.95`
- [ ] $98.89 账户可以成功开仓
- [ ] 剩余余额 ≥ $10 或 ≥ 账户余额的 10%
- [ ] 无 "Insufficient margin" 错误

### risk_usd 公式修复

- [ ] 所有 prompt 文件移除 `× 杠杆` / `× Leverage`
- [ ] 添加了警告说明
- [ ] risk_usd 计算值 = 实际风险（不是放大的值）
- [ ] Issue #592 已关闭

---

## 📚 参考资料

**相关 Issue**:
- #592 - [BUG] Adaptive 提示词中公式错误问题

**测试分支**:
- zhouyongyou/nofx z-dev 分支（已包含修复）

**修复 Commits** (z-dev):
- c5e21e3 - 保证金使用率修复
- e911962 - risk_usd 公式修复

---

**报告结束**
