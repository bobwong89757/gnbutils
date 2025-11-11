# 分库分表快速开始指南

## 安装依赖

依赖已自动安装，如果遇到问题，可以手动执行：

```bash
go get -u gorm.io/sharding
```

## 配置

在配置文件中添加 `sharding` 配置项（参考 `cfg/development.yml`）：

```yaml
mysql:
  host: 127.0.0.1
  port: 3306
  username: root
  password: your_password
  database: nbgame  # 如果启用分库，可以使用占位符: nbgame_{db_index}

# 分库分表配置
sharding:
  sharding_key: "user_id"        # 分片键字段名
  database_count: 2                # 分库数量
  table_count_per_db: 4           # 每个库的分表数量
  primary_key_generator: "snowflake" # 主键生成器
  sharding_tables: "users,players"  # 需要分片的表名（逗号分隔）
```

## 使用方式

### 方式一：替换原有代码（推荐）

**修改前：**
```go
helpers.MDataPool.GetDB().Where("user_id = ?", userID).First(user)
```

**修改后：**
```go
helpers.MShardingDB.GetDB(userID).Where("user_id = ?", userID).First(user)
```

### 方式二：使用便捷函数

```go
import "nbmesh/helpers/sharding"

db := sharding.GetDBWithShardingKey(userID)
db.Where("user_id = ?", userID).First(user)
```

## 重要提示

1. **查询条件必须包含分片键**，否则会报 `ErrMissingShardingKey` 错误
2. **分片键的值决定了数据存储在哪个库的哪个表**
3. **与 Java 端逻辑对齐**：使用取模运算进行路由

## 示例

完整示例请参考 `helpers/sharding/README.md`

