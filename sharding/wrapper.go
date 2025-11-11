// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc 分库分表包装器 - 提供便捷的 API
// ///////////////////////////////////////////////////////////////////////////////
package sharding

import (
	"fmt"

	"gorm.io/gorm"
)

// ShardingDB 分库分表数据库包装器
type ShardingDB struct {
	manager *ShardingManager
}

// NewShardingDB 创建分库分表数据库包装器
func NewShardingDB() *ShardingDB {
	return &ShardingDB{
		manager: GetManager(),
	}
}

// GetDB 根据分片键获取数据库连接
// 使用示例: GetDB(userID) 或 GetDB("123")
// 注意：由于每个表可能有不同的算法，建议使用 GetDBForTable 方法
func (sdb *ShardingDB) GetDB(shardingValue interface{}) (*gorm.DB, error) {
	return sdb.manager.GetDB(shardingValue)
}

// GetDBForTable 根据表名和分片键获取数据库连接
// 使用表配置中的算法进行路由（推荐使用）
// 使用示例: GetDBForTable("users", userID)
func (sdb *ShardingDB) GetDBForTable(tableName string, shardingValue interface{}) (*gorm.DB, error) {
	return sdb.manager.GetDBForTable(tableName, shardingValue)
}

// GetDBByIndex 根据数据库索引获取数据库连接
func (sdb *ShardingDB) GetDBByIndex(dbIndex int) (*gorm.DB, error) {
	return sdb.manager.GetDBByIndex(dbIndex)
}

// GetAllDBs 获取所有数据库连接
func (sdb *ShardingDB) GetAllDBs() []*gorm.DB {
	return sdb.manager.GetAllDBs()
}

// GetDefaultDB 获取默认数据库连接（兼容原有代码，返回第一个数据库）
func (sdb *ShardingDB) GetDefaultDB() (*gorm.DB, error) {
	return sdb.manager.GetDBByIndex(0)
}

// 全局分库分表数据库实例
var MShardingDB = NewShardingDB()

// GetDBWithShardingKey 便捷函数：根据分片键获取数据库连接
// 注意：由于每个表可能有不同的算法，建议使用 GetDBWithShardingKeyForTable
func GetDBWithShardingKey(shardingValue interface{}) *gorm.DB {
	db, err := MShardingDB.GetDB(shardingValue)
	if err != nil {
		// 降级到默认数据库
		fmt.Printf("Warning: Failed to get sharding DB: %v, using default DB\n", err)
		db, _ = MShardingDB.GetDefaultDB()
	}
	return db
}

// GetDBWithShardingKeyForTable 便捷函数：根据表名和分片键获取数据库连接（推荐使用）
// 使用表配置中的算法进行路由
func GetDBWithShardingKeyForTable(tableName string, shardingValue interface{}) *gorm.DB {
	db, err := MShardingDB.GetDBForTable(tableName, shardingValue)
	if err != nil {
		// 降级到默认数据库
		fmt.Printf("Warning: Failed to get sharding DB for table %s: %v, using default DB\n", tableName, err)
		db, _ = MShardingDB.GetDefaultDB()
	}
	return db
}

// GetShardedDB 便捷函数：返回已设置表名的 DB session（最便捷）
// 自动计算分片表名并设置到 session 中
// 使用示例：db := sharding.GetShardedDB("relate_user", "test1013")
//
//	db.Where("open_id = ?", "test1013").Find(&user)
func GetShardedDB(tableName string, shardingValue interface{}) (*gorm.DB, string, error) {
	// 1. 计算分片信息
	shardInfo, err := CalculateShardForTable(tableName, shardingValue)
	if err != nil {
		return nil, "", fmt.Errorf("failed to calculate shard: %w", err)
	}

	// 2. 获取数据库连接
	db, err := MShardingDB.GetDBForTable(tableName, shardingValue)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get DB: %w", err)
	}

	// 3. 设置表名
	db = db.Table(shardInfo.TableName)

	return db, shardInfo.TableName, nil
}

// MustGetShardedDB 便捷函数：返回已设置表名的 DB session，失败时自动降级（最简洁）
// 不返回 error，失败时自动降级到默认数据库 + 原始表名
// 使用示例（链式调用）：
//
//	sharding.MustGetShardedDB("relate_user", "test1013").Where("open_id = ?", "test1013").Find(&user)
func MustGetShardedDB(tableName string, shardingValue interface{}) *gorm.DB {
	db, _, err := GetShardedDB(tableName, shardingValue)
	if err != nil {
		// 降级到默认数据库 + 原始表名（无分片）
		fmt.Printf("Warning: Failed to get sharded DB for table %s: %v, using default DB without sharding\n", tableName, err)
		defaultDB, err := MShardingDB.GetDefaultDB()
		if err != nil {
			// 如果连默认数据库都获取不到，返回一个空的 DB（会在实际查询时报错）
			return &gorm.DB{}
		}
		return defaultDB.Table(tableName)
	}
	return db
}

// CalculateShardForTable 计算指定表的分片位置
// 用于验证和调试，计算数据应该路由到哪个分表
// tableName: 表名，如 "users"
// shardingValue: 分片键的值
// 返回: 分片信息，包括数据库索引、表索引、数据库名、表名
func CalculateShardForTable(tableName string, shardingValue interface{}) (*ShardInfo, error) {
	manager := GetManager()

	if !manager.IsInitialized() {
		return nil, fmt.Errorf("sharding manager not initialized")
	}

	config := manager.GetConfig()
	if config == nil {
		return nil, fmt.Errorf("sharding config not found")
	}

	// 获取表的配置
	tableConfig, exists := config.TableConfigs[tableName]

	var algorithm ShardingAlgorithm
	var tableCount int

	if exists && tableConfig != nil {
		// 使用表级别的配置（必需）
		algorithm = tableConfig.Algorithm
		tableCount = tableConfig.TableCount
	} else {
		// 表配置不存在，返回错误
		return nil, fmt.Errorf("table config not found for table %s, each table must be configured in table_configs", tableName)
	}

	// 验证表配置
	if algorithm == nil {
		return nil, fmt.Errorf("algorithm is required for table %s", tableName)
	}
	if tableCount <= 0 {
		return nil, fmt.Errorf("table_count must be greater than 0 for table %s", tableName)
	}

	// 计算数据库索引
	dbIndex, err := algorithm.CalculateShardIndex(shardingValue, config.DatabaseCount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate database index: %w", err)
	}

	// 计算表索引
	tableIndex, err := algorithm.CalculateShardIndex(shardingValue, tableCount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate table index: %w", err)
	}

	// 生成数据库名
	dbName := config.DatabaseTemplate.Database
	if config.DatabaseCount > 1 {
		// 替换占位符（使用简单的字符串替换）
		placeholder := "{db_index}"
		for {
			idx := -1
			for i := 0; i <= len(dbName)-len(placeholder); i++ {
				if dbName[i:i+len(placeholder)] == placeholder {
					idx = i
					break
				}
			}
			if idx == -1 {
				break
			}
			dbName = dbName[:idx] + fmt.Sprintf("%d", dbIndex) + dbName[idx+len(placeholder):]
		}
	}

	// 生成表名
	fullTableName := fmt.Sprintf("%s_%d", tableName, tableIndex)

	return &ShardInfo{
		DatabaseIndex: dbIndex,
		TableIndex:    tableIndex,
		DatabaseName:  dbName,
		TableName:     fullTableName,
	}, nil
}
