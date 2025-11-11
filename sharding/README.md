# 分库分表使用指南

本模块基于 [GORM Sharding 插件](https://gorm.io/zh_CN/docs/sharding.html) 实现分库分表功能，与 Java `@echo-module-sharding` 的逻辑对齐。

## 功能特点

- ✅ **非侵入式设计**：加载插件，指定配置，即可实现分库分表
- ✅ **高性能**：基于连接层做 SQL 拦截、AST 解析、分表路由，额外开销极小
- ✅ **支持分库+分表**：先分库，再在每个库内分表
- ✅ **自动路由**：根据分片键自动路由到对应的库和表
- ✅ **与 Java 对齐**：使用相同的取模算法，确保数据路由一致

## 配置说明

在配置文件中添加 `sharding` 配置项：

```yaml
mysql:
  host: 127.0.0.1
  port: 3306
  username: root
  password: your_password
  database: nbgame  # 如果启用分库，可以使用占位符: nbgame_{db_index}

# 分库分表配置（可选，如果不配置则不启用分库分表）
sharding:
  # 分库数量（如果为1则不分库，只分表）
  database_count: 1
  # 主键生成器: snowflake, sequence, custom
  primary_key_generator: "snowflake"
  # 表级别的详细配置（每个表单独配置，必需）
  # 每个表必须配置：algorithm_type, sharding_key, table_count
  table_configs:
    users:
      algorithm_type: "long"      # 分片算法类型: long, string, multi_string
      sharding_key: "user_id"      # 分片键字段名
      table_count: 4               # 每个库的分表数量
    players:
      algorithm_type: "long"       # 分片算法类型
      sharding_key: "user_id"      # 分片键字段名
      table_count: 4               # 每个库的分表数量
```

### 配置参数说明

**全局配置：**
- `database_count`: 分库数量，例如 2 表示分成 2 个库（nbgame_0, nbgame_1）
- `primary_key_generator`: 主键生成器类型
  - `snowflake`: 雪花算法（推荐）
  - `sequence`: PostgreSQL 序列
  - `custom`: 自定义生成器

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
db := sharding.GetDBWithShardingKeyForTable("users", userID)

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

### 3. 跨库查询

如果需要跨库查询，可以使用 `GetAllDBs()` 方法：

```go
allDBs := helpers.MShardingDB.GetAllDBs()
for _, db := range allDBs {
    var users []models.User
    db.Where("status = ?", 1).Find(&users)
    // 处理查询结果...
}
```

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

1. **安装依赖**：
   ```bash
   go get -u gorm.io/sharding
   ```

2. **添加配置**：在配置文件中添加 `sharding` 配置项

3. **修改代码**：将 `helpers.MDataPool.GetDB()` 替换为 `helpers.MShardingDB.GetDB(shardingKey)`

4. **确保查询包含分片键**：所有查询条件必须包含分片键字段

5. **测试验证**：确保数据路由正确，与 Java 端逻辑一致

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

- [GORM Sharding 官方文档](https://gorm.io/zh_CN/docs/sharding.html)
- [GORM Sharding GitHub](https://github.com/go-gorm/sharding)

