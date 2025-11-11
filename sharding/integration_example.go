// Package sharding
// ///////////////////////////////////////////////////////////////////////////////
// @desc 与 gnbutils 集成使用示例
// ///////////////////////////////////////////////////////////////////////////////
package sharding

import (
	"fmt"

	"github.com/bobwong89757/gnbutils/yaml"
)

// ExampleWithGnbutilsYaml 示例：使用 gnbutils 的 yaml 工具初始化 sharding
func ExampleWithGnbutilsYaml() {
	// 方式一：使用 gnbutils 的 YamlUtil
	yamlUtil := &yaml.YamlUtil{}
	yamlUtil.InitConfig("./config.yaml")

	// 从 YamlUtil 获取 Viper 实例
	viper := yamlUtil.GetViper()

	// 初始化 sharding
	err := InitFromViper(viper, "sharding")
	if err != nil {
		panic(fmt.Errorf("failed to init sharding: %w", err))
	}

	// 现在可以使用 sharding 功能了
	userID := int64(12345)

	// 获取数据库连接
	db, err := MShardingDB.GetDBForTable("users", userID)
	if err != nil {
		panic(err)
	}

	// 使用数据库连接进行查询
	fmt.Printf("Got DB for user %d: %v\n", userID, db)
}

// ExampleDirectYAML 示例：直接从 YAML 文件初始化
func ExampleDirectYAML() {
	// 方式二：直接从 YAML 文件初始化
	err := InitFromYAML("./config.yaml", "sharding")
	if err != nil {
		panic(fmt.Errorf("failed to init sharding: %w", err))
	}

	// 使用全局实例
	userID := int64(12345)
	db := GetDBWithShardingKeyForTable("users", userID)

	fmt.Printf("Got DB for user %d: %v\n", userID, db)
}

// ExampleQueryWithSharding 示例：完整的查询示例
func ExampleQueryWithSharding() {
	// 初始化配置
	yamlUtil := &yaml.YamlUtil{}
	yamlUtil.InitConfig("./config.yaml")
	viper := yamlUtil.GetViper()

	// 初始化 sharding
	if err := InitFromViper(viper, "sharding"); err != nil {
		panic(err)
	}

	// ========== 示例 1: 基于 user_id 的查询 ==========
	userID := int64(12345)

	// 方式 1: 使用 GetDBForTable（推荐）
	db, err := MShardingDB.GetDBForTable("users", userID)
	if err != nil {
		panic(err)
	}

	// 查询用户
	var user struct {
		ID       int64  `gorm:"column:id"`
		UserID   int64  `gorm:"column:user_id"`
		Username string `gorm:"column:username"`
	}
	db.Where("user_id = ?", userID).First(&user)

	fmt.Printf("Found user: %+v\n", user)

	// 方式 2: 使用便捷函数
	db2 := GetDBWithShardingKeyForTable("users", userID)
	db2.Where("user_id = ?", userID).First(&user)

	// 方式 3: 使用 GetShardedDB（最便捷）
	db3, tableName, err := GetShardedDB("users", userID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Table name: %s\n", tableName) // 输出: users_1
	db3.Where("user_id = ?", userID).First(&user)

	// 方式 4: 使用 MustGetShardedDB（最简洁，失败自动降级）
	MustGetShardedDB("users", userID).Where("user_id = ?", userID).First(&user)

	// ========== 示例 2: 基于字符串分片键的查询 ==========
	openID := "test_open_id_1234"

	// 对于字符串类型的分片键
	db = MustGetShardedDB("game_player", openID)
	var player struct {
		ID     int64  `gorm:"column:id"`
		OpenID string `gorm:"column:open_id"`
		Name   string `gorm:"column:name"`
	}
	db.Where("open_id = ?", openID).First(&player)

	// ========== 示例 3: 插入数据 ==========
	newUser := struct {
		UserID   int64  `gorm:"column:user_id"`
		Username string `gorm:"column:username"`
	}{
		UserID:   userID,
		Username: "test_user",
	}

	db = MustGetShardedDB("users", userID)
	db.Create(&newUser)

	// ========== 示例 4: 更新数据 ==========
	db = MustGetShardedDB("users", userID)
	db.Where("user_id = ?", userID).Update("username", "updated_name")

	// ========== 示例 5: 删除数据 ==========
	db = MustGetShardedDB("users", userID)
	db.Where("user_id = ?", userID).Delete(&user)

	// ========== 示例 6: 跨库查询（遍历所有分库） ==========
	allDBs := MShardingDB.GetAllDBs()
	for i, db := range allDBs {
		fmt.Printf("Querying database %d\n", i)

		// 需要手动指定表名
		for tableIndex := 0; tableIndex < 4; tableIndex++ {
			tableName := fmt.Sprintf("users_%d", tableIndex)
			var users []struct {
				ID       int64  `gorm:"column:id"`
				UserID   int64  `gorm:"column:user_id"`
				Username string `gorm:"column:username"`
			}
			db.Table(tableName).Find(&users)
			fmt.Printf("Found %d users in table %s\n", len(users), tableName)
		}
	}
}

// ExampleCalculateShard 示例：计算分片位置（调试用）
func ExampleCalculateShard() {
	// 初始化
	yamlUtil := &yaml.YamlUtil{}
	yamlUtil.InitConfig("./config.yaml")
	viper := yamlUtil.GetViper()

	if err := InitFromViper(viper, "sharding"); err != nil {
		panic(err)
	}

	// 计算分片位置
	userID := int64(12345)
	shardInfo, err := CalculateShardForTable("users", userID)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Shard Info for user %d:\n", userID)
	fmt.Printf("  Database Index: %d\n", shardInfo.DatabaseIndex)
	fmt.Printf("  Table Index: %d\n", shardInfo.TableIndex)
	fmt.Printf("  Database Name: %s\n", shardInfo.DatabaseName)
	fmt.Printf("  Table Name: %s\n", shardInfo.TableName)

	// 输出示例:
	// Shard Info for user 12345:
	//   Database Index: 1
	//   Table Index: 1
	//   Database Name: myapp_1
	//   Table Name: users_1
}

// ExampleWithCustomConfig 示例：自定义配置
func ExampleWithCustomConfig() {
	// 手动构建配置
	config := &ShardingConfig{
		DatabaseCount: 2,
		DatabaseTemplate: DatabaseConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			Username: "root",
			Password: "password",
			Database: "myapp_{db_index}",
			Charset:  "utf8mb4",
		},
		TableConfigs: map[string]*TableShardingConfig{
			"users": {
				TableName:     "users",
				ShardingKey:   "user_id",
				AlgorithmType: "long",
				Algorithm:     NewLongShardingAlgorithm(),
				TableCount:    4,
			},
		},
	}

	// 初始化管理器
	manager := GetManager()
	if err := manager.Init(config); err != nil {
		panic(err)
	}

	// 使用
	userID := int64(12345)
	db := MustGetShardedDB("users", userID)
	fmt.Printf("Got DB: %v\n", db)
}

// ExampleIntegrationWithLog 示例：与 log 模块集成
func ExampleIntegrationWithLog() {
	// 同时初始化日志和分库分表
	yamlUtil := &yaml.YamlUtil{}
	yamlUtil.InitConfig("./config.yaml")
	viper := yamlUtil.GetViper()

	// 初始化日志（使用旧的 API）
	// 注意：需要先将 viper 配置转换为 map[string]string
	logConfig := viper.GetStringMapString("logger")

	// 如果需要，可以从 gnbutils/log 包导入
	// log.MLog.InitLog(logConfig, "myapp")

	// 初始化 sharding
	if err := InitFromViper(viper, "sharding"); err != nil {
		panic(err)
	}

	fmt.Printf("Log config: %+v\n", logConfig)
	fmt.Println("Both log and sharding initialized successfully")
}
