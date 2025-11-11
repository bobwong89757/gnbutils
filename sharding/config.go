// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc Viper 配置加载器 - 从 YAML 配置文件加载 sharding 配置
// ///////////////////////////////////////////////////////////////////////////////
package sharding

import (
	"fmt"

	"github.com/spf13/viper"
)

// LoadConfigFromViper 从 Viper 实例加载 sharding 配置
// v: Viper 实例
// configKey: 配置键名，如 "sharding"，留空则从根读取
func LoadConfigFromViper(v *viper.Viper, configKey string) (*ShardingConfig, error) {
	var subViper *viper.Viper

	if configKey != "" {
		// 获取子配置
		subViper = v.Sub(configKey)
		if subViper == nil {
			return nil, fmt.Errorf("sharding config not found at key: %s", configKey)
		}
	} else {
		subViper = v
	}

	config := &ShardingConfig{}

	// 读取基本配置
	config.DatabaseCount = subViper.GetInt("database_count")
	if config.DatabaseCount <= 0 {
		config.DatabaseCount = 1 // 默认不分库
	}

	config.ShardingKey = subViper.GetString("sharding_key")
	config.TableCountPerDB = subViper.GetInt("table_count_per_db")
	config.PrimaryKeyGenerator = subViper.GetString("primary_key_generator")
	config.AlgorithmType = subViper.GetString("algorithm_type")

	// 读取数据库模板配置
	config.DatabaseTemplate = DatabaseConfig{
		Host:     subViper.GetString("database_template.host"),
		Port:     subViper.GetInt("database_template.port"),
		Username: subViper.GetString("database_template.username"),
		Password: subViper.GetString("database_template.password"),
		Database: subViper.GetString("database_template.database"),
		Charset:  subViper.GetString("database_template.charset"),
	}

	// 设置默认值
	if config.DatabaseTemplate.Charset == "" {
		config.DatabaseTemplate.Charset = "utf8mb4"
	}

	// 读取简单表列表（兼容旧格式）
	config.ShardingTables = subViper.GetStringSlice("sharding_tables")

	// 读取表级别的配置
	tableConfigsMap := subViper.GetStringMap("table_configs")
	if len(tableConfigsMap) > 0 {
		config.TableConfigs = make(map[string]*TableShardingConfig)

		for tableName := range tableConfigsMap {
			tableKey := fmt.Sprintf("table_configs.%s", tableName)

			// 读取表配置
			algorithmType := subViper.GetString(fmt.Sprintf("%s.algorithm_type", tableKey))
			shardingKey := subViper.GetString(fmt.Sprintf("%s.sharding_key", tableKey))
			tableCount := subViper.GetInt(fmt.Sprintf("%s.table_count", tableKey))

			// 验证必填字段
			if algorithmType == "" {
				return nil, fmt.Errorf("algorithm_type is required for table %s", tableName)
			}
			if shardingKey == "" {
				return nil, fmt.Errorf("sharding_key is required for table %s", tableName)
			}
			if tableCount <= 0 {
				return nil, fmt.Errorf("table_count must be greater than 0 for table %s", tableName)
			}

			// 创建算法实例
			algorithm, err := GetShardingAlgorithm(ShardingAlgorithmType(algorithmType))
			if err != nil {
				return nil, fmt.Errorf("invalid algorithm_type for table %s: %w", tableName, err)
			}

			// 创建表配置
			tableConfig := &TableShardingConfig{
				TableName:     tableName,
				ShardingKey:   shardingKey,
				AlgorithmType: algorithmType,
				Algorithm:     algorithm,
				TableCount:    tableCount,
			}

			config.TableConfigs[tableName] = tableConfig
		}
	}

	// 创建全局默认算法（如果配置了）
	if config.AlgorithmType != "" {
		algorithm, err := GetShardingAlgorithm(ShardingAlgorithmType(config.AlgorithmType))
		if err != nil {
			return nil, fmt.Errorf("invalid global algorithm_type: %w", err)
		}
		config.Algorithm = algorithm
	}

	return config, nil
}

// LoadConfigFromYAML 从 YAML 配置文件加载 sharding 配置
// configPath: 配置文件路径，如 "./config.yaml"
// configKey: 配置键名，如 "sharding"，留空则从根读取
func LoadConfigFromYAML(configPath, configKey string) (*ShardingConfig, error) {
	v := viper.New()

	// 设置配置文件
	v.SetConfigFile(configPath)

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return LoadConfigFromViper(v, configKey)
}

// InitFromViper 从 Viper 实例初始化全局 sharding 管理器
// v: Viper 实例
// configKey: 配置键名，如 "sharding"
func InitFromViper(v *viper.Viper, configKey string) error {
	config, err := LoadConfigFromViper(v, configKey)
	if err != nil {
		return err
	}

	manager := GetManager()
	return manager.Init(config)
}

// InitFromYAML 从 YAML 配置文件初始化全局 sharding 管理器
// configPath: 配置文件路径，如 "./config.yaml"
// configKey: 配置键名，如 "sharding"
func InitFromYAML(configPath, configKey string) error {
	config, err := LoadConfigFromYAML(configPath, configKey)
	if err != nil {
		return err
	}

	manager := GetManager()
	return manager.Init(config)
}
