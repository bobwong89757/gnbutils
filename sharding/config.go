// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc Viper 配置加载器 - 从 YAML 配置文件加载 sharding 配置
// ///////////////////////////////////////////////////////////////////////////////
package sharding

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// IsShardingEnabled 是否启用分库分表。
// 只认根配置 enable_sharding（与 mysql 并列的开关）：
//
//	true  → 读取 sharding.databases，不使用 mysql
//	false / 未配置 → 跳过 sharding，由调用方使用 mysql
func IsShardingEnabled(v *viper.Viper) bool {
	if v == nil {
		return false
	}
	return v.GetBool("enable_sharding")
}

// LoadConfigFromViperWithMysql 已废弃：会把 mysql 和 sharding 混在一起。
// 新代码请用 enable_sharding + LoadConfigFromViper（只读 sharding.databases）。
//
// 配置示例:
//
//	mysql:
//	  host: 127.0.0.1
//	  port: 3306
//	  username: root
//	  password: password
//	  database: myapp
//	sharding:
//	  database_count: 1
//	  table_configs:
//	    users:
//	      algorithm_type: long
//	      sharding_key: user_id
//	      table_count: 4
func LoadConfigFromViperWithMysql(v *viper.Viper, shardingKey, mysqlKey string) (*ShardingConfig, error) {
	// 1. 读取 mysql 配置
	if !v.IsSet(mysqlKey) {
		return nil, fmt.Errorf("mysql config not found at key: %s", mysqlKey)
	}

	mysqlHost := v.GetString(fmt.Sprintf("%s.host", mysqlKey))
	mysqlPort := v.GetInt(fmt.Sprintf("%s.port", mysqlKey))
	mysqlUsername := v.GetString(fmt.Sprintf("%s.username", mysqlKey))
	mysqlPassword := v.GetString(fmt.Sprintf("%s.password", mysqlKey))
	mysqlDatabase := v.GetString(fmt.Sprintf("%s.database", mysqlKey))

	// 验证必填字段
	if mysqlHost == "" {
		return nil, fmt.Errorf("mysql.host is required")
	}
	if mysqlPort == 0 {
		mysqlPort = 3306 // 默认端口
	}
	if mysqlUsername == "" {
		return nil, fmt.Errorf("mysql.username is required")
	}
	if mysqlDatabase == "" {
		return nil, fmt.Errorf("mysql.database is required")
	}

	// 2. 读取 sharding 配置
	if !v.IsSet(shardingKey) {
		return nil, fmt.Errorf("sharding config not found at key: %s", shardingKey)
	}

	subViper := v.Sub(shardingKey)
	if subViper == nil {
		return nil, fmt.Errorf("failed to get sharding config")
	}

	config := &ShardingConfig{}

	countFromYAML := subViper.GetInt("database_count")

	config.PrimaryKeyGenerator = subViper.GetString("primary_key_generator")
	if config.PrimaryKeyGenerator == "" {
		config.PrimaryKeyGenerator = "snowflake"
	}
	config.Snowflake = loadSnowflakeConfig(subViper, v)

	// 3. 设置数据库模板配置（从 mysql 配置中读取）
	config.DatabaseTemplate = DatabaseConfig{
		Host:     mysqlHost,
		Port:     mysqlPort,
		Username: mysqlUsername,
		Password: mysqlPassword,
		Database: mysqlDatabase,
		Charset:  "utf8mb4",
	}

	databases, err := loadDatabaseOverrides(v, shardingKey)
	if err != nil {
		return nil, err
	}
	config.Databases = databases
	if err := normalizeDatabaseCount(config, countFromYAML); err != nil {
		return nil, err
	}

	// 多分库且模板库名无占位符时，自动追加 _{db_index}（作为未在 databases 中写明 database 时的默认值）
	if config.DatabaseCount > 1 && !strings.Contains(config.DatabaseTemplate.Database, "{db_index}") {
		config.DatabaseTemplate.Database = mysqlDatabase + "_{db_index}"
	}

	// 4. 读取表级别的配置
	tableConfigsMap := subViper.GetStringMap("table_configs")
	if len(tableConfigsMap) == 0 {
		return nil, fmt.Errorf("table_configs is required in sharding config")
	}

	config.TableConfigs = make(map[string]*TableShardingConfig)

	for tableName := range tableConfigsMap {
		tableKey := fmt.Sprintf("table_configs.%s", tableName)

		// 读取表配置
		algorithmType := subViper.GetString(fmt.Sprintf("%s.algorithm_type", tableKey))
		shardingKeyField := subViper.GetString(fmt.Sprintf("%s.sharding_key", tableKey))
		tableCount := subViper.GetInt(fmt.Sprintf("%s.table_count", tableKey))

		// 验证必填字段
		if algorithmType == "" {
			return nil, fmt.Errorf("algorithm_type is required for table %s", tableName)
		}
		if shardingKeyField == "" {
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
			ShardingKey:   shardingKeyField,
			AlgorithmType: algorithmType,
			Algorithm:     algorithm,
			TableCount:    tableCount,
		}

		config.TableConfigs[tableName] = tableConfig
	}

	return config, nil
}

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

	countFromYAML := subViper.GetInt("database_count")

	config.ShardingKey = subViper.GetString("sharding_key")
	config.TableCountPerDB = subViper.GetInt("table_count_per_db")
	config.PrimaryKeyGenerator = subViper.GetString("primary_key_generator")
	if config.PrimaryKeyGenerator == "" {
		config.PrimaryKeyGenerator = "snowflake"
	}
	config.Snowflake = loadSnowflakeConfig(subViper, v)
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

	if config.DatabaseTemplate.Charset == "" {
		config.DatabaseTemplate.Charset = "utf8mb4"
	}

	databases, err := loadDatabaseOverrides(v, configKey)
	if err != nil {
		return nil, err
	}
	config.Databases = databases
	if err := normalizeDatabaseCount(config, countFromYAML); err != nil {
		return nil, err
	}
	if len(config.Databases) == 0 {
		return nil, fmt.Errorf("sharding.databases is required when sharding is enabled (do not use mysql for shard connections)")
	}
	for i := 0; i < config.DatabaseCount; i++ {
		if _, err := config.ResolveDatabaseConfig(i); err != nil {
			return nil, fmt.Errorf("sharding.databases[%d]: %w", i, err)
		}
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

// loadSnowflakeConfig 读取 sharding.snowflake；未配置时回退根级 snowflake.machine-id / datacenter-id（对齐 Java game-server）。
func loadSnowflakeConfig(subViper, rootViper *viper.Viper) SnowflakeConfig {
	cfg := SnowflakeConfig{
		WorkerID:                    subViper.GetInt64("snowflake.worker_id"),
		DatacenterID:                subViper.GetInt64("snowflake.datacenter_id"),
		MaxTolerateTimeDifferenceMs: subViper.GetInt64("snowflake.max_tolerate_time_difference_ms"),
	}
	if rootViper != nil {
		if cfg.WorkerID == 0 {
			if id := rootViper.GetInt64("snowflake.machine-id"); id != 0 {
				cfg.WorkerID = id
			} else if id := rootViper.GetInt64("snowflake.worker_id"); id != 0 {
				cfg.WorkerID = id
			}
		}
		if cfg.DatacenterID == 0 {
			if id := rootViper.GetInt64("snowflake.datacenter-id"); id != 0 {
				cfg.DatacenterID = id
			} else if id := rootViper.GetInt64("snowflake.datacenter_id"); id != 0 {
				cfg.DatacenterID = id
			}
		}
	}
	return cfg
}

func loadDatabaseOverrides(v *viper.Viper, configKey string) ([]DatabaseConfig, error) {
	key := "databases"
	if configKey != "" {
		key = configKey + ".databases"
	}
	if !v.IsSet(key) {
		return nil, nil
	}
	var list []DatabaseConfig
	if err := v.UnmarshalKey(key, &list); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", key, err)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s is empty", key)
	}
	return list, nil
}

func normalizeDatabaseCount(config *ShardingConfig, countFromYAML int) error {
	if len(config.Databases) > 0 {
		n := len(config.Databases)
		if countFromYAML <= 0 {
			config.DatabaseCount = n
			return nil
		}
		if countFromYAML != n {
			return fmt.Errorf("sharding.database_count (%d) must equal len(sharding.databases) (%d)", countFromYAML, n)
		}
		config.DatabaseCount = n
		return nil
	}
	config.DatabaseCount = countFromYAML
	if config.DatabaseCount <= 0 {
		config.DatabaseCount = 1
	}
	return nil
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
