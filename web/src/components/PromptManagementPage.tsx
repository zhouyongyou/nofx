import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { httpClient } from '../lib/httpClient'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

interface PromptTemplate {
  name: string
  content: string
  display_name?: { [key: string]: string }
  description?: { [key: string]: string }
}

export default function PromptManagementPage() {
  const { language } = useLanguage()
  const [templates, setTemplates] = useState<PromptTemplate[]>([])
  const [selectedTemplate, setSelectedTemplate] =
    useState<PromptTemplate | null>(null)
  const [editContent, setEditContent] = useState('')
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [newTemplateName, setNewTemplateName] = useState('')
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false)

  // 加载模板列表
  const loadTemplates = async () => {
    try {
      const response = await httpClient.get<{ templates: PromptTemplate[] }>(
        '/api/prompt-templates'
      )
      if (response.success && response.data) {
        setTemplates(response.data.templates || [])
      } else {
        toast.error(response.message || t('promptsLoadFailed', language))
      }
    } catch (error) {
      console.error('加载模板失败:', error)
      // Network errors are already handled by httpClient, no need to toast here
    }
  }

  useEffect(() => {
    loadTemplates()
  }, [])

  // 选择模板
  const handleSelectTemplate = async (template: PromptTemplate) => {
    setSelectedTemplate(template)

    // 获取完整的模板内容
    try {
      const response = await httpClient.get<{ content: string }>(
        `/api/prompt-templates/${template.name}`
      )
      if (response.success && response.data) {
        setEditContent(response.data.content || '')
      } else {
        toast.error(response.message || t('promptsGetFailed', language))
        setEditContent('')
      }
    } catch (error) {
      console.error('获取模板内容失败:', error)
      setEditContent('')
    }
  }

  // 保存模板
  const handleSave = async () => {
    if (!selectedTemplate) return

    try {
      const response = await httpClient.put(
        `/api/prompt-templates/${selectedTemplate.name}`,
        { content: editContent }
      )

      if (response.success) {
        toast.success(t('promptsSaveSuccess', language))
        loadTemplates()
      } else {
        toast.error(response.message || t('promptsSaveFailed', language))
      }
    } catch (error) {
      console.error('保存失败:', error)
    }
  }

  // 创建新模板
  const handleCreate = async () => {
    if (!newTemplateName.trim()) {
      toast.error(t('promptsEnterName', language))
      return
    }

    try {
      const response = await httpClient.post(
        '/api/prompt-templates',
        {
          name: newTemplateName,
          content: '# 新模板\n\n请输入您的提示词内容...',
        }
      )

      if (response.success) {
        toast.success(t('promptsCreateSuccess', language))
        setIsCreateModalOpen(false)
        setNewTemplateName('')
        loadTemplates()
      } else {
        toast.error(response.message || t('promptsCreateFailed', language))
      }
    } catch (error) {
      console.error('创建失败:', error)
    }
  }

  // 删除模板
  const handleDelete = async () => {
    if (!selectedTemplate) return

    try {
      const response = await httpClient.delete(
        `/api/prompt-templates/${selectedTemplate.name}`
      )

      if (response.success) {
        toast.success(t('promptsDeleteSuccess', language))
        setIsDeleteModalOpen(false)
        setSelectedTemplate(null)
        setEditContent('')
        loadTemplates()
      } else {
        toast.error(response.message || t('promptsDeleteFailed', language))
      }
    } catch (error) {
      console.error('删除失败:', error)
    }
  }

  return (
    <div
      className="min-h-screen p-3 sm:p-6"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      {/* Header */}
      <div className="max-w-7xl mx-auto mb-4 sm:mb-8">
        <h1 className="text-2xl sm:text-3xl font-bold mb-2">{t('promptsManagement', language)}</h1>
        <p className="text-sm sm:text-base text-gray-400">
          {t('promptsManagementDesc', language)}
        </p>
      </div>

      {/* Actions */}
      <div className="max-w-7xl mx-auto mb-4 sm:mb-6 flex flex-col sm:flex-row gap-2 sm:gap-4">
        <button
          onClick={() => setIsCreateModalOpen(true)}
          className="px-4 py-2 rounded font-semibold transition-all hover:scale-105 text-sm sm:text-base"
          style={{ background: '#F0B90B', color: '#000' }}
        >
          {t('promptsCreateBtn', language)}
        </button>
        <button
          onClick={loadTemplates}
          className="px-4 py-2 rounded font-semibold transition-all hover:scale-105 text-sm sm:text-base"
          style={{
            background: 'rgba(240, 185, 11, 0.1)',
            color: '#F0B90B',
            border: '1px solid #F0B90B',
          }}
        >
          {t('promptsRefreshBtn', language)}
        </button>
      </div>

      {/* Main Content: Template List + Editor */}
      <div className="max-w-7xl mx-auto grid grid-cols-1 lg:grid-cols-12 gap-4 lg:gap-6">
        {/* Template List (Left Sidebar) */}
        <div className="lg:col-span-3 bg-[#1E2329] border border-[#2B3139] rounded-lg p-3 sm:p-4">
          <h2 className="text-base sm:text-lg font-bold mb-3 sm:mb-4">
            {t('promptsTemplateList', language)} ({templates.length})
          </h2>
          <div className="space-y-2 max-h-[200px] lg:max-h-none overflow-y-auto lg:overflow-visible">
            {templates.map((template) => (
              <button
                key={template.name}
                onClick={() => handleSelectTemplate(template)}
                className={`w-full text-left px-3 py-2 rounded transition-all text-sm sm:text-base ${
                  selectedTemplate?.name === template.name
                    ? 'bg-yellow-500 bg-opacity-20 border border-yellow-500'
                    : 'hover:bg-gray-700'
                }`}
                style={{
                  color:
                    selectedTemplate?.name === template.name
                      ? '#F0B90B'
                      : '#EAECEF',
                }}
              >
                {template.name === 'default' && '⭐ '}
                {template.display_name?.[language] || template.name}
              </button>
            ))}
          </div>
        </div>

        {/* Editor (Right Panel) */}
        <div className="lg:col-span-9 bg-[#1E2329] border border-[#2B3139] rounded-lg p-4 sm:p-6">
          {selectedTemplate ? (
            <>
              <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 gap-3">
                <h2 className="text-lg sm:text-xl font-bold truncate">
                  📝{' '}
                  {selectedTemplate.display_name?.[language] || selectedTemplate.name}
                </h2>
                <div className="flex gap-2 sm:gap-3 flex-shrink-0">
                  <button
                    onClick={handleSave}
                    className="flex-1 sm:flex-none px-3 sm:px-4 py-2 rounded font-semibold transition-all hover:scale-105 text-sm sm:text-base"
                    style={{ background: '#0ECB81', color: '#FFF' }}
                  >
                    {t('promptsSaveBtn', language)}
                  </button>
                  {selectedTemplate.name !== 'default' && (
                    <button
                      onClick={() => setIsDeleteModalOpen(true)}
                      className="flex-1 sm:flex-none px-3 sm:px-4 py-2 rounded font-semibold transition-all hover:scale-105 text-sm sm:text-base"
                      style={{
                        background: 'rgba(246, 70, 93, 0.1)',
                        color: '#F6465D',
                        border: '1px solid #F6465D',
                      }}
                    >
                      {t('promptsDeleteBtn', language)}
                    </button>
                  )}
                </div>
              </div>

              {selectedTemplate.description?.[language] && (
                <p className="text-xs sm:text-sm text-gray-400 mb-4">
                  {selectedTemplate.description[language]}
                </p>
              )}

              <textarea
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                className="w-full h-[400px] sm:h-[500px] p-3 sm:p-4 rounded font-mono text-xs sm:text-sm"
                style={{
                  background: '#0B0E11',
                  color: '#EAECEF',
                  border: '1px solid #2B3139',
                  resize: 'none',
                }}
              />

              <div className="mt-2 flex justify-between text-xs text-gray-500">
                <span>{t('promptsCharCount', language).replace('{count}', String(editContent?.length || 0))}</span>
                <span>{t('promptsLineCount', language).replace('{count}', String(editContent?.split('\n').length || 0))}</span>
              </div>
            </>
          ) : (
            <div className="flex flex-col items-center justify-center h-[300px] sm:h-[500px] text-gray-500">
              <p className="text-base sm:text-lg">{t('promptsSelectPrompt', language)}</p>
            </div>
          )}
        </div>
      </div>

      {/* Create Modal */}
      {isCreateModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-[#1E2329] border border-[#2B3139] rounded-lg p-4 sm:p-6 w-full max-w-md">
            <h2 className="text-lg sm:text-xl font-bold mb-4">{t('promptsNewTemplate', language)}</h2>
            <input
              type="text"
              value={newTemplateName}
              onChange={(e) => setNewTemplateName(e.target.value)}
              placeholder={t('promptsEnterName', language)}
              className="w-full px-3 py-2 rounded mb-4 text-sm sm:text-base"
              style={{
                background: '#0B0E11',
                color: '#EAECEF',
                border: '1px solid #2B3139',
              }}
            />
            <div className="flex flex-col sm:flex-row gap-2 sm:gap-3 sm:justify-end">
              <button
                onClick={() => setIsCreateModalOpen(false)}
                className="px-4 py-2 rounded text-sm sm:text-base order-2 sm:order-1"
                style={{
                  background: 'rgba(255,255,255,0.1)',
                  color: '#EAECEF',
                }}
              >
                {t('promptsCancelBtn', language)}
              </button>
              <button
                onClick={handleCreate}
                className="px-4 py-2 rounded font-semibold text-sm sm:text-base order-1 sm:order-2"
                style={{ background: '#F0B90B', color: '#000' }}
              >
                {t('promptsCreateBtn2', language)}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {isDeleteModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-[#1E2329] border border-[#2B3139] rounded-lg p-4 sm:p-6 w-full max-w-md">
            <h2 className="text-lg sm:text-xl font-bold mb-4">{t('promptsConfirmDelete', language)}</h2>
            <p className="mb-4 text-sm sm:text-base text-gray-400">
              {t('promptsConfirmDeleteMsg', language).replace('{name}', selectedTemplate?.name || '')}
            </p>
            <div className="flex flex-col sm:flex-row gap-2 sm:gap-3 sm:justify-end">
              <button
                onClick={() => setIsDeleteModalOpen(false)}
                className="px-4 py-2 rounded text-sm sm:text-base order-2 sm:order-1"
                style={{
                  background: 'rgba(255,255,255,0.1)',
                  color: '#EAECEF',
                }}
              >
                {t('promptsCancelBtn', language)}
              </button>
              <button
                onClick={handleDelete}
                className="px-4 py-2 rounded font-semibold text-sm sm:text-base order-1 sm:order-2"
                style={{ background: '#F6465D', color: '#FFF' }}
              >
                {t('promptsDeleteBtn', language)}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
