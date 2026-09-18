# 分库分表使用指南

本模块在 **应用层手动路由** 分库分表（`GetDBForTable` + `db.Table("logic_N")`），与 Java `@echo-module-sharding` 的取模算法对齐。

> **不是** [GORM Sharding 插件](https://gorm.io/zh_CN/docs/sharding.html)：不做 SQL 拦截/AST 解析，不依赖 `gorm.io/sharding`。每个表可在 `table_configs` 中配置独立的 `sharding_key`、`table_count`、`algorithm_type`。

## 功能特点

- ✅ **显式路由**：调用方传入分片键，框架计算库/表后缀
- ✅ **开关与 mysql 分离**：根配置 `enable_sharding`；true 只读 `sharding.databases`，false 使用 `mysql`
- ✅ **每库独立地址**：`sharding.databases` 每项自带 host/port/账号/库名，不继承 mysql
- ✅ **与 Java 对齐**：long / string / multi_string 取模算法一致
- ✅ **跨分片 fan-out**：`QueryAllTableShards` / `QueryAllShards` 封装报表类查询
- ✅ **路由失败不降级**：`GetShardedDB` 返回 error；`MustGetShardedDB` panic
- ✅ **雪花主键**：`primary_key_generator: snowflake`（对齐 MyBatis-Plus ASSIGN_ID 布局）

## 初始化（可选）

```go
err := shardingPool.TryInitShardingWithConfig(viper)
if err != nil { /* 处理 */ }

if shardingPool.IsInitialized() {
    mysqlPool.UseDelegate(shardingPool.GetDefaultDB)
} else {
    mysqlPool.InitMysqlWithConfig(mysqlConfig)
}
```

## 配置说明

`enable_sharding` 与 `mysql`、`sharding` 并列。打开后**只读** `sharding.databases`，与 `mysql` 无关。

```yaml
mysql:
  host: 127.0.0.1
  port: 3306
  username: root
  password: your_password
  database: nbgame

# true：用 sharding.databases；false：用 mysql
enable_sharding: true

sharding:
  databases:
    - host: 10.0.0.1
      port: 3306
      username: root
      password: your_password
      database: nbgame_0
    - host: 10.0.0.2
      port: 3306
      username: root
      password: your_password
      database: nbgame_1
  primary_key_generator: "snowflake"
  snowflake:
    worker_id: 1
    datacenter_id: 1
    max_tolerate_time_difference_ms: 2000
  table_configs:
    users:
      algorithm_type: "long"
      sharding_key: "user_id"
      table_count: 4
    players:
      algorithm_type: "long"
      sharding_key: "user_id"
      table_count: 4
```

同一套账号只想少写几遍时，可用 `sharding.database_template` 作为 **sharding 内部** 缺省值（仍然不读 mysql）：

```yaml
sharding:
  database_template:
    port: 3306
    username: root
    password: your_password
  databases:
    - host: 10.0.0.1
      database: nbgame_0
    - host: 10.0.0.2
      database: nbgame_1
```

### 配置参数说明

**全局配置：**
- `enable_sharding`: 根开关。`true` 读 `sharding.databases`；`false` / 不写则走 `mysql`
- `sharding.databases`: 每个分库的完整（或相对 template 的）连接。`database_count` 可省略（等于列表长度）；若写了必须与列表长度一致
- `sharding.database_template`: 可选，仅给 `databases` 补缺省字段，**不会**使用外层 `mysql`
- `primary_key_generator`: 主键生成器，`snowflake`（默认，已实现）、`sequence`/`custom`（未实现）
- `snowflake.worker_id` / `snowflake.datacenter_id`: 对齐 Java `mybatis-plus.global-config.sequence`；缺省均为 1
- `snowflake.max_tolerate_time_difference_ms`: 时钟回拨容忍毫秒数，默认 2000

**生成 ID（如 game_player.id 需在 Create 前赋值）：**

```go
id, err := sharding.NextUint64()
player.ID = int64(id)
db, _ := pool.GetDBForTable("game_player", player.ID)
db.Table("game_player_3").Create(&player)
```

**表级别配置（table_configs，每个表必须配置）：**
- `algorithm_type`: 分片算法类型
  - `long`: 基于 Long 类型的精确分片（取模）
    - 适用场景：分片键是数字类型（int, int64 等）
    - 算法：`id % shardCount`
  - `string`: 基于 String 类型的精确分片（hashCode取模）
    - 适用场景：分片键是字符串类型
    - 算法：`(column.hashCode() & Integer.MAX_VALUE) % shardCount`
  - `multi_string`: 基于多字符串组合的分片
    - 适用场景：需要多个字段组合作为分片键
    - 算法：将多个列值用 `_` 连接，计算 hashCode，然后取模
- `sharding_key`: 分片键字段名，所有查询条件必须包含此字段
- `table_count`: 每个库的分表数量，例如 4 表示每个库有 4 张表（users_0, users_1, users_2, users_3）

## 使用方式

### 方式一：使用表级别的便捷函数（推荐）

```go
import (
    "nbmesh/helpers"
    "nbmesh/models"
    "nbmesh/helpers/sharding"
)

// 根据表名和分片键获取数据库连接（推荐）
userID := int64(12345)
db, err := sharding.GetDBWithShardingKeyForTable("users", userID)
if err != nil {
    return err
}
shardDB, shardTable, err := sharding.GetShardedDB("users", userID)
if err != nil {
    return err
}
_ = shardTable

// 查询用户 - 会自动路由到对应的分库分表
user := &models.User{}
db.Where("id = ? AND user_id = ?", userID, userID).First(user)

// 创建用户 - 会自动路由到对应的分库分表
newUser := &models.User{
    UserID: userID,
    Username: "test",
}
db.Create(newUser)
```

### 方式二：使用全局便捷函数

```go
import "nbmesh/helpers/sharding"

// 根据分片键获取数据库连接
db := sharding.GetDBWithShardingKey(userID)
db.Where("id = ?", userID).First(user)
```

### 方式三：替换原有代码

将原有的 `helpers.MDataPool.GetDB()` 替换为分库分表版本：

**原代码：**
```go
helpers.MDataPool.GetDB().Where("user_id = ?", userID).First(user)
```

**新代码：**
```go
// 使用表级别的 API（推荐）
db := sharding.GetDBWithShardingKeyForTable("users", userID)
db.Where("user_id = ?", userID).First(user)
```

## 重要注意事项

### 1. 查询条件必须包含分片键

所有 CRUD 操作必须在查询条件中包含分片键，否则会抛出 `ErrMissingShardingKey` 错误。

✅ **正确示例：**
```go
// 查询条件包含 user_id（分片键）
db.Where("id = ? AND user_id = ?", id, userID).First(user)
db.Where("user_id = ?", userID).Find(users)
```

❌ **错误示例：**
```go
// 缺少分片键，会报错
db.Where("id = ?", id).First(user)  // 错误！
db.Where("username = ?", username).First(user)  // 错误！
```

### 2. 分片键的计算逻辑

分库分表的计算逻辑与 Java `@echo-module-sharding` 对齐：

1. **分库路由**：`database_index = sharding_value % database_count`
2. **分表路由**：由 GORM Sharding 插件自动处理，`table_index = sharding_value % table_count_per_db`

例如：
- `user_id = 12345`
- `database_count = 2`，`table_count_per_db = 4`
- 路由到：`nbgame_1` 库的 `users_1` 表（12345 % 2 = 1，12345 % 4 = 1）

### 3. 跨分片 / 跨库查询

使用 fan-out 辅助函数，避免手写 `for i := 0; i < tableCount; i++`：

```go
var users []models.User
err := sharding.QueryAllTableShards("relate_user", func(db *gorm.DB, shardTable string) error {
    var batch []models.User
    if err := db.Table(shardTable).Where("status = ?", 1).Find(&batch).Error; err != nil {
        return err
    }
    users = append(users, batch...)
    return nil
})

// 多库场景
err = sharding.QueryAllShards("relate_user", func(db *gorm.DB, dbIndex int, shardTable string) error {
    // ...
    return nil
})
```

底层也可使用 `GetAllDBs()` 自行遍历。

## 与 Java @echo-module-sharding 的对应关系

| Java 配置/类 | Go 配置/实现 | 说明 |
|-------------|-------------|------|
| `@ShardingKey("user_id")` | `sharding_key: "user_id"` | 分片键字段 |
| `@ShardingDatabase(count=2)` | `database_count: 2` | 分库数量 |
| `@ShardingTable(count=4)` | `table_count_per_db: 4` | 每库分表数 |
| `DataPreciseShardingAlgorithm` | `algorithm_type: "long"` | Long 类型精确分片 |
| `DataPreciseShardingStringAlgorithm` | `algorithm_type: "string"` | String 类型精确分片 |
| `DataPreciseShardingMultiStringAlgorithm` | `algorithm_type: "multi_string"` | 多字符串组合分片 |
| 自动路由 | `GetDB(shardingValue)` | 根据分片键自动路由 |

### 分片算法详细说明

#### 1. Long 算法（`algorithm_type: "long"`）

对应 Java: `DataPreciseShardingAlgorithm`

**算法逻辑：**
```go
tableIndex = (int) (id % availableTargetNames.size())
```

**使用场景：**
- 分片键是数字类型（int, int64, uint 等）
- 例如：`user_id = 12345` → `12345 % 4 = 1` → 路由到 `users_1` 表

**示例：**
```go
userID := int64(12345)
db := helpers.MShardingDB.GetDB(userID)
db.Where("user_id = ?", userID).First(user)
```

#### 2. String 算法（`algorithm_type: "string"`）

对应 Java: `DataPreciseShardingStringAlgorithm`

**算法逻辑：**
```go
hashCode = column.hashCode()
tableSuffix = (hashCode & Integer.MAX_VALUE) % availableTargetNames.size()
```

**使用场景：**
- 分片键是字符串类型
- 例如：`username = "alice"` → hashCode 取模 → 路由到对应表

**示例：**
```go
username := "alice"
db := helpers.MShardingDB.GetDB(username)
db.Where("username = ?", username).First(user)
```

#### 3. MultiString 算法（`algorithm_type: "multi_string"`）

对应 Java: `DataPreciseShardingMultiStringAlgorithm`

**算法逻辑：**
```go
combined = String.join("_", columnValues)
hashCode = combined.hashCode()
shardIndex = (hashCode & Integer.MAX_VALUE) % shardingCount
```

**使用场景：**
- 需要多个字段组合作为分片键
- 例如：`user_id + order_id` 组合分片

**示例：**
```go
// 注意：multi_string 算法需要特殊处理，目前 GORM sharding 插件主要支持单列分片
// 如果需要多列组合，可能需要自定义实现
```

## 迁移步骤

1. **添加配置**：在 YAML 中添加 `sharding`（可选；不配置则不启用）
2. **初始化**：`TryInitShardingWithConfig` + `MysqlDataPool.UseDelegate` 避免双连接池
3. **改访问方式**：单分片用 `GetShardedDB`；跨分片用 `QueryAllTableShards`
4. **查询必须带分片键**（单分片路由时）
5. **测试验证**：与 Java 端路由结果对比

## 示例：修改现有代码

**修改前（models_fit/user_fit.go）：**
```go
func FindUserById(userId string) *models.User {
    user := &models.User{}
    helpers.MDataPool.GetDB().Where("id = ? ", userId).First(user)
    return user
}
```

**修改后：**
```go
func FindUserById(userId string) *models.User {
    user := &models.User{}
    // 将 userId 转换为 int64 作为分片键
    userIDInt, _ := strconv.ParseInt(userId, 10, 64)
    db := helpers.MShardingDB.GetDB(userIDInt)
    db.Where("id = ? AND user_id = ?", userId, userIDInt).First(user)
    return user
}
```

## 故障排查

1. **错误：`ErrMissingShardingKey`**
   - 原因：查询条件中没有包含分片键
   - 解决：在查询条件中添加分片键字段

2. **错误：`sharding manager not initialized`**
   - 原因：分库分表未初始化
   - 解决：检查配置文件是否正确，确保 `sharding` 配置项存在

3. **数据路由不一致**
   - 原因：分片键计算逻辑不一致
   - 解决：确保与 Java 端使用相同的取模算法

## 参考文档

- Java `@echo-module-sharding` 对齐说明见上文算法章节
- [GORM 文档](https://gorm.io/)（本模块不使用 gorm.io/sharding 插件）

