package i18n

import (
	"fmt"

	"github.com/spf13/viper"
)

type I18n struct {
	viper *viper.Viper
}

// SetLang 设置语言，加载对应的语言配置文件
// lang: 语言代码，如 "zh-CN", "en-US" 等
func (i *I18n) SetLang(lang string) {
	i.viper = viper.New()

	// 设置配置文件
	i.viper.SetConfigName(lang)
	i.viper.SetConfigType("yaml")
	i.viper.AddConfigPath("conf/locales")
	i.viper.AddConfigPath("./conf/locales")

	// 读取配置文件
	if err := i.viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取语言文件失败 [%s]: %w", lang, err))
	}
}

// Get 获取翻译文本
// key: 翻译键名，支持嵌套访问（如 "user.welcome"）
func (i *I18n) Get(key string) interface{} {
	if i.viper == nil {
		return ""
	}

	value := i.viper.Get(key)
	if value == nil {
		return ""
	}
	return value
}

// GetString 获取字符串类型的翻译文本
func (i *I18n) GetString(key string) string {
	if i.viper == nil {
		return ""
	}
	return i.viper.GetString(key)
}

// GetStringMap 获取 map[string]interface{} 类型的翻译
func (i *I18n) GetStringMap(key string) map[string]interface{} {
	if i.viper == nil {
		return nil
	}
	return i.viper.GetStringMap(key)
}

// IsSet 检查翻译键是否存在
func (i *I18n) IsSet(key string) bool {
	if i.viper == nil {
		return false
	}
	return i.viper.IsSet(key)
}

// GetViper 获取底层的 viper 实例
func (i *I18n) GetViper() *viper.Viper {
	return i.viper
}

// AllTranslations 返回所有翻译
func (i *I18n) AllTranslations() map[string]interface{} {
	if i.viper == nil {
		return nil
	}
	return i.viper.AllSettings()
}

// ClearCache 清空缓存
func (i *I18n) ClearCache() {
	i.viper = nil
}
