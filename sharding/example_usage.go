// Package sharding
// 使用示例
package sharding

/*
// 示例1：查询数据（根据 sharding key）
func FindUserByOpenID(openID string) (*models.RelateUser, error) {
    // 1. 计算分片信息
    shardInfo, err := CalculateShardForTable("relate_user", openID)
    if err != nil {
        return nil, err
    }

    // 2. 获取数据库连接
    db, err := GetDBWithShardingKeyForTable("relate_user", openID)
    if err != nil {
        return nil, err
    }

    // 3. 使用带后缀的表名查询
    var user models.RelateUser
    err = db.Table(shardInfo.TableName).Where("open_id = ?", openID).First(&user).Error
    // 实际执行：SELECT * FROM relate_user_3 WHERE open_id = 'xxx'

    return &user, err
}

// 示例2：插入数据（根据 sharding key）
func CreateUser(user *models.RelateUser) error {
    // 1. 计算分片信息
    shardInfo, err := CalculateShardForTable("relate_user", user.OpenID)
    if err != nil {
        return err
    }

    // 2. 获取数据库连接
    db, err := GetDBWithShardingKeyForTable("relate_user", user.OpenID)
    if err != nil {
        return err
    }

    // 3. 插入到分片表
    err = db.Table(shardInfo.TableName).Create(user).Error
    // 实际执行：INSERT INTO relate_user_3 ...

    return err
}

// 示例3：更新数据（根据 sharding key）
func UpdateUser(openID string, updates map[string]interface{}) error {
    shardInfo, err := CalculateShardForTable("relate_user", openID)
    if err != nil {
        return err
    }

    db, err := GetDBWithShardingKeyForTable("relate_user", openID)
    if err != nil {
        return err
    }

    err = db.Table(shardInfo.TableName).
        Where("open_id = ?", openID).
        Updates(updates).Error
    // 实际执行：UPDATE relate_user_3 SET ... WHERE open_id = 'xxx'

    return err
}

// 示例4：删除数据（根据 sharding key）
func DeleteUser(openID string) error {
    shardInfo, err := CalculateShardForTable("relate_user", openID)
    if err != nil {
        return err
    }

    db, err := GetDBWithShardingKeyForTable("relate_user", openID)
    if err != nil {
        return err
    }

    err = db.Table(shardInfo.TableName).
        Where("open_id = ?", openID).
        Delete(&models.RelateUser{}).Error
    // 实际执行：DELETE FROM relate_user_3 WHERE open_id = 'xxx'

    return err
}

// 示例5：批量操作（不同的 sharding key）
func CreateMultipleUsers(users []*models.RelateUser) error {
    for _, user := range users {
        shardInfo, err := CalculateShardForTable("relate_user", user.OpenID)
        if err != nil {
            return err
        }

        db, err := GetDBWithShardingKeyForTable("relate_user", user.OpenID)
        if err != nil {
            return err
        }

        err = db.Table(shardInfo.TableName).Create(user).Error
        if err != nil {
            return err
        }
    }
    return nil
}

// 示例6：使用 game_player 表（Long 算法）
func FindPlayerByID(playerID int64) (*models.GamePlayer, error) {
    shardInfo, err := CalculateShardForTable("game_player", playerID)
    if err != nil {
        return nil, err
    }

    db, err := GetDBWithShardingKeyForTable("game_player", playerID)
    if err != nil {
        return nil, err
    }

    var player models.GamePlayer
    err = db.Table(shardInfo.TableName).Where("id = ?", playerID).First(&player).Error
    // 实际执行：SELECT * FROM game_player_5 WHERE id = 123

    return &player, err
}

// 示例7：跨分片查询（需要查询所有分片表）
func FindAllUsersByCondition(condition string) ([]*models.RelateUser, error) {
    var allUsers []*models.RelateUser

    // 获取配置
    manager := GetManager()
    config := manager.GetConfig()
    tableConfig := config.TableConfigs["relate_user"]

    // 遍历所有分片表
    for i := 0; i < tableConfig.TableCount; i++ {
        tableName := fmt.Sprintf("relate_user_%d", i)

        db, err := manager.GetDBByIndex(0) // 获取第一个数据库
        if err != nil {
            return nil, err
        }

        var users []*models.RelateUser
        err = db.Table(tableName).Where(condition).Find(&users).Error
        if err != nil {
            return nil, err
        }

        allUsers = append(allUsers, users...)
    }

    return allUsers, nil
}

// 示例8：验证分片计算结果（调试用）
func DebugSharding() {
    // 测试 relate_user 表（String 算法）
    shardInfo, _ := CalculateShardForTable("relate_user", "test1013")
    fmt.Printf("relate_user with 'test1013' -> %s (table index: %d)\n",
        shardInfo.TableName, shardInfo.TableIndex)

    // 测试 game_player 表（Long 算法）
    shardInfo, _ = CalculateShardForTable("game_player", int64(12345))
    fmt.Printf("game_player with 12345 -> %s (table index: %d)\n",
        shardInfo.TableName, shardInfo.TableIndex)

    // 测试多个值，查看分布
    for i := 0; i < 10; i++ {
        openID := fmt.Sprintf("user_%d", i)
        shardInfo, _ := CalculateShardForTable("relate_user", openID)
        fmt.Printf("user_%d -> %s\n", i, shardInfo.TableName)
    }
}
*/
