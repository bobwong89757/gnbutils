package yaml

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type YamlUtil struct {
	viper *viper.Viper
}

// InitConfig 初始化配置文件
// configPath: 配置文件路径，如 "./config.yaml" 或 "./config.yml"
func (c *YamlUtil) InitConfig(configPath string) {
	c.viper = viper.New()

	// 解析文件路径
	dir := filepath.Dir(configPath)
	filename := filepath.Base(configPath)
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// 设置配置文件
	c.viper.SetConfigName(name)
	c.viper.SetConfigType(strings.TrimPrefix(ext, "."))
	c.viper.AddConfigPath(dir)

	// 读取配置文件
	if err := c.viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("读取配置文件失败: %w", err))
	}
}

// Get 获取指定 key 的配置，返回 map[string]string
// 兼容旧的 API，如果值不是 map[string]string 类型会返回 nil
func (c *YamlUtil) Get(key string) map[string]string {
	if c.viper == nil {
		return nil
	}

	// 获取配置项
	value := c.viper.Get(key)
	if value == nil {
		return nil
	}

	// 尝试转换为 map[string]string
	result := make(map[string]string)

	// 如果是 map[string]interface{}，转换为 map[string]string
	if m, ok := value.(map[string]interface{}); ok {
		for k, v := range m {
			result[k] = fmt.Sprintf("%v", v)
		}
		return result
	}

	// 如果已经是 map[string]string，直接返回
	if m, ok := value.(map[string]string); ok {
		return m
	}

	return nil
}

// GetViper 获取底层的 viper 实例，用于更高级的配置操作
func (c *YamlUtil) GetViper() *viper.Viper {
	return c.viper
}

// GetString 获取字符串值
func (c *YamlUtil) GetString(key string) string {
	if c.viper == nil {
		return ""
	}
	return c.viper.GetString(key)
}

// GetInt 获取整数值
func (c *YamlUtil) GetInt(key string) int {
	if c.viper == nil {
		return 0
	}
	return c.viper.GetInt(key)
}

// GetBool 获取布尔值
func (c *YamlUtil) GetBool(key string) bool {
	if c.viper == nil {
		return false
	}
	return c.viper.GetBool(key)
}

// GetStringMap 获取 map[string]interface{}
func (c *YamlUtil) GetStringMap(key string) map[string]interface{} {
	if c.viper == nil {
		return nil
	}
	return c.viper.GetStringMap(key)
}

// GetStringMapString 获取 map[string]string
func (c *YamlUtil) GetStringMapString(key string) map[string]string {
	if c.viper == nil {
		return nil
	}
	return c.viper.GetStringMapString(key)
}

// IsSet 检查配置项是否存在
func (c *YamlUtil) IsSet(key string) bool {
	if c.viper == nil {
		return false
	}
	return c.viper.IsSet(key)
}

// AllSettings 返回所有配置
func (c *YamlUtil) AllSettings() map[string]interface{} {
	if c.viper == nil {
		return nil
	}
	return c.viper.AllSettings()
}

// ClearCache 清空缓存（保留方法以兼容旧 API）
func (c *YamlUtil) ClearCache() {
	c.viper = nil
}
