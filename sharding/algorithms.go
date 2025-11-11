// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc 分片算法实现
// ///////////////////////////////////////////////////////////////////////////////
package sharding

import (
	"fmt"
	"strconv"
	"strings"
)

// ShardingAlgorithmType 分片算法类型
type ShardingAlgorithmType string

const (
	// AlgorithmTypeLong 基于 Long 类型的精确分片（取模）
	AlgorithmTypeLong ShardingAlgorithmType = "long"
	// AlgorithmTypeString 基于 String 类型的精确分片（hashCode取模）
	AlgorithmTypeString ShardingAlgorithmType = "string"
	// AlgorithmTypeMultiString 基于多字符串组合的分片
	AlgorithmTypeMultiString ShardingAlgorithmType = "multi_string"
)

// ShardingAlgorithm 分片算法接口
type ShardingAlgorithm interface {
	// CalculateShardIndex 计算分片索引
	// shardingValue: 分片键的值（可能是单个值或多个值的组合）
	// shardCount: 分片数量
	CalculateShardIndex(shardingValue interface{}, shardCount int) (int, error)
}

// LongShardingAlgorithm Long 类型精确分片算法
// 逻辑: id % shardCount
type LongShardingAlgorithm struct{}

func NewLongShardingAlgorithm() *LongShardingAlgorithm {
	return &LongShardingAlgorithm{}
}

func (a *LongShardingAlgorithm) CalculateShardIndex(shardingValue interface{}, shardCount int) (int, error) {
	var id int64

	switch v := shardingValue.(type) {
	case int:
		id = int64(v)
	case int32:
		id = int64(v)
	case int64:
		id = v
	case uint:
		id = int64(v)
	case uint32:
		id = int64(v)
	case uint64:
		id = int64(v)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string to int64: %w", err)
		}
		id = parsed
	default:
		return 0, fmt.Errorf("unsupported type for LongShardingAlgorithm: %T", shardingValue)
	}

	// 取模运算: (int) (id % shardCount)
	tableIndex := int(id % int64(shardCount))
	if tableIndex < 0 {
		tableIndex = -tableIndex
	}

	return tableIndex, nil
}

// StringShardingAlgorithm String 类型精确分片算法
// 逻辑: (column.hashCode() & Integer.MAX_VALUE) % shardCount
type StringShardingAlgorithm struct{}

func NewStringShardingAlgorithm() *StringShardingAlgorithm {
	return &StringShardingAlgorithm{}
}

func (a *StringShardingAlgorithm) CalculateShardIndex(shardingValue interface{}, shardCount int) (int, error) {
	var column string

	switch v := shardingValue.(type) {
	case string:
		column = v
	default:
		column = fmt.Sprintf("%v", shardingValue)
	}

	// 计算 hashCode 并取模: (column.hashCode() & Integer.MAX_VALUE) % shardCount
	hashCode := stringHashCode(column)
	tableSuffix := int((hashCode & 0x7FFFFFFF) % int32(shardCount)) // 0x7FFFFFFF = Integer.MAX_VALUE

	return tableSuffix, nil
}

// MultiStringShardingAlgorithm 多字符串组合分片算法
// 逻辑: 将多个列值用 "_" 连接，计算 hashCode，然后取模
type MultiStringShardingAlgorithm struct {
	separator string
}

func NewMultiStringShardingAlgorithm() *MultiStringShardingAlgorithm {
	return &MultiStringShardingAlgorithm{
		separator: "_", // 分隔符
	}
}

func (a *MultiStringShardingAlgorithm) CalculateShardIndex(shardingValue interface{}, shardCount int) (int, error) {
	var columnValues []string

	switch v := shardingValue.(type) {
	case []string:
		columnValues = v
	case []interface{}:
		columnValues = make([]string, len(v))
		for i, val := range v {
			columnValues[i] = fmt.Sprintf("%v", val)
		}
	case map[string]interface{}:
		// 按照键名排序，确保组合顺序的一致性
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		// 简单排序（实际应该使用更稳定的排序）
		for i := 0; i < len(keys)-1; i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		columnValues = make([]string, len(keys))
		for i, k := range keys {
			columnValues[i] = fmt.Sprintf("%v", v[k])
		}
	default:
		// 单个值也支持
		columnValues = []string{fmt.Sprintf("%v", shardingValue)}
	}

	if len(columnValues) == 0 {
		return 0, fmt.Errorf("empty column values for multi-string sharding")
	}

	// 组合列值: String.join(SEPARATOR, columnValues)
	combined := strings.Join(columnValues, a.separator)
	hashCode := stringHashCode(combined)

	// 计算分片索引: (hashCode & Integer.MAX_VALUE) % shardingCount
	shardIndex := int((hashCode & 0x7FFFFFFF) % int32(shardCount))

	return shardIndex, nil
}

// stringHashCode 计算字符串的 hashCode
// 实现公式: s[0]*31^(n-1) + s[1]*31^(n-2) + ... + s[n-1]
func stringHashCode(s string) int32 {
	var hash int32 = 0
	for _, c := range s {
		hash = hash*31 + c
	}
	return hash
}

// GetShardingAlgorithm 根据算法类型获取对应的分片算法
func GetShardingAlgorithm(algorithmType ShardingAlgorithmType) (ShardingAlgorithm, error) {
	switch algorithmType {
	case AlgorithmTypeLong:
		return NewLongShardingAlgorithm(), nil
	case AlgorithmTypeString:
		return NewStringShardingAlgorithm(), nil
	case AlgorithmTypeMultiString:
		return NewMultiStringShardingAlgorithm(), nil
	default:
		return nil, fmt.Errorf("unknown sharding algorithm type: %s", algorithmType)
	}
}
