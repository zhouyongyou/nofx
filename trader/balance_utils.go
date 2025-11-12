package trader

import "log"

// ParseTotalEquity 從交易所余額信息中提取總資產（total equity）
// 總資產 = 錢包餘額 + 未實現盈虧
// 這是計算交易員盈虧的正確基準，而不是可用餘額
func ParseTotalEquity(balanceInfo map[string]interface{}, logPrefix string) (totalEquity float64, success bool) {
	totalWalletBalance := 0.0
	totalUnrealizedProfit := 0.0

	// 提取錢包餘額
	if wallet, ok := balanceInfo["totalWalletBalance"].(float64); ok {
		totalWalletBalance = wallet
	}

	// 提取未實現盈虧
	if unrealized, ok := balanceInfo["totalUnrealizedProfit"].(float64); ok {
		totalUnrealizedProfit = unrealized
	}

	// 計算總資產
	totalEquity = totalWalletBalance + totalUnrealizedProfit

	if totalEquity > 0 {
		if logPrefix != "" {
			log.Printf("%s 查詢到交易所總資產: %.2f USDT (錢包: %.2f + 未實現: %.2f)",
				logPrefix, totalEquity, totalWalletBalance, totalUnrealizedProfit)
		}
		return totalEquity, true
	}

	// 嘗試 fallback 字段
	if availableBalance, ok := balanceInfo["availableBalance"].(float64); ok && availableBalance > 0 {
		if logPrefix != "" {
			log.Printf("⚠️ %s 無法提取 totalEquity，使用 availableBalance: %.2f", logPrefix, availableBalance)
		}
		return availableBalance, true
	}

	if balance, ok := balanceInfo["balance"].(float64); ok && balance > 0 {
		if logPrefix != "" {
			log.Printf("⚠️ %s 無法提取 totalEquity，使用 balance: %.2f", logPrefix, balance)
		}
		return balance, true
	}

	// 所有字段都失敗
	if logPrefix != "" {
		log.Printf("⚠️ %s 無法提取任何余額字段", logPrefix)
	}
	return 0, false
}
