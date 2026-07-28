package sharding

import (
	"fmt"

	"gorm.io/gorm"
)

// QueryAllTableShards 遍历指定逻辑表的全部分表（单库或多库均适用）。
// database_count=1 时只连一个 DB，但仍 fan-out 到 table_count 个分表。
func (sm *ShardingManager) QueryAllTableShards(tableName string, fn func(db *gorm.DB, shardTableName string) error) error {
	return sm.queryAllShards(tableName, func(db *gorm.DB, dbIndex int, shardTableName string) error {
		_ = dbIndex
		return fn(db, shardTableName)
	})
}

// QueryAllShards 遍历所有库 × 分表，回调携带 dbIndex。
func (sm *ShardingManager) QueryAllShards(tableName string, fn func(db *gorm.DB, dbIndex int, shardTableName string) error) error {
	return sm.queryAllShards(tableName, fn)
}

func (sm *ShardingManager) queryAllShards(tableName string, fn func(db *gorm.DB, dbIndex int, shardTableName string) error) error {
	if fn == nil {
		return fmt.Errorf("query callback is nil")
	}
	if !sm.initialized {
		return fmt.Errorf("sharding manager not initialized")
	}

	config := sm.GetConfig()
	if config == nil {
		return fmt.Errorf("sharding config not found")
	}

	tableConfig, exists := config.TableConfigs[tableName]
	if !exists || tableConfig == nil {
		return fmt.Errorf("table config not found for table %s", tableName)
	}
	if tableConfig.TableCount <= 0 {
		return fmt.Errorf("table_count must be greater than 0 for table %s", tableName)
	}

	for dbIndex := 0; dbIndex < config.DatabaseCount; dbIndex++ {
		db, err := sm.GetDBByIndex(dbIndex)
		if err != nil {
			return fmt.Errorf("get db index %d: %w", dbIndex, err)
		}
		for tableIndex := 0; tableIndex < tableConfig.TableCount; tableIndex++ {
			shardTableName := fmt.Sprintf("%s_%d", tableName, tableIndex)
			if err := fn(db, dbIndex, shardTableName); err != nil {
				return fmt.Errorf("query shard %s on db %d: %w", shardTableName, dbIndex, err)
			}
		}
	}
	return nil
}

// QueryAllTableShards 包级便捷函数。
func QueryAllTableShards(tableName string, fn func(db *gorm.DB, shardTableName string) error) error {
	return GetManager().QueryAllTableShards(tableName, fn)
}

// QueryAllShards 包级便捷函数。
func QueryAllShards(tableName string, fn func(db *gorm.DB, dbIndex int, shardTableName string) error) error {
	return GetManager().QueryAllShards(tableName, fn)
}
