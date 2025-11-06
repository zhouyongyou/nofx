import { mutate } from 'swr'

/**
 * 統一緩存失效機制
 * 解決跨頁面數據同步問題
 *
 * 使用場景：
 * - 創建/更新/刪除交易員後，同步更新所有相關頁面
 * - 避免手動管理多個 mutate 調用
 * - 確保數據一致性
 */

// 定義數據依賴關係
export const CacheDependencies = {
  /**
   * 交易員生命週期相關的緩存
   * 影響：交易員列表、排行榜
   */
  TRADER_LIFECYCLE: [
    'traders',      // App.tsx 導航 + AITradersPage.tsx 列表
    'competition',  // CompetitionPage.tsx 排行榜
  ],

  /**
   * 單個交易員的狀態數據
   */
  TRADER_STATE: (traderId: string) => [
    `status-${traderId}`,      // 運行狀態
    `account-${traderId}`,     // 賬戶信息
    `performance-${traderId}`, // 性能分析
  ],
} as const

/**
 * 緩存管理器
 * 提供語義化的緩存失效方法
 */
export const cacheManager = {
  /**
   * 交易員創建後調用
   * 影響：所有顯示交易員列表的頁面
   */
  onTraderCreated: () => {
    console.log('[Cache] Invalidating after trader created')
    CacheDependencies.TRADER_LIFECYCLE.forEach(key => mutate(key))
  },

  /**
   * 交易員更新後調用（名稱、配置、提示詞等）
   * 影響：列表頁 + 該交易員的詳細頁
   */
  onTraderUpdated: (traderId: string) => {
    console.log(`[Cache] Invalidating after trader updated: ${traderId}`)
    // 更新列表
    CacheDependencies.TRADER_LIFECYCLE.forEach(key => mutate(key))
    // 更新該交易員的詳細數據
    CacheDependencies.TRADER_STATE(traderId).forEach(key => mutate(key))
  },

  /**
   * 交易員刪除後調用
   * 影響：所有相關頁面（包括對比圖表）
   */
  onTraderDeleted: (traderId: string) => {
    console.log(`[Cache] Invalidating after trader deleted: ${traderId}`)
    // 更新列表（移除已刪除的交易員）
    CacheDependencies.TRADER_LIFECYCLE.forEach(key => mutate(key))
    // 清除該交易員的所有緩存
    CacheDependencies.TRADER_STATE(traderId).forEach(key => mutate(key))
    // 清除包含該交易員的對比圖表緩存
    mutate((key: any) =>
      typeof key === 'string' &&
      key.startsWith('all-equity-histories-') &&
      key.includes(traderId)
    )
  },

  /**
   * 交易員狀態改變（啟動/停止）
   * 影響：列表頁的 is_running 狀態 + 狀態詳情
   */
  onTraderStateChanged: (traderId: string) => {
    console.log(`[Cache] Invalidating after trader state changed: ${traderId}`)
    // 更新列表（is_running 字段）
    CacheDependencies.TRADER_LIFECYCLE.forEach(key => mutate(key))
    // 更新該交易員狀態
    mutate(`status-${traderId}`)
  },

  /**
   * 強制刷新所有交易員相關數據
   * 用於：緊急情況或手動刷新按鈕
   */
  refreshAll: () => {
    console.log('[Cache] Force refreshing all trader data')
    mutate((key: any) =>
      typeof key === 'string' && (
        key === 'traders' ||
        key === 'competition' ||
        key.startsWith('status-') ||
        key.startsWith('account-') ||
        key.startsWith('performance-') ||
        key.startsWith('all-equity-histories-')
      )
    )
  },

  /**
   * 餘額同步後調用
   * 影響：賬戶信息和性能統計
   */
  onBalanceSynced: (traderId: string) => {
    console.log(`[Cache] Invalidating after balance synced: ${traderId}`)
    mutate(`account-${traderId}`)
    mutate(`performance-${traderId}`)
    // 列表頁也需要更新（顯示當前餘額）
    CacheDependencies.TRADER_LIFECYCLE.forEach(key => mutate(key))
  },
}
