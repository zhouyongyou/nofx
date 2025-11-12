package decision

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PromptTemplate 系统提示词模板
type PromptTemplate struct {
	Name        string            // 模板名称（文件名，不含扩展名）
	Content     string            // 模板内容
	DisplayName map[string]string // 显示名称（多语言）{"zh": "中文名", "en": "English Name"}
	Description map[string]string // 描述（多语言）
}

// TemplateMetadata 模板元数据配置
type TemplateMetadata struct {
	Name        map[string]string `json:"name"`
	Description map[string]string `json:"description"`
	File        string            `json:"file"`
}

// PromptManager 提示词管理器
type PromptManager struct {
	templates map[string]*PromptTemplate
	mu        sync.RWMutex
}

var (
	// globalPromptManager 全局提示词管理器
	globalPromptManager *PromptManager
	// promptsDir 提示词文件夹路径
	promptsDir = "prompts"
)

// init 包初始化时加载所有提示词模板
func init() {
	globalPromptManager = NewPromptManager()
	if err := globalPromptManager.LoadTemplates(promptsDir); err != nil {
		log.Printf("⚠️  加载提示词模板失败: %v", err)
	} else {
		log.Printf("✓ 已加载 %d 个系统提示词模板", len(globalPromptManager.templates))
	}
}

// NewPromptManager 创建提示词管理器
func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string]*PromptTemplate),
	}
}

// LoadTemplates 从指定目录加载所有提示词模板
func (pm *PromptManager) LoadTemplates(dir string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("提示词目录不存在: %s", dir)
	}

	// 尝试加载 templates.json 配置文件
	metadataMap := make(map[string]*TemplateMetadata)
	configPath := filepath.Join(dir, "templates.json")
	if configData, err := os.ReadFile(configPath); err == nil {
		var config struct {
			Templates map[string]*TemplateMetadata `json:"templates"`
		}
		if err := json.Unmarshal(configData, &config); err == nil {
			metadataMap = config.Templates
			log.Printf("  ✓ 已加载提示词配置文件: templates.json")
		} else {
			log.Printf("  ⚠️  解析 templates.json 失败: %v", err)
		}
	}

	// 扫描目录中的所有 .txt 文件
	files, err := filepath.Glob(filepath.Join(dir, "*.txt"))
	if err != nil {
		return fmt.Errorf("扫描提示词目录失败: %w", err)
	}

	if len(files) == 0 {
		log.Printf("⚠️  提示词目录 %s 中没有找到 .txt 文件", dir)
		return nil
	}

	// 加载每个模板文件
	for _, file := range files {
		// 读取文件内容
		content, err := os.ReadFile(file)
		if err != nil {
			log.Printf("⚠️  读取提示词文件失败 %s: %v", file, err)
			continue
		}

		// 提取文件名（不含扩展名）作为模板名称
		fileName := filepath.Base(file)
		templateName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		// 创建模板对象
		template := &PromptTemplate{
			Name:    templateName,
			Content: string(content),
		}

		// 如果有配置元数据，填充显示名称和描述
		if metadata, exists := metadataMap[templateName]; exists {
			template.DisplayName = metadata.Name
			template.Description = metadata.Description
		} else {
			// 如果没有配置，使用模板名称作为默认显示名称
			template.DisplayName = map[string]string{
				"zh": templateName,
				"en": templateName,
			}
			template.Description = map[string]string{
				"zh": "",
				"en": "",
			}
		}

		// 存储模板
		pm.templates[templateName] = template

		log.Printf("  📄 加载提示词模板: %s (%s)", templateName, fileName)
	}

	return nil
}

// GetTemplate 获取指定名称的提示词模板
func (pm *PromptManager) GetTemplate(name string) (*PromptTemplate, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	template, exists := pm.templates[name]
	if !exists {
		return nil, fmt.Errorf("提示词模板不存在: %s", name)
	}

	return template, nil
}

// GetAllTemplateNames 获取所有模板名称列表
func (pm *PromptManager) GetAllTemplateNames() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	names := make([]string, 0, len(pm.templates))
	for name := range pm.templates {
		names = append(names, name)
	}

	return names
}

// GetAllTemplates 获取所有模板
func (pm *PromptManager) GetAllTemplates() []*PromptTemplate {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	templates := make([]*PromptTemplate, 0, len(pm.templates))
	for _, template := range pm.templates {
		templates = append(templates, template)
	}

	return templates
}

// ReloadTemplates 重新加载所有模板
func (pm *PromptManager) ReloadTemplates(dir string) error {
	pm.mu.Lock()
	pm.templates = make(map[string]*PromptTemplate)
	pm.mu.Unlock()

	return pm.LoadTemplates(dir)
}

// === 全局函数（供外部调用）===

// GetPromptTemplate 获取指定名称的提示词模板（全局函数）
func GetPromptTemplate(name string) (*PromptTemplate, error) {
	return globalPromptManager.GetTemplate(name)
}

// GetAllPromptTemplateNames 获取所有模板名称（全局函数）
func GetAllPromptTemplateNames() []string {
	return globalPromptManager.GetAllTemplateNames()
}

// GetAllPromptTemplates 获取所有模板（全局函数）
func GetAllPromptTemplates() []*PromptTemplate {
	return globalPromptManager.GetAllTemplates()
}

// ReloadPromptTemplates 重新加载所有模板（全局函数）
func ReloadPromptTemplates() error {
	return globalPromptManager.ReloadTemplates(promptsDir)
}
