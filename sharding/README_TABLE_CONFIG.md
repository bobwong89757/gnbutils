# 表级别分片配置说明

## 概述

支持为每个表配置不同的分片算法，满足不同表的业务需求。

## 配置方式

### 方式一：简单格式（所有表使用相同算法）

```yaml
sharding:
  sharding_key: "user_id"
  database_count: 2
  table_count_per_db: 4
  algorithm_type: "long"
  sharding_tables: "users,players"
```

所有表（users, players）都使用 `long` 算法，分片键为 `user_id`，每个库分 4 张表。

### 方式二：详细格式（每个表独立配置）

```yaml
sharding:
  # 全局默认值
  sharding_key: "user_id"
  database_count: 2
  table_count_per_db: 4
  algorithm_type: "long"
  
  # 表级别配置（覆盖全局配置）
  table_configs:
    users:
      algorithm_type: "long"      # 使用 long 算法
      sharding_key: "user_id"      # 分片键
      table_count: 4               # 每个库分 4 张表
    orders:
      algorithm_type: "string"     # 使用 string 算法
      sharding_key: "order_no"     # 不同的分片键
      table_count: 8               # 每个库分 8 张表
    players:
      algorithm_type: "long"      # 使用 long 算法
      sharding_key: "player_id"    # 不同的分片键
      table_count: 4               # 每个库分 4 张表
```

## 配置项说明

### 表级别配置项（可选）

- `algorithm_type`: 分片算法类型
  - `long`: 基于 Long 类型的精确分片（取模）
  - `string`: 基于 String 类型的精确分片（hashCode取模）
  - `multi_string`: 基于多字符串组合的分片
  - 如果不配置，使用全局的 `algorithm_type`

- `sharding_key`: 分片键字段名
  - 如果不配置，使用全局的 `sharding_key`

- `table_count`: 每个库的分表数量
  - 如果不配置，使用全局的 `table_count_per_db`

## 使用示例

### 示例 1：不同表使用不同算法

```yaml
sharding:
  database_count: 2
  table_count_per_db: 4
  algorithm_type: "long"  # 默认算法
  
  table_configs:
    users:
      algorithm_type: "long"    # users 表使用 long 算法
      sharding_key: "user_id"
    orders:
      algorithm_type: "string"  # orders 表使用 string 算法
      sharding_key: "order_no"
```

代码使用：

```go
// users 表 - 使用 long 算法
userID := int64(12345)
db := helpers.MShardingDB.GetDB(userID)
db.Table("users").Where("user_id = ?", userID).First(&user)

// orders 表 - 使用 string 算法
orderNo := "ORD123456"
db = helpers.MShardingDB.GetDB(orderNo)
db.Table("orders").Where("order_no = ?", orderNo).First(&order)
```

### 示例 2：不同表使用不同的分表数量

```yaml
sharding:
  database_count: 2
  table_count_per_db: 4  # 默认每个库分 4 张表
  
  table_configs:
    users:
      table_count: 4     # users 表：每个库分 4 张表
    orders:
      table_count: 8     # orders 表：每个库分 8 张表
    logs:
      table_count: 16     # logs 表：每个库分 16 张表
```

### 示例 3：混合使用

```yaml
sharding:
  # 全局默认配置
  sharding_key: "user_id"
  database_count: 2
  table_count_per_db: 4
  algorithm_type: "long"
  
  # 简单格式：使用全局配置
  sharding_tables: "users,players"
  
  # 详细格式：覆盖全局配置
  table_configs:
    orders:
      algorithm_type: "string"
      sharding_key: "order_no"
      table_count: 8
```

## 注意事项

1. **表配置优先级**：表级别的配置会覆盖全局配置
2. **分片键一致性**：查询时必须包含对应表的分片键
3. **算法选择**：根据分片键的数据类型选择合适的算法
   - 数字类型（int, int64）→ `long` 算法
   - 字符串类型 → `string` 算法
   - 多字段组合 → `multi_string` 算法

## 与 Java 的对应关系

在 Java 中，可以通过不同的注解为不同的表配置不同的算法：

```java
@ShardingTable(count = 4, algorithm = DataPreciseShardingAlgorithm.class)
public class User {
    @ShardingKey
    private Long userId;
}

@ShardingTable(count = 8, algorithm = DataPreciseShardingStringAlgorithm.class)
public class Order {
    @ShardingKey
    private String orderNo;
}
```

在 Go 中，通过 `table_configs` 实现相同的功能。

