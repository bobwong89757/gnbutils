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

// GetDBWithShardingKey 便捷函数：根据分片键获取数据库连接。
func GetDBWithShardingKey(shardingValue interface{}) (*gorm.DB, error) {
	return MShardingDB.GetDB(shardingValue)
}

// GetDBWithShardingKeyForTable 便捷函数：根据表名和分片键获取数据库连接（推荐使用）。
func GetDBWithShardingKeyForTable(tableName string, shardingValue interface{}) (*gorm.DB, error) {
	return MShardingDB.GetDBForTable(tableName, shardingValue)
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

// MustGetShardedDB 返回已设置分片表名的 DB session；路由失败时 panic（不降级）。
func MustGetShardedDB(tableName string, shardingValue interface{}) *gorm.DB {
	db, _, err := GetShardedDB(tableName, shardingValue)
	if err != nil {
		panic(fmt.Sprintf("sharding route failed for table %s: %v", tableName, err))
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

	// 生成表名
	fullTableName := fmt.Sprintf("%s_%d", tableName, tableIndex)

	cfg, err := config.ResolveDatabaseConfig(dbIndex)
	if err != nil {
		return nil, err
	}

	return &ShardInfo{
		DatabaseIndex: dbIndex,
		TableIndex:    tableIndex,
		DatabaseName:  cfg.Database,
		TableName:     fullTableName,
	}, nil
}
