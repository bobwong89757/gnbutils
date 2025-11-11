// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc 分片计算工具 - 用于计算数据应该路由到哪个分表
// ///////////////////////////////////////////////////////////////////////////////
package sharding

import (
	"fmt"
)

// ShardCalculator 分片计算器
// 用于根据分片键值计算应该路由到哪个库的哪个表
type ShardCalculator struct {
	algorithm       ShardingAlgorithm
	databaseCount   int
	tableCountPerDB int
}

// NewShardCalculator 创建分片计算器
func NewShardCalculator(algorithmType ShardingAlgorithmType, databaseCount, tableCountPerDB int) (*ShardCalculator, error) {
	algorithm, err := GetShardingAlgorithm(algorithmType)
	if err != nil {
		return nil, err
	}

	return &ShardCalculator{
		algorithm:       algorithm,
		databaseCount:   databaseCount,
		tableCountPerDB: tableCountPerDB,
	}, nil
}

// CalculateShard 计算分片位置
// 返回: (databaseIndex, tableIndex, tableName)
func (sc *ShardCalculator) CalculateShard(shardingValue interface{}) (dbIndex int, tableIndex int, tableName string, err error) {
	// 1. 计算数据库索引（分库）
	dbIndex, err = sc.algorithm.CalculateShardIndex(shardingValue, sc.databaseCount)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to calculate database index: %w", err)
	}

	// 2. 计算表索引（分表）
	tableIndex, err = sc.algorithm.CalculateShardIndex(shardingValue, sc.tableCountPerDB)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to calculate table index: %w", err)
	}

	// 3. 生成表名（格式：table_name_tableIndex）
	// 注意：这里需要知道原始表名，所以 tableName 需要外部传入
	// 或者返回 tableIndex，让调用者自己拼接表名

	return dbIndex, tableIndex, "", nil
}

// CalculateTableName 计算完整的表名
// tableName: 原始表名，如 "users"
// 返回: 分表后的表名，如 "users_1"
func (sc *ShardCalculator) CalculateTableName(originalTableName string, shardingValue interface{}) (string, error) {
	_, tableIndex, _, err := sc.CalculateShard(shardingValue)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s_%d", originalTableName, tableIndex), nil
}

// CalculateDatabaseName 计算数据库名
// databaseTemplate: 数据库名模板，如 "nbgame_{db_index}"
// 返回: 实际的数据库名，如 "nbgame_1"
func (sc *ShardCalculator) CalculateDatabaseName(databaseTemplate string, shardingValue interface{}) (string, error) {
	dbIndex, _, _, err := sc.CalculateShard(shardingValue)
	if err != nil {
		return "", err
	}

	// 替换占位符
	result := databaseTemplate
	placeholder := "{db_index}"
	for {
		idx := -1
		for i := 0; i <= len(result)-len(placeholder); i++ {
			if result[i:i+len(placeholder)] == placeholder {
				idx = i
				break
			}
		}
		if idx == -1 {
			break
		}
		result = result[:idx] + fmt.Sprintf("%d", dbIndex) + result[idx+len(placeholder):]
	}

	return result, nil
}

// GetShardInfo 获取完整的分片信息
// 用于调试和验证分片计算结果
type ShardInfo struct {
	DatabaseIndex int    // 数据库索引
	TableIndex    int    // 表索引
	DatabaseName  string // 数据库名
	TableName     string // 完整表名（包含后缀）
}

func (sc *ShardCalculator) GetShardInfo(originalTableName, databaseTemplate string, shardingValue interface{}) (*ShardInfo, error) {
	dbIndex, tableIndex, _, err := sc.CalculateShard(shardingValue)
	if err != nil {
		return nil, err
	}

	dbName, err := sc.CalculateDatabaseName(databaseTemplate, shardingValue)
	if err != nil {
		return nil, err
	}

	tableName, err := sc.CalculateTableName(originalTableName, shardingValue)
	if err != nil {
		return nil, err
	}

	return &ShardInfo{
		DatabaseIndex: dbIndex,
		TableIndex:    tableIndex,
		DatabaseName:  dbName,
		TableName:     tableName,
	}, nil
}

// 全局便捷函数：根据配置计算分片位置
// 这个函数可以从 helpers.MShardingDB 获取配置后计算
func CalculateShardLocation(algorithmType ShardingAlgorithmType, databaseCount, tableCountPerDB int, originalTableName, databaseTemplate string, shardingValue interface{}) (*ShardInfo, error) {
	calculator, err := NewShardCalculator(algorithmType, databaseCount, tableCountPerDB)
	if err != nil {
		return nil, err
	}

	return calculator.GetShardInfo(originalTableName, databaseTemplate, shardingValue)
}
