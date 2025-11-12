package trader

import (
	"testing"
)

// TestTrySmartFallback 测试 Smart Fallback 机制
func TestTrySmartFallback(t *testing.T) {
	tests := []struct {
		name              string
		originalSize      float64
		originalLeverage  int
		availableBalance  float64
		minLeverage       int
		takerFeeRate      float64
		expectSuccess     bool
		expectedSize      float64
		expectedLeverage  int
		expectedAdjustLen int // 预期调整记录数量
	}{
		{
			name:              "微调持仓98%成功",
			originalSize:      100.0,
			originalLeverage:  5,
			availableBalance:  20.0, // 原始需要 20.04，98% 需要 19.64
			minLeverage:       1,
			takerFeeRate:      0.0004,
			expectSuccess:     true,
			expectedSize:      98.0,
			expectedLeverage:  5,
			expectedAdjustLen: 1,
		},
		{
			name:              "微调持仓95%成功",
			originalSize:      100.0,
			originalLeverage:  5,
			availableBalance:  19.2, // 原始需要 20.04，95% 需要 19.04
			minLeverage:       1,
			takerFeeRate:      0.0004,
			expectSuccess:     true,
			expectedSize:      95.0,
			expectedLeverage:  5,
			expectedAdjustLen: 1,
		},
		{
			name:              "微调持仓90%成功",
			originalSize:      100.0,
			originalLeverage:  5,
			availableBalance:  18.2, // 原始需要 20.04，90% 需要 18.04
			minLeverage:       1,
			takerFeeRate:      0.0004,
			expectSuccess:     true,
			expectedSize:      90.0,
			expectedLeverage:  5,
			expectedAdjustLen: 1,
		},
		{
			name:              "所有方法都失败",
			originalSize:      100.0,
			originalLeverage:  5,
			availableBalance:  10.0, // 远不够
			minLeverage:       1,
			takerFeeRate:      0.0004,
			expectSuccess:     false,
			expectedSize:      100.0,
			expectedLeverage:  5,
			expectedAdjustLen: 0,
		},
		{
			name:              "边界情况：用户报告的99.67_USDT案例",
			originalSize:      498.0, // 5x BTC at ~100k = ~500 USDT
			originalLeverage:  5,
			availableBalance:  99.67, // 原始需要 99.80，差 0.13
			minLeverage:       1,
			takerFeeRate:      0.0004,
			expectSuccess:     true,
			expectedSize:      498.0 * 0.98, // 98% 应该成功
			expectedLeverage:  5,
			expectedAdjustLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试用的 AutoTrader
			at := &AutoTrader{
				config: AutoTraderConfig{
					MinLeverage:  tt.minLeverage,
					TakerFeeRate: tt.takerFeeRate,
				},
			}

			// 调用 trySmartFallback
			adjustedSize, adjustedLev, success, adjustments := at.trySmartFallback(
				tt.originalSize,
				tt.originalLeverage,
				tt.availableBalance,
				"BTCUSDT",
				100000.0, // BTC 价格
			)

			// 验证成功状态
			if success != tt.expectSuccess {
				t.Errorf("success = %v, want %v", success, tt.expectSuccess)
			}

			// 如果预期成功，验证调整结果
			if tt.expectSuccess {
				const epsilon = 0.01 // 允许小数点后2位的误差
				sizeDiff := adjustedSize - tt.expectedSize
				if sizeDiff < -epsilon || sizeDiff > epsilon {
					t.Errorf("adjustedSize = %.2f, want %.2f (diff: %.4f)", adjustedSize, tt.expectedSize, sizeDiff)
				}
				if adjustedLev != tt.expectedLeverage {
					t.Errorf("adjustedLeverage = %d, want %d", adjustedLev, tt.expectedLeverage)
				}

				// 验证最终的保证金计算
				requiredMargin := adjustedSize / float64(adjustedLev)
				estimatedFee := adjustedSize * tt.takerFeeRate
				totalRequired := requiredMargin + estimatedFee

				if totalRequired > tt.availableBalance {
					t.Errorf("最终仍然保证金不足: 需要 %.2f, 可用 %.2f",
						totalRequired, tt.availableBalance)
				}

				// 验证调整记录数量
				if len(adjustments) != tt.expectedAdjustLen {
					t.Errorf("adjustments 数量 = %d, want %d (记录: %v)",
						len(adjustments), tt.expectedAdjustLen, adjustments)
				}

				t.Logf("✓ %s: 调整成功", tt.name)
				for i, adj := range adjustments {
					t.Logf("  %d. %s", i+1, adj)
				}
				t.Logf("  最终: 持仓 %.2f USDT, 杠杆 %dx, 需要 %.2f USDT",
					adjustedSize, adjustedLev, totalRequired)
			} else {
				t.Logf("✓ %s: 按预期失败", tt.name)
			}
		})
	}
}

// TestSmartFallbackEdgeCases 测试边界情况
func TestSmartFallbackEdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		originalSize     float64
		originalLeverage int
		availableBalance float64
		minLeverage      int
		takerFeeRate     float64
		expectSuccess    bool
		description      string
	}{
		{
			name:             "零可用余额",
			originalSize:     100.0,
			originalLeverage: 5,
			availableBalance: 0.0,
			minLeverage:      1,
			takerFeeRate:     0.0004,
			expectSuccess:    false,
			description:      "完全没有可用余额",
		},
		{
			name:             "极小持仓1_USDT",
			originalSize:     1.0,
			originalLeverage: 5,
			availableBalance: 0.19, // 需要 0.2004
			minLeverage:      1,
			takerFeeRate:     0.0004,
			expectSuccess:    true,
			description:      "极小持仓也能调整",
		},
		{
			name:             "极高杠杆100x",
			originalSize:     100.0,
			originalLeverage: 100,
			availableBalance: 1.0, // 100x 只需要 1.04
			minLeverage:      1,
			takerFeeRate:     0.0004,
			expectSuccess:    true,
			description:      "极高杠杆不需要调整",
		},
		{
			name:             "1x杠杆无法再降",
			originalSize:     100.0,
			originalLeverage: 1,
			availableBalance: 90.0, // 1x 需要 100.04
			minLeverage:      1,
			takerFeeRate:     0.0004,
			expectSuccess:    true, // 可以微调持仓
			description:      "1x 杠杆只能微调持仓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			at := &AutoTrader{
				config: AutoTraderConfig{
					MinLeverage:  tt.minLeverage,
					TakerFeeRate: tt.takerFeeRate,
				},
			}

			adjustedSize, adjustedLev, success, adjustments := at.trySmartFallback(
				tt.originalSize,
				tt.originalLeverage,
				tt.availableBalance,
				"BTCUSDT",
				100000.0,
			)

			if success != tt.expectSuccess {
				t.Errorf("%s: success = %v, want %v", tt.description, success, tt.expectSuccess)
			}

			if success {
				requiredMargin := adjustedSize / float64(adjustedLev)
				estimatedFee := adjustedSize * tt.takerFeeRate
				totalRequired := requiredMargin + estimatedFee

				if totalRequired > tt.availableBalance {
					t.Errorf("%s: 最终保证金不足 (需要 %.4f > 可用 %.4f)",
						tt.description, totalRequired, tt.availableBalance)
				}

				t.Logf("✓ %s: 成功 (调整: %v)", tt.description, adjustments)
			} else {
				t.Logf("✓ %s: 按预期失败", tt.description)
			}
		})
	}
}
