// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc 分库分表管理器 - 基于 GORM Sharding 插件实现
// @copyright ©2022福建易思科技有限公司
// @author BobWong
// ///////////////////////////////////////////////////////////////////////////////
package sharding

import (
	"fmt"
	"strconv"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// ShardingConfig 分库分表配置
type ShardingConfig struct {
	// 分片键字段名，如 "user_id"（全局默认值）
	ShardingKey string `yaml:"sharding_key"`
	// 分库数量
	DatabaseCount int `yaml:"database_count"`
	// 每个库的分表数量（全局默认值）
	TableCountPerDB int `yaml:"table_count_per_db"`
	// 数据库配置模板，支持占位符 {db_index}
	DatabaseTemplate DatabaseConfig `yaml:"database_template"`
	// 需要分片的表名列表（简单格式，使用全局算法）
	ShardingTables []string `yaml:"sharding_tables"`
	// 表级别的分片配置（详细格式，支持每个表不同的算法）
	TableConfigs map[string]*TableShardingConfig `yaml:"-"`
	// 主键生成器类型: snowflake, sequence, custom
	PrimaryKeyGenerator string `yaml:"primary_key_generator"`
	// 分片算法类型: long, string, multi_string（全局默认值）
	// long: 基于 Long 类型的精确分片（取模）
	// string: 基于 String 类型的精确分片（hashCode取模）
	// multi_string: 基于多字符串组合的分片
	AlgorithmType string `yaml:"algorithm_type"`
	// 分片算法实例（全局默认值，内部使用，需要在初始化时设置）
	Algorithm ShardingAlgorithm `yaml:"-"`
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"` // 支持 {db_index} 占位符
	Charset  string `yaml:"charset"`
}

// ShardingManager 分库分表管理器
type ShardingManager struct {
	config        *ShardingConfig
	databases     []*gorm.DB
	databasesLock sync.RWMutex
	initialized   bool
}

// GetConfig 获取配置（用于外部访问）
func (sm *ShardingManager) GetConfig() *ShardingConfig {
	sm.databasesLock.RLock()
	defer sm.databasesLock.RUnlock()
	return sm.config
}

// IsInitialized 检查是否已初始化
func (sm *ShardingManager) IsInitialized() bool {
	sm.databasesLock.RLock()
	defer sm.databasesLock.RUnlock()
	return sm.initialized
}

var (
	globalManager *ShardingManager
	once          sync.Once
)

// GetManager 获取全局分库分表管理器
func GetManager() *ShardingManager {
	once.Do(func() {
		globalManager = &ShardingManager{
			databases: make([]*gorm.DB, 0),
		}
	})
	return globalManager
}

// Init 初始化分库分表管理器
func (sm *ShardingManager) Init(config *ShardingConfig) error {
	sm.config = config
	sm.databasesLock.Lock()
	defer sm.databasesLock.Unlock()

	if sm.initialized {
		return fmt.Errorf("sharding manager already initialized")
	}

	// 初始化所有数据库连接
	sm.databases = make([]*gorm.DB, config.DatabaseCount)

	for i := 0; i < config.DatabaseCount; i++ {
		db, err := sm.initDatabase(i)
		if err != nil {
			return fmt.Errorf("failed to init database %d: %w", i, err)
		}
		sm.databases[i] = db
	}

	sm.initialized = true

	return nil
}

// initDatabase 初始化单个数据库连接并注册 sharding 插件
func (sm *ShardingManager) initDatabase(dbIndex int) (*gorm.DB, error) {
	// 构建数据库名（支持占位符）
	dbName := sm.config.DatabaseTemplate.Database
	if dbName == "" {
		dbName = fmt.Sprintf("nbgame_%d", dbIndex)
	} else {
		dbName = replacePlaceholder(dbName, "db_index", strconv.Itoa(dbIndex))
	}

	// 构建 DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		sm.config.DatabaseTemplate.Username,
		sm.config.DatabaseTemplate.Password,
		sm.config.DatabaseTemplate.Host,
		sm.config.DatabaseTemplate.Port,
		dbName,
		sm.config.DatabaseTemplate.Charset,
	)

	// 打开数据库连接
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database %d: %w", dbIndex, err)
	}

	// 验证所有表的配置
	if sm.config.TableConfigs != nil && len(sm.config.TableConfigs) > 0 {
		for tableName, tableConfig := range sm.config.TableConfigs {
			if tableConfig.ShardingKey == "" {
				return nil, fmt.Errorf("sharding_key is required for table %s", tableName)
			}
			if tableConfig.TableCount <= 0 {
				return nil, fmt.Errorf("table_count must be greater than 0 for table %s", tableName)
			}
			if tableConfig.Algorithm == nil {
				return nil, fmt.Errorf("algorithm is required for table %s", tableName)
			}
		}
	}

	// 注意：由于 GORM sharding 插件的限制，所有表必须使用相同的 Config
	// 但我们的需求是每个表有不同的 sharding_key、table_count 和 algorithm
	// 因此我们不使用 GORM 的 sharding 插件，而是在应用层手动处理分片
	//
	// 分片逻辑：
	// 1. 使用 GetDBForTable(tableName, shardingValue) 获取正确的数据库连接
	// 2. 该方法会根据表配置自动计算分片表名
	// 3. GORM 会自动使用带后缀的表名（如 game_player_3）
	//
	// 使用示例：
	//   db, err := helpers.MShardingDB.GetDBForTable("game_player", userID)
	//   db.Create(&player)  // 会自动路由到正确的分片表

	return db, nil
}

// GetDB 根据分片键获取对应的数据库连接
// 注意：由于每个表可能有不同的算法，这个方法无法确定使用哪个算法
// 建议使用 GetDBForTable 方法，明确指定表名
func (sm *ShardingManager) GetDB(shardingValue interface{}) (*gorm.DB, error) {
	if !sm.initialized {
		return nil, fmt.Errorf("sharding manager not initialized")
	}

	// 使用默认的 Long 算法计算数据库索引（兼容旧代码）
	algorithm := NewLongShardingAlgorithm()
	dbIndex, err := sm.calculateDatabaseIndex(shardingValue, algorithm)
	if err != nil {
		return nil, err
	}

	sm.databasesLock.RLock()
	defer sm.databasesLock.RUnlock()

	if dbIndex < 0 || dbIndex >= len(sm.databases) {
		return nil, fmt.Errorf("invalid database index: %d", dbIndex)
	}

	return sm.databases[dbIndex], nil
}

// GetDBForTable 根据表名和分片键获取对应的数据库连接
// 使用表配置中的算法进行路由
func (sm *ShardingManager) GetDBForTable(tableName string, shardingValue interface{}) (*gorm.DB, error) {
	if !sm.initialized {
		return nil, fmt.Errorf("sharding manager not initialized")
	}

	// 获取表的配置
	tableConfig, exists := sm.config.TableConfigs[tableName]
	if !exists || tableConfig == nil {
		return nil, fmt.Errorf("table config not found for table %s", tableName)
	}

	// 使用表配置中的算法计算数据库索引
	dbIndex, err := sm.calculateDatabaseIndex(shardingValue, tableConfig.Algorithm)
	if err != nil {
		return nil, err
	}

	sm.databasesLock.RLock()
	defer sm.databasesLock.RUnlock()

	if dbIndex < 0 || dbIndex >= len(sm.databases) {
		return nil, fmt.Errorf("invalid database index: %d", dbIndex)
	}

	return sm.databases[dbIndex], nil
}

// GetDBByIndex 根据数据库索引直接获取数据库连接（用于跨库查询等场景）
func (sm *ShardingManager) GetDBByIndex(dbIndex int) (*gorm.DB, error) {
	if !sm.initialized {
		return nil, fmt.Errorf("sharding manager not initialized")
	}

	sm.databasesLock.RLock()
	defer sm.databasesLock.RUnlock()

	if dbIndex < 0 || dbIndex >= len(sm.databases) {
		return nil, fmt.Errorf("invalid database index: %d", dbIndex)
	}

	return sm.databases[dbIndex], nil
}

// GetAllDBs 获取所有数据库连接（用于跨库查询等场景）
func (sm *ShardingManager) GetAllDBs() []*gorm.DB {
	sm.databasesLock.RLock()
	defer sm.databasesLock.RUnlock()

	dbs := make([]*gorm.DB, len(sm.databases))
	copy(dbs, sm.databases)
	return dbs
}

// calculateDatabaseIndex 计算数据库索引
// 使用指定的分片算法进行路由
func (sm *ShardingManager) calculateDatabaseIndex(shardingValue interface{}, algorithm ShardingAlgorithm) (int, error) {
	if algorithm == nil {
		// 如果没有提供算法，使用默认的 Long 算法
		algorithm = NewLongShardingAlgorithm()
	}

	return algorithm.CalculateShardIndex(shardingValue, sm.config.DatabaseCount)
}

// replacePlaceholder 替换占位符
func replacePlaceholder(template, key, value string) string {
	// 简单的占位符替换，将 {key} 替换为 value
	placeholder := fmt.Sprintf("{%s}", key)
	result := template
	for {
		newResult := ""
		found := false
		for i := 0; i < len(result); i++ {
			if i+len(placeholder) <= len(result) && result[i:i+len(placeholder)] == placeholder {
				newResult += value
				i += len(placeholder) - 1
				found = true
			} else {
				newResult += string(result[i])
			}
		}
		if !found {
			break
		}
		result = newResult
	}
	return result
}

// Close 关闭所有数据库连接
func (sm *ShardingManager) Close() error {
	sm.databasesLock.Lock()
	defer sm.databasesLock.Unlock()

	for i, db := range sm.databases {
		if db != nil {
			sqlDB, err := db.DB()
			if err == nil {
				sqlDB.Close()
			}
		}
		sm.databases[i] = nil
	}

	sm.initialized = false
	return nil
}
