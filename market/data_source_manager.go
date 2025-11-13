package market

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// DataSource 数据源接口
type DataSource interface {
	GetName() string                                              // 获取数据源名称
	GetKlines(symbol, interval string, limit int) ([]Kline, error) // 获取K线数据
	GetTicker(symbol string) (*Ticker, error)                     // 获取ticker数据
	HealthCheck() error                                           // 健康检查
	GetLatency() time.Duration                                    // 获取延迟
}

// DataSourceStatus 数据源状态
type DataSourceStatus struct {
	Name           string        // 数据源名称
	Healthy        bool          // 是否健康
	Latency        time.Duration // 延迟
	LastCheckTime  time.Time     // 最后检查时间
	FailureCount   int           // 连续失败次数
	SuccessCount   int           // 总成功次数
	TotalRequests  int           // 总请求次数
}

// DataSourceManager 数据源管理器
type DataSourceManager struct {
	sources      []DataSource                   // 数据源列表
	statuses     map[string]*DataSourceStatus   // 数据源状态
	currentIndex int                             // 当前使用的数据源索引（轮询）
	mu           sync.RWMutex                   // 读写锁
	stopChan     chan struct{}                  // 停止信号
	checkInterval time.Duration                 // 健康检查间隔
}

// NewDataSourceManager 创建数据源管理器
func NewDataSourceManager(checkInterval time.Duration) *DataSourceManager {
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second // 默认30秒检查一次
	}

	return &DataSourceManager{
		sources:       make([]DataSource, 0),
		statuses:      make(map[string]*DataSourceStatus),
		currentIndex:  0,
		stopChan:      make(chan struct{}),
		checkInterval: checkInterval,
	}
}

// AddSource 添加数据源
func (dsm *DataSourceManager) AddSource(source DataSource) {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	dsm.sources = append(dsm.sources, source)
	dsm.statuses[source.GetName()] = &DataSourceStatus{
		Name:          source.GetName(),
		Healthy:       true,
		LastCheckTime: time.Now(),
	}

	log.Printf("✅ 添加数据源: %s", source.GetName())
}

// Start 启动健康检查
func (dsm *DataSourceManager) Start() {
	log.Printf("🚀 启动数据源管理器，健康检查间隔: %v", dsm.checkInterval)

	go func() {
		ticker := time.NewTicker(dsm.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				dsm.performHealthCheck()
			case <-dsm.stopChan:
				log.Println("⏹  数据源管理器已停止")
				return
			}
		}
	}()
}

// Stop 停止健康检查
func (dsm *DataSourceManager) Stop() {
	close(dsm.stopChan)
}

// performHealthCheck 执行健康检查
func (dsm *DataSourceManager) performHealthCheck() {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	log.Println("🔍 执行数据源健康检查...")

	for _, source := range dsm.sources {
		status := dsm.statuses[source.GetName()]
		status.LastCheckTime = time.Now()

		start := time.Now()
		err := source.HealthCheck()
		latency := time.Since(start)

		if err != nil {
			status.Healthy = false
			status.FailureCount++
			log.Printf("❌ 数据源 %s 健康检查失败: %v (连续失败 %d 次)",
				source.GetName(), err, status.FailureCount)
		} else {
			status.Healthy = true
			status.FailureCount = 0
			status.Latency = latency
			status.SuccessCount++
			log.Printf("✅ 数据源 %s 健康检查成功 (延迟: %v)",
				source.GetName(), latency)
		}
	}

	// 打印健康摘要
	healthy, total := dsm.getHealthySummary()
	log.Printf("📊 数据源健康状态: %d/%d 健康", healthy, total)
}

// getHealthySummary 获取健康摘要（内部调用，不加锁）
func (dsm *DataSourceManager) getHealthySummary() (healthy, total int) {
	total = len(dsm.sources)
	for _, status := range dsm.statuses {
		if status.Healthy {
			healthy++
		}
	}
	return
}

// GetHealthySource 获取一个健康的数据源（轮询）
func (dsm *DataSourceManager) GetHealthySource() (DataSource, error) {
	dsm.mu.Lock()
	defer dsm.mu.Unlock()

	if len(dsm.sources) == 0 {
		return nil, fmt.Errorf("没有可用的数据源")
	}

	// 尝试从当前索引开始查找健康的数据源
	for i := 0; i < len(dsm.sources); i++ {
		idx := (dsm.currentIndex + i) % len(dsm.sources)
		source := dsm.sources[idx]
		status := dsm.statuses[source.GetName()]

		if status.Healthy {
			// 更新索引到下一个（轮询）
			dsm.currentIndex = (idx + 1) % len(dsm.sources)
			return source, nil
		}
	}

	// 所有数据源都不健康，返回第一个并警告
	log.Printf("⚠️  所有数据源都不健康，强制使用 %s", dsm.sources[0].GetName())
	dsm.currentIndex = 1 % len(dsm.sources)
	return dsm.sources[0], nil
}

// GetKlinesWithFallback 获取K线数据（带故障转移）
func (dsm *DataSourceManager) GetKlinesWithFallback(symbol, interval string, limit int) ([]Kline, error) {
	dsm.mu.Lock()
	sources := make([]DataSource, len(dsm.sources))
	copy(sources, dsm.sources)
	dsm.mu.Unlock()

	var lastErr error

	// 尝试所有健康的数据源
	for _, source := range sources {
		dsm.mu.RLock()
		status := dsm.statuses[source.GetName()]
		healthy := status.Healthy
		dsm.mu.RUnlock()

		if !healthy {
			continue // 跳过不健康的数据源
		}

		klines, err := source.GetKlines(symbol, interval, limit)

		dsm.mu.Lock()
		status.TotalRequests++
		dsm.mu.Unlock()

		if err == nil && len(klines) > 0 {
			log.Printf("✅ 从 %s 获取 %s %s K线数据成功 (%d 条)",
				source.GetName(), symbol, interval, len(klines))
			return klines, nil
		}

		lastErr = err
		log.Printf("⚠️  从 %s 获取数据失败: %v，尝试下一个数据源...",
			source.GetName(), err)
	}

	return nil, fmt.Errorf("所有数据源都失败: %w", lastErr)
}

// GetTickerWithFallback 获取ticker数据（带故障转移）
func (dsm *DataSourceManager) GetTickerWithFallback(symbol string) (*Ticker, error) {
	dsm.mu.Lock()
	sources := make([]DataSource, len(dsm.sources))
	copy(sources, dsm.sources)
	dsm.mu.Unlock()

	var lastErr error

	// 尝试所有健康的数据源
	for _, source := range sources {
		dsm.mu.RLock()
		status := dsm.statuses[source.GetName()]
		healthy := status.Healthy
		dsm.mu.RUnlock()

		if !healthy {
			continue
		}

		ticker, err := source.GetTicker(symbol)

		dsm.mu.Lock()
		status.TotalRequests++
		dsm.mu.Unlock()

		if err == nil && ticker != nil {
			return ticker, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("所有数据源都失败: %w", lastErr)
}

// GetStatus 获取所有数据源的状态
func (dsm *DataSourceManager) GetStatus() map[string]*DataSourceStatus {
	dsm.mu.RLock()
	defer dsm.mu.RUnlock()

	// 复制状态（避免并发修改）
	statusCopy := make(map[string]*DataSourceStatus)
	for name, status := range dsm.statuses {
		statusCopy[name] = &DataSourceStatus{
			Name:          status.Name,
			Healthy:       status.Healthy,
			Latency:       status.Latency,
			LastCheckTime: status.LastCheckTime,
			FailureCount:  status.FailureCount,
			SuccessCount:  status.SuccessCount,
			TotalRequests: status.TotalRequests,
		}
	}

	return statusCopy
}

// VerifyPriceConsistency 验证价格一致性（对比多个数据源）
func (dsm *DataSourceManager) VerifyPriceConsistency(symbol string, maxDeviation float64) (bool, map[string]float64, error) {
	dsm.mu.Lock()
	sources := make([]DataSource, len(dsm.sources))
	copy(sources, dsm.sources)
	dsm.mu.Unlock()

	prices := make(map[string]float64)

	// 从所有健康的数据源获取价格
	for _, source := range sources {
		dsm.mu.RLock()
		healthy := dsm.statuses[source.GetName()].Healthy
		dsm.mu.RUnlock()

		if !healthy {
			continue
		}

		ticker, err := source.GetTicker(symbol)
		if err == nil && ticker != nil {
			prices[source.GetName()] = ticker.LastPrice
		}
	}

	if len(prices) < 2 {
		return true, prices, fmt.Errorf("数据源不足，无法验证价格一致性")
	}

	// 计算平均价格
	var sum float64
	for _, price := range prices {
		sum += price
	}
	avgPrice := sum / float64(len(prices))

	// 检查偏差
	consistent := true
	for name, price := range prices {
		deviation := abs((price - avgPrice) / avgPrice)
		if deviation > maxDeviation {
			consistent = false
			log.Printf("⚠️  价格异常: %s 的 %s 价格 %.2f 偏离平均值 %.2f (偏差 %.2f%%)",
				name, symbol, price, avgPrice, deviation*100)
		}
	}

	if consistent {
		log.Printf("✅ 价格一致性验证通过: %s 平均价格 %.2f", symbol, avgPrice)
	}

	return consistent, prices, nil
}

// abs 绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
