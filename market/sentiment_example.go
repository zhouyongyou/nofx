package market

import (
	"fmt"
	"log"
)

// ExampleUsage 展示如何使用免費市場情緒 API
func ExampleUsage() {
	fmt.Println("========== 免費市場情緒數據使用示例 ==========")

	// ========== 1. 獲取 Binance 多空比（完全免費）==========
	fmt.Println("📊 Binance 多空比數據：")

	symbol := "BTCUSDT"

	// 全市場多空持倉人數比
	longShortRatio, err := FetchLongShortRatio(symbol)
	if err != nil {
		log.Printf("❌ 獲取多空比失敗: %v", err)
	} else {
		fmt.Printf("  • 全市場多空比：%.2f\n", longShortRatio)
		if longShortRatio > 1 {
			fmt.Printf("    → 多頭占優 (%.1f%% 做多 vs %.1f%% 做空)\n",
				longShortRatio/(1+longShortRatio)*100,
				1/(1+longShortRatio)*100)
		} else {
			fmt.Printf("    → 空頭占優 (%.1f%% 做空 vs %.1f%% 做多)\n",
				1/(1+longShortRatio)*100,
				longShortRatio/(1+longShortRatio)*100)
		}
	}

	// 大戶多空持倉量比
	topTraderRatio, err := FetchTopTraderLongShortRatio(symbol)
	if err != nil {
		log.Printf("❌ 獲取大戶多空比失敗: %v", err)
	} else {
		fmt.Printf("  • 大戶多空比：%.2f\n", topTraderRatio)
		if topTraderRatio > 1 {
			fmt.Println("    → 大戶偏多（通常是好信號）")
		} else {
			fmt.Println("    → 大戶偏空（需謹慎）")
		}
	}

	// 綜合分析
	if longShortRatio > 0 && topTraderRatio > 0 {
		sentiment := AnalyzeSentiment(longShortRatio, topTraderRatio)
		fmt.Printf("  • 市場情緒：%s\n", sentiment)
	}

	fmt.Println()

	// ========== 2. 獲取 VIX 恐慌指數（免費）==========
	fmt.Println("😱 VIX 恐慌指數：")

	vix, err := FetchVIX()
	if err != nil {
		log.Printf("❌ 獲取 VIX 失敗: %v", err)
	} else {
		fearLevel, recommendation := AnalyzeVIX(vix)
		fmt.Printf("  • VIX 值：%.2f\n", vix)
		fmt.Printf("  • 恐慌等級：%s\n", fearLevel)
		fmt.Printf("  • 建議：%s\n", recommendation)

		// 具體建議
		switch recommendation {
		case "normal":
			fmt.Println("    → 市場平穩，正常交易")
		case "cautious":
			fmt.Println("    → 市場輕度恐慌，謹慎交易，降低槓桿")
		case "defensive":
			fmt.Println("    → 市場恐慌，防禦性交易，收緊止損")
		case "avoid_new_positions":
			fmt.Println("    → 極度恐慌，避免新開倉，保護已有倉位")
		}
	}

	fmt.Println()

	// ========== 3. 獲取美股狀態（需要免費 API Key）==========
	fmt.Println("🇺🇸 美股狀態（可選）：")

	// Alpha Vantage 免費 API Key（500 calls/day）
	// 註冊：https://www.alphavantage.co/support/#api-key
	alphaVantageKey := "YOUR_FREE_API_KEY" // 替換為你的免費 API Key

	if alphaVantageKey == "YOUR_FREE_API_KEY" {
		fmt.Println("  ℹ️  未設置 Alpha Vantage API Key，跳過美股數據")
		fmt.Println("  💡 免費註冊：https://www.alphavantage.co/support/#api-key")
	} else {
		usMarket, err := FetchSPXStatus(alphaVantageKey)
		if err != nil {
			log.Printf("❌ 獲取美股狀態失敗: %v", err)
		} else {
			if usMarket.IsOpen {
				fmt.Printf("  • 美股狀態：開盤中\n")
				fmt.Printf("  • S&P 500 趨勢：%s\n", usMarket.SPXTrend)
				fmt.Printf("  • 1 小時變化：%.2f%%\n", usMarket.SPXChange1h)
				if usMarket.Warning != "" {
					fmt.Printf("  • 警告：%s\n", usMarket.Warning)
				}
			} else {
				fmt.Println("  • 美股狀態：休市")
			}
		}
	}

	fmt.Println()

	// ========== 4. 整合使用 ==========
	fmt.Println("🎯 整合使用示例（AI 決策前調用）：")

	sentiment, err := FetchMarketSentiment(alphaVantageKey)
	if err != nil {
		log.Printf("❌ 獲取市場情緒失敗: %v", err)
	} else {
		fmt.Printf("  • VIX：%.2f (%s)\n", sentiment.VIX, sentiment.FearLevel)
		fmt.Printf("  • 建議：%s\n", sentiment.Recommendation)

		if sentiment.USMarket != nil && sentiment.USMarket.IsOpen {
			fmt.Printf("  • 美股：%s (%.2f%%)\n",
				sentiment.USMarket.SPXTrend,
				sentiment.USMarket.SPXChange1h)
		}
	}

	fmt.Println("\n========== 成本分析 ==========")
	fmt.Println("✅ Binance 多空比：完全免費，無限制")
	fmt.Println("✅ VIX 恐慌指數：完全免費，Yahoo Finance")
	fmt.Println("⚠️  S&P 500 狀態：免費但有限流（500 calls/day）")
	fmt.Println("\n每次 AI 決策調用成本：$0.00")
	fmt.Println("每月總成本：$0.00（完全免費）")
}

// ========== AI Prompt 整合範例 ==========

// BuildAIPromptSentiment 構建給 AI 的市場情緒描述（簡潔版）
func BuildAIPromptSentiment(symbol string, alphaVantageKey string) string {
	var prompt string

	// 1. 多空比數據
	longShortRatio, err := FetchLongShortRatio(symbol)
	topTraderRatio, _ := FetchTopTraderLongShortRatio(symbol)

	if err == nil && longShortRatio > 0 {
		sentiment := AnalyzeSentiment(longShortRatio, topTraderRatio)
		prompt += fmt.Sprintf("市場情緒：%s（多空比 %.2f，大戶 %.2f）\n",
			sentiment, longShortRatio, topTraderRatio)

		if sentiment == "bullish" {
			prompt += "→ 市場偏多，可考慮做多機會\n"
		} else if sentiment == "bearish" {
			prompt += "→ 市場偏空，需謹慎做多\n"
		}
	}

	// 2. VIX 恐慌指數
	vix, err := FetchVIX()
	if err == nil {
		fearLevel, recommendation := AnalyzeVIX(vix)
		prompt += fmt.Sprintf("VIX 恐慌指數：%.1f（%s）\n", vix, fearLevel)

		switch recommendation {
		case "cautious":
			prompt += "→ 市場輕度恐慌，建議降低槓桿至 5-10x\n"
		case "defensive":
			prompt += "→ 市場恐慌，建議收緊止損，避免激進操作\n"
		case "avoid_new_positions":
			prompt += "→ 極度恐慌，強烈建議觀望，不要新開倉\n"
		}
	}

	// 3. 美股狀態（可選）
	if alphaVantageKey != "" {
		usMarket, err := FetchSPXStatus(alphaVantageKey)
		if err == nil && usMarket.IsOpen {
			prompt += fmt.Sprintf("美股狀態：%s (%.2f%%)\n",
				usMarket.SPXTrend, usMarket.SPXChange1h)

			if usMarket.Warning != "" {
				prompt += usMarket.Warning + "\n"
			}
		}
	}

	if prompt == "" {
		return "市場情緒數據暫時不可用\n"
	}

	return prompt
}
