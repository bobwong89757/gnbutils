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

func (d *ShardingDataPool) InitShardingWithAutoConfig(v *viper.Viper) error {
	if !sharding.IsShardingEnabled(v) {
		return fmt.Errorf("enable_sharding is not true")
	}
	if err := d.TryInitShardingWithConfig(v); err != nil {
		return err
	}
	if !d.IsInitialized() {
		return fmt.Errorf("sharding did not initialize")
	}
	return nil
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
//	@Description: 使用配置初始化分库分表（自动检测配置格式）；失败时 panic
//	@receiver d
//	@param v Viper 实例
func (d *ShardingDataPool) InitShardingWithConfig(v *viper.Viper) {
	if err := d.TryInitShardingWithConfig(v); err != nil {
		fmt.Println("could not init sharding: " + err.Error())
		panic("sharding init error")
	}
}

// TryInitShardingWithConfig 尝试初始化分库分表。
// - enable_sharding=false / 未配置：跳过，返回 nil（调用方应使用 mysql）
// - enable_sharding=true：只读 sharding.databases，与 mysql 无关
func (d *ShardingDataPool) TryInitShardingWithConfig(v *viper.Viper) error {
	if !sharding.IsShardingEnabled(v) {
		fmt.Println("[Sharding Init] enable_sharding=false, skip (use mysql)")
		return nil
	}
	if !v.IsSet("sharding") {
		return fmt.Errorf("enable_sharding is true but sharding config is missing")
	}

	fmt.Println("[Sharding Init] enable_sharding=true, loading sharding.databases")
	config, err := sharding.LoadConfigFromViper(v, "sharding")
	if err != nil {
		return err
	}

	fmt.Printf("[Sharding Init] Config loaded - DB count: %d, Tables: %d\n",
		config.DatabaseCount, len(config.TableConfigs))
	for i := 0; i < config.DatabaseCount; i++ {
		cfg, err := config.ResolveDatabaseConfig(i)
		if err != nil {
			return fmt.Errorf("resolve database %d: %w", i, err)
		}
		fmt.Printf("[Sharding Init] DB[%d] %s:%d/%s\n", i, cfg.Host, cfg.Port, cfg.Database)
	}

	manager := sharding.GetManager()
	if err := manager.Init(config); err != nil {
		return fmt.Errorf("failed to init manager: %w", err)
	}

	d.manager = manager
	fmt.Println("[Sharding Init] Initialization successful")
	return nil
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
func (d *ShardingDataPool) GetDB(shardingValue interface{}) (*gorm.DB, error) {
	return d.manager.GetDB(shardingValue)
}

// GetDBForTable
//
//	@Description: 根据表名和分片键获取数据库连接（推荐使用）
func (d *ShardingDataPool) GetDBForTable(tableName string, shardingValue interface{}) (*gorm.DB, error) {
	return d.manager.GetDBForTable(tableName, shardingValue)
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
func (d *ShardingDataPool) GetShardedDB(tableName string, shardingValue interface{}) (*gorm.DB, string, error) {
	return sharding.GetShardedDB(tableName, shardingValue)
}

// MustGetShardedDB 路由失败时 panic，不会静默降级到默认库。
func (d *ShardingDataPool) MustGetShardedDB(tableName string, shardingValue interface{}) *gorm.DB {
	return sharding.MustGetShardedDB(tableName, shardingValue)
}

// QueryAllTableShards 跨分表 fan-out 查询。
func (d *ShardingDataPool) QueryAllTableShards(tableName string, fn func(db *gorm.DB, shardTableName string) error) error {
	if d.manager == nil {
		return fmt.Errorf("sharding manager not initialized")
	}
	return d.manager.QueryAllTableShards(tableName, fn)
}

// QueryAllShards 跨库 × 跨分表 fan-out 查询。
func (d *ShardingDataPool) QueryAllShards(tableName string, fn func(db *gorm.DB, dbIndex int, shardTableName string) error) error {
	if d.manager == nil {
		return fmt.Errorf("sharding manager not initialized")
	}
	return d.manager.QueryAllShards(tableName, fn)
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

// ========== 全局便捷函数 ==========

// GetShardDB 全局便捷函数：获取分片表 DB session；路由失败 panic。
func GetShardDB(tableName string, shardingKey interface{}) *gorm.DB {
	return sharding.MustGetShardedDB(tableName, shardingKey)
}
