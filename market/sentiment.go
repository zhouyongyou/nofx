package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ========== Binance 多空比數據（完全免費）==========

// FetchLongShortRatio 獲取 Binance 多空持倉人數比
// API 文檔：https://binance-docs.github.io/apidocs/futures/en/#long-short-ratio
func FetchLongShortRatio(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/futures/data/globalLongShortAccountRatio?symbol=%s&period=5m&limit=1", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch long/short ratio: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var data []struct {
		Symbol         string `json:"symbol"`
		LongShortRatio string `json:"longShortRatio"`
		LongAccount    string `json:"longAccount"`
		ShortAccount   string `json:"shortAccount"`
		Timestamp      int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, fmt.Errorf("no data returned for symbol %s", symbol)
	}

	var ratio float64
	fmt.Sscanf(data[0].LongShortRatio, "%f", &ratio)
	return ratio, nil
}

// FetchTopTraderLongShortRatio 獲取大戶多空持倉量比
func FetchTopTraderLongShortRatio(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/futures/data/topLongShortPositionRatio?symbol=%s&period=5m&limit=1", symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch top trader ratio: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var data []struct {
		Symbol         string `json:"symbol"`
		LongShortRatio string `json:"longShortRatio"`
		LongAccount    string `json:"longAccount"`
		ShortAccount   string `json:"shortAccount"`
		Timestamp      int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, fmt.Errorf("no data returned for symbol %s", symbol)
	}

	var ratio float64
	fmt.Sscanf(data[0].LongShortRatio, "%f", &ratio)
	return ratio, nil
}

// AnalyzeSentiment 分析市場情緒（基於多空比）
func AnalyzeSentiment(longShortRatio, topTraderRatio float64) string {
	// 綜合判斷：全市場 + 大戶
	avgRatio := (longShortRatio + topTraderRatio) / 2

	if avgRatio > 1.5 {
		return "bullish" // 多頭極度占優
	} else if avgRatio > 1.1 {
		return "bullish" // 多頭占優
	} else if avgRatio > 0.9 {
		return "neutral" // 中性
	} else if avgRatio > 0.7 {
		return "bearish" // 空頭占優
	}
	return "bearish" // 空頭極度占優
}

// ========== VIX 恐慌指數（Yahoo Finance - 免費）==========

// FetchVIX 獲取 VIX 恐慌指數
// 使用 Yahoo Finance API（免費，但有限流）
func FetchVIX() (float64, error) {
	// Yahoo Finance API（非官方但穩定）
	url := "https://query1.finance.yahoo.com/v8/finance/chart/%5EVIX?interval=1m&range=1d"

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch VIX: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var data struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return 0, err
	}

	if len(data.Chart.Result) == 0 {
		return 0, fmt.Errorf("no VIX data returned")
	}

	return data.Chart.Result[0].Meta.RegularMarketPrice, nil
}

// AnalyzeVIX 分析 VIX 指數並給出建議
func AnalyzeVIX(vix float64) (fearLevel, recommendation string) {
	switch {
	case vix < 15:
		return "low", "normal"
	case vix < 20:
		return "moderate", "cautious"
	case vix < 30:
		return "high", "defensive"
	default:
		return "extreme", "avoid_new_positions"
	}
}

// ========== S&P 500 狀態（Alpha Vantage - 免費）==========

// FetchSPXStatus 獲取 S&P 500 狀態
// 注意：需要 Alpha Vantage API Key（免費：500 calls/day）
func FetchSPXStatus(apiKey string) (*USMarketStatus, error) {
	// 檢查美股交易時段（美東時間 9:30-16:00）
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Now().In(loc)
	hour := now.Hour()
	minute := now.Minute()

	isOpen := false
	if hour > 9 || (hour == 9 && minute >= 30) {
		if hour < 16 {
			isOpen = true
		}
	}

	// 如果市場休市，返回簡單狀態
	if !isOpen {
		return &USMarketStatus{
			IsOpen:      false,
			SPXTrend:    "neutral",
			SPXChange1h: 0,
			Warning:     "",
		}, nil
	}

	// 獲取 S&P 500 數據（使用 Alpha Vantage 免費 API）
	url := fmt.Sprintf("https://www.alphavantage.co/query?function=GLOBAL_QUOTE&symbol=SPY&apikey=%s", apiKey)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch SPX: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data struct {
		GlobalQuote struct {
			Price         string `json:"05. price"`
			Change        string `json:"09. change"`
			ChangePercent string `json:"10. change percent"`
		} `json:"Global Quote"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	var changePercent float64
	fmt.Sscanf(data.GlobalQuote.ChangePercent, "%f%%", &changePercent)

	// 判斷趨勢
	trend := "neutral"
	warning := ""

	if changePercent > 0.5 {
		trend = "up"
	} else if changePercent < -0.5 {
		trend = "down"
	}

	if changePercent < -2.0 {
		warning = fmt.Sprintf("⚠️ S&P 500 大跌 %.2f%%，市場風險偏好下降", changePercent)
	} else if changePercent > 2.0 {
		warning = fmt.Sprintf("🔥 S&P 500 大漲 %.2f%%，市場風險偏好上升", changePercent)
	}

	return &USMarketStatus{
		IsOpen:      true,
		SPXTrend:    trend,
		SPXChange1h: changePercent,
		Warning:     warning,
	}, nil
}

// ========== 整合函數 ==========

// FetchMarketSentiment 獲取完整的市場情緒數據（免費版本）
// alphaVantageKey: 可選，用於獲取美股數據（免費 500 calls/day）
func FetchMarketSentiment(alphaVantageKey string) (*MarketSentiment, error) {
	sentiment := &MarketSentiment{
		UpdatedAt: time.Now(),
	}

	// 1. 獲取 VIX（免費）
	vix, err := FetchVIX()
	if err == nil {
		sentiment.VIX = vix
		sentiment.FearLevel, sentiment.Recommendation = AnalyzeVIX(vix)
	}

	// 2. 獲取美股狀態（可選，需要 API Key）
	if alphaVantageKey != "" {
		usMarket, err := FetchSPXStatus(alphaVantageKey)
		if err == nil {
			sentiment.USMarket = usMarket
		}
	}

	return sentiment, nil
}

// EnhanceOIData 增強 OI 數據（加入多空比）
// 這個函數會被 market.Get() 調用來增強現有的 OI 數據
func EnhanceOIData(symbol string, oi *OIData) error {
	if oi == nil {
		return fmt.Errorf("OI data is nil")
	}

	// 獲取多空比（完全免費）
	longShortRatio, err := FetchLongShortRatio(symbol)
	if err == nil {
		oi.LongShortRatio = longShortRatio
	}

	// 獲取大戶多空比（完全免費）
	topTraderRatio, err := FetchTopTraderLongShortRatio(symbol)
	if err == nil {
		oi.TopTraderLongShortRatio = topTraderRatio
	}

	// 分析市場情緒
	if oi.LongShortRatio > 0 && oi.TopTraderLongShortRatio > 0 {
		oi.Sentiment = AnalyzeSentiment(oi.LongShortRatio, oi.TopTraderLongShortRatio)
	}

	return nil
}
