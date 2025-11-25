import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { useAuth } from '../contexts/AuthContext'
import { useTradersConfigStore, useTradersModalStore } from '../stores'
import { useTraderActions } from '../hooks/useTraderActions'
import { TraderConfigModal } from '../components/TraderConfigModal'
import {
  SignalSourceModal,
  ModelConfigModal,
  ExchangeConfigModal,
} from '../components/traders'
import { PageHeader } from '../components/traders/sections/PageHeader'
import { SignalSourceWarning } from '../components/traders/sections/SignalSourceWarning'
import { AIModelsSection } from '../components/traders/sections/AIModelsSection'
import { ExchangesSection } from '../components/traders/sections/ExchangesSection'
import { TradersGrid } from '../components/traders/sections/TradersGrid'

interface AITradersPageProps {
  onTraderSelect?: (traderId: string) => void
}

export function AITradersPage({ onTraderSelect }: AITradersPageProps) {
  const { language } = useLanguage()
  const { user, token } = useAuth()
  const navigate = useNavigate()

  // Zustand stores
  const {
    allModels,
    allExchanges,
    supportedModels,
    supportedExchanges,
    configuredModels,
    configuredExchanges,
    userSignalSource,
    loadConfigs,
    setAllModels,
    setAllExchanges,
    setUserSignalSource,
  } = useTradersConfigStore()

  const {
    showCreateModal,
    showEditModal,
    showModelModal,
    showExchangeModal,
    showSignalSourceModal,
    editingModel,
    editingExchange,
    editingTrader,
    setShowCreateModal,
    setShowEditModal,
    setShowModelModal,
    setShowExchangeModal,
    setShowSignalSourceModal,
    setEditingModel,
    setEditingExchange,
    setEditingTrader,
  } = useTradersModalStore()

  // SWR for traders data
  const { data: traders, mutate: mutateTraders } = useSWR(
    user && token ? 'traders' : null,
    api.getTraders,
    { refreshInterval: 5000 }
  )

  // Load configurations
  useEffect(() => {
    loadConfigs(user, token)
  }, [user, token, loadConfigs])

  // Business logic hook
  const {
    isModelInUse,
    isExchangeInUse,
    handleCreateTrader,
    handleEditTrader,
    handleSaveEditTrader,
    handleDeleteTrader,
    handleToggleTrader,
    handleAddModel,
    handleAddExchange,
    handleModelClick,
    handleExchangeClick,
    handleSaveModel,
    handleDeleteModel,
    handleSaveExchange,
    handleDeleteExchange,
    handleSaveSignalSource,
  } = useTraderActions({
    traders,
    allModels,
    allExchanges,
    supportedModels,
    supportedExchanges,
    language,
    mutateTraders,
    setAllModels,
    setAllExchanges,
    setUserSignalSource,
    setShowCreateModal,
    setShowEditModal,
    setShowModelModal,
    setShowExchangeModal,
    setShowSignalSourceModal,
    setEditingModel,
    setEditingExchange,
    editingTrader,
    setEditingTrader,
  })

  // 计算派生状态 - 只在创建交易员时使用已启用的配置
  // 注意：后端出于安全考虑不返回 apiKey 等敏感字段
  // enabled=true 表示用户已配置完整的 API Key（后端已验证并存储）
  const enabledModels = allModels?.filter((m) => m.enabled) || []
  const enabledExchanges = allExchanges?.filter((e) => e.enabled) || []

  // 检查是否需要显示信号源警告
  const showSignalWarning =
    traders?.some((t) => t.use_coin_pool || t.use_oi_top) &&
    !userSignalSource.coinPoolUrl &&
    !userSignalSource.oiTopUrl

  // 处理交易员查看
  const handleTraderSelect = (traderId: string) => {
    if (onTraderSelect) {
      onTraderSelect(traderId)
    } else {
      navigate(`/dashboard?trader=${traderId}`)
    }
  }

  return (
    <div className="space-y-4 md:space-y-6 animate-fade-in">
      {/* Header */}
      <PageHeader
        language={language}
        tradersCount={traders?.length || 0}
        configuredModelsCount={configuredModels.length}
        configuredExchangesCount={configuredExchanges.length}
        onAddModel={handleAddModel}
        onAddExchange={handleAddExchange}
        onConfigureSignalSource={() => setShowSignalSourceModal(true)}
        onCreateTrader={() => setShowCreateModal(true)}
      />

      {/* Signal Source Warning */}
      {showSignalWarning && (
        <SignalSourceWarning
          language={language}
          onConfigure={() => setShowSignalSourceModal(true)}
        />
      )}

      {/* Configuration Status */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 md:gap-6">
        <AIModelsSection
          language={language}
          configuredModels={configuredModels}
          isModelInUse={isModelInUse}
          onModelClick={handleModelClick}
        />

        <ExchangesSection
          language={language}
          configuredExchanges={configuredExchanges}
          isExchangeInUse={isExchangeInUse}
          onExchangeClick={handleExchangeClick}
        />
      </div>

      {/* Traders Grid */}
      <TradersGrid
        language={language}
        traders={traders}
        configuredModelsCount={configuredModels.length}
        configuredExchangesCount={configuredExchanges.length}
        onTraderSelect={handleTraderSelect}
        onEditTrader={handleEditTrader}
        onDeleteTrader={handleDeleteTrader}
        onToggleTrader={handleToggleTrader}
      />

      {/* Modals */}
      <TraderConfigModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        isEditMode={false}
        availableModels={enabledModels}
        availableExchanges={enabledExchanges}
        onSave={handleCreateTrader}
      />

      <TraderConfigModal
        isOpen={showEditModal}
        onClose={() => setShowEditModal(false)}
        isEditMode={true}
        traderData={editingTrader}
        availableModels={enabledModels}
        availableExchanges={enabledExchanges}
        onSave={handleSaveEditTrader}
      />

      {showModelModal && (
        <ModelConfigModal
          allModels={supportedModels}
          configuredModels={allModels}
          editingModelId={editingModel}
          onSave={handleSaveModel}
          onDelete={handleDeleteModel}
          onClose={() => setShowModelModal(false)}
          language={language}
        />
      )}

      {showExchangeModal && (
        <ExchangeConfigModal
          allExchanges={supportedExchanges}
          editingExchangeId={editingExchange}
          onSave={handleSaveExchange}
          onDelete={handleDeleteExchange}
          onClose={() => setShowExchangeModal(false)}
          language={language}
        />
      )}

      {showSignalSourceModal && (
        <SignalSourceModal
          coinPoolUrl={userSignalSource.coinPoolUrl}
          oiTopUrl={userSignalSource.oiTopUrl}
          onSave={handleSaveSignalSource}
          onClose={() => setShowSignalSourceModal(false)}
          language={language}
        />
      )}

      {/* Fork attribution - Experimental version warning */}
      {typeof __GIT_BRANCH__ !== 'undefined' &&
        __GIT_BRANCH__ !== 'unknown' &&
        __GIT_BRANCH__ !== 'main' &&
        __GIT_BRANCH__ !== 'master' && (
          <div className="mt-8 pt-6" style={{ borderTop: '1px solid var(--panel-border)' }}>
            <div className="text-center text-xs" style={{ color: 'var(--text-tertiary)' }}>
              <p className="mb-1" style={{ fontWeight: '500' }}>
                {language === 'zh'
                  ? '實驗性社區版本（非官方）'
                  : 'Experimental Community Fork (Unofficial)'}
              </p>
              <p style={{ fontSize: '0.7rem' }}>
                {language === 'zh' ? '維護者：' : 'Maintainer: '}
                <a
                  href="https://github.com/the-dev-z/nofx"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-[#F0B90B] transition-colors"
                  style={{ color: 'var(--brand-yellow)' }}
                >
                  the-dev-z/nofx
                </a>
                {' ('}
                <span style={{ color: '#848E9C' }}>{__GIT_BRANCH__}</span>
                {') | '}
                {language === 'zh' ? '上游：' : 'Upstream: '}
                <a
                  href="https://github.com/tinkle-community/nofx"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="hover:text-[#F0B90B] transition-colors"
                  style={{ color: '#848E9C' }}
                >
                  tinkle-community/nofx
                </a>
              </p>
            </div>
          </div>
        )}
    </div>
  )
}
