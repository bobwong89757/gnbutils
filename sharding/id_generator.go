package sharding

import "fmt"

// IDGenerator 主键生成器。
type IDGenerator interface {
	NextID() (int64, error)
	NextUint64() (uint64, error)
}

var globalIDGenerator IDGenerator

func setGlobalIDGenerator(gen IDGenerator) {
	globalIDGenerator = gen
}

// GetIDGenerator 返回当前主键生成器（可能为 nil）。
func GetIDGenerator() IDGenerator {
	return globalIDGenerator
}

// NextID 生成下一个主键 ID。
func NextID() (int64, error) {
	if globalIDGenerator == nil {
		return 0, fmt.Errorf("primary key generator not initialized")
	}
	return globalIDGenerator.NextID()
}

// NextUint64 生成下一个 uint64 主键 ID。
func NextUint64() (uint64, error) {
	if globalIDGenerator == nil {
		return 0, fmt.Errorf("primary key generator not initialized")
	}
	return globalIDGenerator.NextUint64()
}

// MustNextID 生成 ID，失败 panic。
func MustNextID() int64 {
	id, err := NextID()
	if err != nil {
		panic(err)
	}
	return id
}

// MustNextUint64 生成 uint64 ID，失败 panic。
func MustNextUint64() uint64 {
	id, err := NextUint64()
	if err != nil {
		panic(err)
	}
	return id
}

func initIDGenerator(config *ShardingConfig) error {
	switch config.PrimaryKeyGenerator {
	case "", "snowflake":
		gen, err := NewSnowflakeFromConfig(config.Snowflake)
		if err != nil {
			return fmt.Errorf("init snowflake generator: %w", err)
		}
		setGlobalIDGenerator(gen)
		return nil
	case "sequence":
		return fmt.Errorf("primary_key_generator sequence is not implemented")
	case "custom":
		return fmt.Errorf("primary_key_generator custom is not implemented")
	default:
		return fmt.Errorf("unsupported primary_key_generator: %s", config.PrimaryKeyGenerator)
	}
}
