# 提示詞管理功能完整實現指南

## ✅ 已完成部分

### 1. 後端核心功能（decision/prompt_manager.go）
- [x] `SavePromptTemplate(name, content string)` - 保存模板
- [x] `DeletePromptTemplate(name string)` - 刪除模板
- [x] `TemplateExists(name string)` - 檢查存在性
- [x] 自動熱重載
- [x] 系統模板保護

---

## 🚧 待完成部分

### 2. 後端 API Endpoints（api/server.go）

**在第 209 行後添加路由：**

```go
// 提示词模板管理（需要认证）
protected.POST("/prompt-templates", s.handleCreatePromptTemplate)
protected.PUT("/prompt-templates/:name", s.handleUpdatePromptTemplate)
protected.DELETE("/prompt-templates/:name", s.handleDeletePromptTemplate)
protected.POST("/prompt-templates/reload", s.handleReloadPromptTemplates)
```

**在文件末尾添加處理函數：**

```go
// handleCreatePromptTemplate 創建新的提示詞模板
func (s *Server) handleCreatePromptTemplate(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請求參數錯誤: " + err.Error()})
		return
	}

	// 檢查模板是否已存在
	if decision.TemplateExists(req.Name) {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("模板已存在: %s", req.Name)})
		return
	}

	// 保存模板
	if err := decision.SavePromptTemplate(req.Name, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("創建模板失敗: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模板創建成功",
		"name":    req.Name,
	})
}

// handleUpdatePromptTemplate 更新提示詞模板
func (s *Server) handleUpdatePromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "請求參數錯誤: " + err.Error()})
		return
	}

	// 檢查模板是否存在
	if !decision.TemplateExists(templateName) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("模板不存在: %s", templateName)})
		return
	}

	// 更新模板
	if err := decision.SavePromptTemplate(templateName, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("更新模板失敗: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模板更新成功",
		"name":    templateName,
	})
}

// handleDeletePromptTemplate 刪除提示詞模板
func (s *Server) handleDeletePromptTemplate(c *gin.Context) {
	templateName := c.Param("name")

	// 刪除模板
	if err := decision.DeletePromptTemplate(templateName); err != nil {
		if strings.Contains(err.Error(), "不能刪除系統模板") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else if strings.Contains(err.Error(), "模板不存在") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("刪除模板失敗: %v", err)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模板刪除成功",
	})
}

// handleReloadPromptTemplates 重新加載所有提示詞模板
func (s *Server) handleReloadPromptTemplates(c *gin.Context) {
	if err := decision.ReloadPromptTemplates(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("重新加載失敗: %v", err),
		})
		return
	}

	templates := decision.GetAllPromptTemplates()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "重新加載成功",
		"count":   len(templates),
	})
}
```

---

### 3. 前端獨立管理頁面（web/src/components/PromptManagementPage.tsx）

**參考另一個分支的實現，創建完整的管理頁面：**

```tsx
import { useEffect, useState } from 'react'
import { toast } from 'sonner'

interface PromptTemplate {
  name: string
  content: string
  display_name?: { [key: string]: string }
  description?: { [key: string]: string }
}

export default function PromptManagementPage() {
  const [templates, setTemplates] = useState<PromptTemplate[]>([])
  const [selectedTemplate, setSelectedTemplate] = useState<PromptTemplate | null>(null)
  const [editContent, setEditContent] = useState('')
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false)
  const [newTemplateName, setNewTemplateName] = useState('')
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false)

  // 加載模板列表
  const loadTemplates = async () => {
    try {
      const response = await fetch('/api/prompt-templates')
      const data = await response.json()
      setTemplates(data.templates || [])
    } catch (error) {
      console.error('加載模板失敗:', error)
      toast.error('加載模板失敗')
    }
  }

  useEffect(() => {
    loadTemplates()
  }, [])

  // 選擇模板
  const handleSelectTemplate = (template: PromptTemplate) => {
    setSelectedTemplate(template)
    setEditContent(template.content)
  }

  // 保存模板
  const handleSave = async () => {
    if (!selectedTemplate) return

    try {
      const response = await fetch(`/api/prompt-templates/${selectedTemplate.name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: editContent }),
      })

      if (response.ok) {
        toast.success('保存成功')
        loadTemplates()
      } else {
        const error = await response.json()
        toast.error(error.error || '保存失敗')
      }
    } catch (error) {
      console.error('保存失敗:', error)
      toast.error('保存失敗')
    }
  }

  // 創建新模板
  const handleCreate = async () => {
    if (!newTemplateName.trim()) {
      toast.error('請輸入模板名稱')
      return
    }

    try {
      const response = await fetch('/api/prompt-templates', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newTemplateName,
          content: editContent || '# 新模板\n\n請輸入您的提示詞內容...',
        }),
      })

      if (response.ok) {
        toast.success('創建成功')
        setIsCreateModalOpen(false)
        setNewTemplateName('')
        loadTemplates()
      } else {
        const error = await response.json()
        toast.error(error.error || '創建失敗')
      }
    } catch (error) {
      console.error('創建失敗:', error)
      toast.error('創建失敗')
    }
  }

  // 刪除模板
  const handleDelete = async () => {
    if (!selectedTemplate) return

    try {
      const response = await fetch(`/api/prompt-templates/${selectedTemplate.name}`, {
        method: 'DELETE',
      })

      if (response.ok) {
        toast.success('刪除成功')
        setIsDeleteModalOpen(false)
        setSelectedTemplate(null)
        setEditContent('')
        loadTemplates()
      } else {
        const error = await response.json()
        toast.error(error.error || '刪除失敗')
      }
    } catch (error) {
      console.error('刪除失敗:', error)
      toast.error('刪除失敗')
    }
  }

  return (
    <div className="min-h-screen p-6" style={{ background: '#0B0E11', color: '#EAECEF' }}>
      {/* Header */}
      <div className="max-w-7xl mx-auto mb-8">
        <h1 className="text-3xl font-bold mb-2">💬 提示詞管理</h1>
        <p className="text-gray-400">管理您的 AI 交易策略提示詞模板</p>
      </div>

      {/* Actions */}
      <div className="max-w-7xl mx-auto mb-6 flex gap-4">
        <button
          onClick={() => setIsCreateModalOpen(true)}
          className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
          style={{ background: '#F0B90B', color: '#000' }}
        >
          + 新建模板
        </button>
        <button
          onClick={loadTemplates}
          className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
          style={{ background: 'rgba(240, 185, 11, 0.1)', color: '#F0B90B', border: '1px solid #F0B90B' }}
        >
          🔄 刷新
        </button>
      </div>

      {/* Main Content: Template List + Editor */}
      <div className="max-w-7xl mx-auto grid grid-cols-12 gap-6">
        {/* Template List (Left Sidebar) */}
        <div className="col-span-3 bg-[#1E2329] border border-[#2B3139] rounded-lg p-4">
          <h2 className="text-lg font-bold mb-4">📁 模板列表 ({templates.length})</h2>
          <div className="space-y-2">
            {templates.map((template) => (
              <button
                key={template.name}
                onClick={() => handleSelectTemplate(template)}
                className={`w-full text-left px-3 py-2 rounded transition-all ${
                  selectedTemplate?.name === template.name
                    ? 'bg-yellow-500 bg-opacity-20 border border-yellow-500'
                    : 'hover:bg-gray-700'
                }`}
                style={{
                  color: selectedTemplate?.name === template.name ? '#F0B90B' : '#EAECEF',
                }}
              >
                {template.name === 'default' && '⭐ '}
                {template.display_name?.zh || template.name}
              </button>
            ))}
          </div>
        </div>

        {/* Editor (Right Panel) */}
        <div className="col-span-9 bg-[#1E2329] border border-[#2B3139] rounded-lg p-6">
          {selectedTemplate ? (
            <>
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-xl font-bold">
                  📝 {selectedTemplate.display_name?.zh || selectedTemplate.name}
                </h2>
                <div className="flex gap-3">
                  <button
                    onClick={handleSave}
                    className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
                    style={{ background: '#0ECB81', color: '#FFF' }}
                  >
                    💾 保存
                  </button>
                  {selectedTemplate.name !== 'default' && (
                    <button
                      onClick={() => setIsDeleteModalOpen(true)}
                      className="px-4 py-2 rounded font-semibold transition-all hover:scale-105"
                      style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D', border: '1px solid #F6465D' }}
                    >
                      🗑️ 刪除
                    </button>
                  )}
                </div>
              </div>

              {selectedTemplate.description?.zh && (
                <p className="text-sm text-gray-400 mb-4">{selectedTemplate.description.zh}</p>
              )}

              <textarea
                value={editContent}
                onChange={(e) => setEditContent(e.target.value)}
                className="w-full h-[500px] p-4 rounded font-mono text-sm"
                style={{
                  background: '#0B0E11',
                  color: '#EAECEF',
                  border: '1px solid #2B3139',
                  resize: 'none',
                }}
              />

              <div className="mt-2 flex justify-between text-xs text-gray-500">
                <span>字符數: {editContent.length}</span>
                <span>行數: {editContent.split('\n').length}</span>
              </div>
            </>
          ) : (
            <div className="flex flex-col items-center justify-center h-[500px] text-gray-500">
              <p className="text-lg">請從左側選擇一個模板</p>
            </div>
          )}
        </div>
      </div>

      {/* Create Modal */}
      {isCreateModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-[#1E2329] border border-[#2B3139] rounded-lg p-6 w-96">
            <h2 className="text-xl font-bold mb-4">新建模板</h2>
            <input
              type="text"
              value={newTemplateName}
              onChange={(e) => setNewTemplateName(e.target.value)}
              placeholder="輸入模板名稱（英文）"
              className="w-full px-3 py-2 rounded mb-4"
              style={{ background: '#0B0E11', color: '#EAECEF', border: '1px solid #2B3139' }}
            />
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setIsCreateModalOpen(false)}
                className="px-4 py-2 rounded"
                style={{ background: 'rgba(255,255,255,0.1)', color: '#EAECEF' }}
              >
                取消
              </button>
              <button
                onClick={handleCreate}
                className="px-4 py-2 rounded font-semibold"
                style={{ background: '#F0B90B', color: '#000' }}
              >
                創建
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {isDeleteModalOpen && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-[#1E2329] border border-[#2B3139] rounded-lg p-6 w-96">
            <h2 className="text-xl font-bold mb-4">確認刪除</h2>
            <p className="mb-4 text-gray-400">
              確定要刪除模板「{selectedTemplate?.name}」嗎？此操作無法撤銷。
            </p>
            <div className="flex gap-3 justify-end">
              <button
                onClick={() => setIsDeleteModalOpen(false)}
                className="px-4 py-2 rounded"
                style={{ background: 'rgba(255,255,255,0.1)', color: '#EAECEF' }}
              >
                取消
              </button>
              <button
                onClick={handleDelete}
                className="px-4 py-2 rounded font-semibold"
                style={{ background: '#F6465D', color: '#FFF' }}
              >
                刪除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
```

---

### 4. 添加路由（web/src/App.tsx）

**在 type Page 定義中添加：**
```typescript
type Page = 'competition' | 'traders' | 'trader' | 'faq' | 'prompts'
```

**導入組件：**
```typescript
import PromptManagementPage from './components/PromptManagementPage'
```

**添加路由處理（參考 /trader 路由的位置）：**
```typescript
if (route === '/prompts') {
  return (
    <div className="min-h-screen" style={{ background: '#000000', color: '#EAECEF' }}>
      <HeaderBar
        isLoggedIn={!!user}
        currentPage="prompts"
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={(page) => {
          if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            setRoute('/traders')
            setCurrentPage('traders')
          } else if (page === 'prompts') {
            window.history.pushState({}, '', '/prompts')
            setRoute('/prompts')
            setCurrentPage('prompts')
          }
        }}
      />
      <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
        <PromptManagementPage />
      </main>
    </div>
  )
}
```

---

### 5. 添加導航按鈕（web/src/components/landing/HeaderBar.tsx）

**在「常見問題」按鈕後添加：**

```tsx
<button
  onClick={() => {
    console.log('Prompts button clicked, onPageChange:', onPageChange)
    onPageChange?.('prompts')
  }}
  className="text-sm font-bold transition-all duration-300 relative"
  style={{
    color: currentPage === 'prompts' ? 'var(--brand-yellow)' : 'var(--brand-light-gray)',
    padding: '8px 16px',
    borderRadius: '8px',
  }}
>
  {currentPage === 'prompts' && (
    <span className="absolute inset-0 rounded-lg" style={{ background: 'rgba(240, 185, 11, 0.15)', zIndex: -1 }} />
  )}
  💬 提示詞
</button>
```

**同樣在移動端菜單中添加：**
```tsx
<button
  onClick={() => {
    onPageChange?.('prompts')
    setMobileMenuOpen(false)
  }}
  className="block text-sm font-bold transition-all"
  style={{
    color: currentPage === 'prompts' ? 'var(--brand-yellow)' : 'var(--brand-light-gray)',
    padding: '12px 16px',
  }}
>
  💬 提示詞
</button>
```

---

## 🧪 測試步驟

1. **重啟後端：**
   ```bash
   cd ~/Documents/GitHub/nofx
   go run main.go
   ```

2. **啟動前端：**
   ```bash
   cd web
   npm run dev
   ```

3. **訪問頁面：**
   - http://localhost:3000/prompts

4. **測試功能：**
   - ✅ 查看模板列表
   - ✅ 點擊模板查看內容
   - ✅ 編輯並保存模板
   - ✅ 創建新模板
   - ✅ 刪除模板（除了 default）
   - ✅ 驗證熱重載（保存後立即在 TraderConfigModal 的下拉選單中看到）

---

## 📝 後續優化（Phase 2+）

### Phase 2：UX 優化
- [ ] 在 TraderConfigModal 中整合簡化選擇器
- [ ] 添加「完整提示詞預覽」功能

### Phase 3：進階功能
- [ ] 使用 Monaco Editor 替代 Textarea
- [ ] 提示詞版本歷史
- [ ] 使用統計（哪些模板被使用最多）
- [ ] A/B 測試對比
- [ ] 模板導入/導出

---

## 🔗 參考資料

- 另一個分支的實現：參考 nofxos-private 的 PromptManagementPage.tsx
- API 設計：RESTful CRUD + 熱重載
- UI/UX：參考 AITradersPage.tsx 的設計風格

---

**完成時間估計：**
- 後端 API：30 分鐘
- 前端頁面：1-2 小時
- 路由整合：30 分鐘
- 測試調試：30 分鐘

**總計：約 3-4 小時**
