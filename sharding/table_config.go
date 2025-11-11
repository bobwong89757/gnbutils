// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc 表级别的分片配置
// ///////////////////////////////////////////////////////////////////////////////
package sharding

// TableShardingConfig 单个表的分片配置
type TableShardingConfig struct {
	// 表名
	TableName string
	// 分片键字段名（如果为空，使用全局的 ShardingKey）
	ShardingKey string
	// 分片算法类型（如果为空，使用全局的 AlgorithmType）
	AlgorithmType string
	// 分片算法实例
	Algorithm ShardingAlgorithm
	// 该表的分表数量（如果为0，使用全局的 TableCountPerDB）
	TableCount int
}

// GetShardingKey 获取分片键，优先使用表级别的配置
func (t *TableShardingConfig) GetShardingKey(globalKey string) string {
	if t.ShardingKey != "" {
		return t.ShardingKey
	}
	return globalKey
}

// GetTableCount 获取分表数量，优先使用表级别的配置
func (t *TableShardingConfig) GetTableCount(globalCount int) int {
	if t.TableCount > 0 {
		return t.TableCount
	}
	return globalCount
}

// GetAlgorithm 获取分片算法，优先使用表级别的配置
func (t *TableShardingConfig) GetAlgorithm(globalAlgorithm ShardingAlgorithm) ShardingAlgorithm {
	if t.Algorithm != nil {
		return t.Algorithm
	}
	return globalAlgorithm
}

