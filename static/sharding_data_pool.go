package static

import (
	"fmt"

	"github.com/bobwong89757/gnbutils/sharding"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// ShardingDataPool 分库分表数据库连接池
type ShardingDataPool struct {
	manager *sharding.ShardingManager
}

// InitShardingWithViper
//
//	@Description: 使用 Viper 实例初始化分库分表
//	@receiver d
//	@param v Viper 实例
//	@param configKey 配置键名，如 "sharding"
func (d *ShardingDataPool) InitShardingWithViper(v *viper.Viper, configKey string) {
	err := sharding.InitFromViper(v, configKey)
	if err != nil {
		fmt.Println("could not init sharding: " + err.Error())
		panic("sharding init error")
	}
	d.manager = sharding.GetManager()
}

// InitShardingWithConfig
//
//	@Description: 使用配置 map 初始化分库分表（从 Viper 读取）
//	@receiver d
//	@param v Viper 实例
func (d *ShardingDataPool) InitShardingWithConfig(v *viper.Viper) {
	d.InitShardingWithViper(v, "sharding")
}

// InitShardingFromYAML
//
//	@Description: 从 YAML 文件初始化分库分表
//	@receiver d
//	@param configPath 配置文件路径
//	@param configKey 配置键名，默认 "sharding"
func (d *ShardingDataPool) InitShardingFromYAML(configPath string, configKey string) {
	if configKey == "" {
		configKey = "sharding"
	}
	err := sharding.InitFromYAML(configPath, configKey)
	if err != nil {
		fmt.Println("could not init sharding: " + err.Error())
		panic("sharding init error")
	}
	d.manager = sharding.GetManager()
}

// GetDB
//
//	@Description: 根据分片键获取数据库连接（使用默认算法）
//	@receiver d
//	@param shardingValue 分片键的值
//	@return *gorm.DB
func (d *ShardingDataPool) GetDB(shardingValue interface{}) *gorm.DB {
	db, err := d.manager.GetDB(shardingValue)
	if err != nil {
		fmt.Printf("Warning: could not get sharding DB: %v, trying default DB\n", err)
		db, _ = d.manager.GetDBByIndex(0)
	}
	return db
}

// GetDBForTable
//
//	@Description: 根据表名和分片键获取数据库连接（推荐使用）
//	@receiver d
//	@param tableName 表名
//	@param shardingValue 分片键的值
//	@return *gorm.DB
func (d *ShardingDataPool) GetDBForTable(tableName string, shardingValue interface{}) *gorm.DB {
	db, err := d.manager.GetDBForTable(tableName, shardingValue)
	if err != nil {
		fmt.Printf("Warning: could not get sharding DB for table %s: %v, trying default DB\n", tableName, err)
		db, _ = d.manager.GetDBByIndex(0)
	}
	return db
}

// GetDBByIndex
//
//	@Description: 根据数据库索引获取数据库连接
//	@receiver d
//	@param dbIndex 数据库索引
//	@return *gorm.DB
func (d *ShardingDataPool) GetDBByIndex(dbIndex int) *gorm.DB {
	db, err := d.manager.GetDBByIndex(dbIndex)
	if err != nil {
		fmt.Printf("Warning: could not get DB by index %d: %v\n", dbIndex, err)
		return nil
	}
	return db
}

// GetDefaultDB
//
//	@Description: 获取默认数据库连接（第一个数据库）
//	@receiver d
//	@return *gorm.DB
func (d *ShardingDataPool) GetDefaultDB() *gorm.DB {
	return d.GetDBByIndex(0)
}

// GetAllDBs
//
//	@Description: 获取所有数据库连接（用于跨库查询）
//	@receiver d
//	@return []*gorm.DB
func (d *ShardingDataPool) GetAllDBs() []*gorm.DB {
	return d.manager.GetAllDBs()
}

// GetShardedDB
//
//	@Description: 获取已设置表名的 DB session（最便捷）
//	@receiver d
//	@param tableName 表名
//	@param shardingValue 分片键的值
//	@return *gorm.DB 已设置表名的 DB session
//	@return string 完整的分片表名（如 users_1）
func (d *ShardingDataPool) GetShardedDB(tableName string, shardingValue interface{}) (*gorm.DB, string) {
	db, tableFullName, err := sharding.GetShardedDB(tableName, shardingValue)
	if err != nil {
		fmt.Printf("Warning: could not get sharded DB for table %s: %v, using default DB\n", tableName, err)
		defaultDB := d.GetDefaultDB()
		if defaultDB != nil {
			return defaultDB.Table(tableName), tableName
		}
		return &gorm.DB{}, tableName
	}
	return db, tableFullName
}

// MustGetShardedDB
//
//	@Description: 获取已设置表名的 DB session，失败时自动降级（最简洁）
//	@receiver d
//	@param tableName 表名
//	@param shardingValue 分片键的值
//	@return *gorm.DB
func (d *ShardingDataPool) MustGetShardedDB(tableName string, shardingValue interface{}) *gorm.DB {
	db, _ := d.GetShardedDB(tableName, shardingValue)
	return db
}

// CalculateShard
//
//	@Description: 计算分片位置（用于调试）
//	@receiver d
//	@param tableName 表名
//	@param shardingValue 分片键的值
//	@return *sharding.ShardInfo 分片信息
func (d *ShardingDataPool) CalculateShard(tableName string, shardingValue interface{}) (*sharding.ShardInfo, error) {
	return sharding.CalculateShardForTable(tableName, shardingValue)
}

// IsInitialized
//
//	@Description: 检查是否已初始化
//	@receiver d
//	@return bool
func (d *ShardingDataPool) IsInitialized() bool {
	if d.manager == nil {
		return false
	}
	return d.manager.IsInitialized()
}

// Close
//
//	@Description: 关闭所有数据库连接
//	@receiver d
func (d *ShardingDataPool) Close() error {
	if d.manager != nil {
		return d.manager.Close()
	}
	return nil
}

