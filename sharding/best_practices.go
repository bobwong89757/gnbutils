// Package sharding
// 最佳实践示例
package sharding

/*
import (
	"errors"
	"fmt"
	"nbmesh/helpers"
	"nbmesh/models"

	"gorm.io/gorm"
)

// ✅ 推荐：查询单条记录 - 使用 First()
func FindUserByOpenID(openID string) (*models.RelateUser, error) {
	var user models.RelateUser
	err := helpers.GetShardDB("relate_user", openID).
		Where("open_id = ?", openID).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 记录不存在，返回 nil 和特定错误
			return nil, fmt.Errorf("user with open_id %s not found", openID)
		}
		// 其他数据库错误
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	return &user, nil
}

// ✅ 推荐：查询多条记录 - 使用 Find()
func FindUsersByStatus(status int) ([]*models.RelateUser, error) {
	var users []*models.RelateUser

	// 注意：跨分片查询需要遍历所有分片表
	// 这里假设我们知道如何获取所有分片
	manager := GetManager()
	config := manager.GetConfig()
	tableConfig := config.TableConfigs["relate_user"]

	for i := 0; i < tableConfig.TableCount; i++ {
		tableName := fmt.Sprintf("relate_user_%d", i)
		var shardUsers []*models.RelateUser

		db, _ := manager.GetDBByIndex(0)
		err := db.Table(tableName).
			Where("status = ?", status).
			Find(&shardUsers).Error

		if err != nil {
			return nil, fmt.Errorf("failed to query shard %s: %w", tableName, err)
		}

		users = append(users, shardUsers...)
	}

	// Find() 查不到记录时不报错，只是返回空数组
	return users, nil
}

// ✅ 推荐：检查记录是否存在
func UserExists(openID string) (bool, error) {
	var count int64
	err := helpers.GetShardDB("relate_user", openID).
		Where("open_id = ?", openID).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// ✅ 推荐：创建记录（检查是否已存在）
func CreateUserIfNotExists(user *models.RelateUser) error {
	// 先检查是否存在
	exists, err := UserExists(user.OpenID)
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("user with open_id %s already exists", user.OpenID)
	}

	// 创建记录
	err = helpers.GetShardDB("relate_user", user.OpenID).Create(user).Error
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// ✅ 推荐：更新记录（检查影响行数）
func UpdateUser(openID string, updates map[string]interface{}) error {
	result := helpers.GetShardDB("relate_user", openID).
		Where("open_id = ?", openID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user with open_id %s not found or no changes", openID)
	}

	return nil
}

// ✅ 推荐：删除记录（检查影响行数）
func DeleteUser(openID string) error {
	result := helpers.GetShardDB("relate_user", openID).
		Where("open_id = ?", openID).
		Delete(&models.RelateUser{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user with open_id %s not found", openID)
	}

	return nil
}

// ✅ 推荐：软删除（使用 GORM 的软删除）
func SoftDeleteUser(openID string) error {
	// 假设 RelateUser 有 DeletedAt 字段（gorm.DeletedAt）
	result := helpers.GetShardDB("relate_user", openID).
		Where("open_id = ?", openID).
		Delete(&models.RelateUser{})

	if result.Error != nil {
		return fmt.Errorf("failed to soft delete user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user with open_id %s not found", openID)
	}

	return nil
}

// ❌ 错误示例：使用 Find() 查询单条记录
func FindUserByOpenIDBad(openID string) (*models.RelateUser, error) {
	var user models.RelateUser
	err := helpers.GetShardDB("relate_user", openID).
		Where("open_id = ?", openID).
		Find(&user).Error

	// 问题：即使查不到记录，err 也是 nil
	// user 会是零值对象，但你不知道是查不到还是数据库错误
	if err != nil {
		return nil, err
	}

	// 需要额外检查是否是零值（不推荐）
	if user.OpenID == "" {
		return nil, fmt.Errorf("user not found")
	}

	return &user, nil
}

// ✅ 推荐：使用事务
func TransferData(fromOpenID, toOpenID string, amount int) error {
	// 获取数据库连接
	db := helpers.GetShardDB("relate_user", fromOpenID)

	// 开启事务
	return db.Transaction(func(tx *gorm.DB) error {
		// 注意：事务中也需要指定表名
		shardInfo, err := CalculateShardForTable("relate_user", fromOpenID)
		if err != nil {
			return err
		}

		var fromUser models.RelateUser
		err = tx.Table(shardInfo.TableName).
			Where("open_id = ?", fromOpenID).
			First(&fromUser).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("from user not found")
			}
			return err
		}

		// 检查余额
		if fromUser.Balance < amount {
			return fmt.Errorf("insufficient balance")
		}

		// 扣款
		err = tx.Table(shardInfo.TableName).
			Where("open_id = ?", fromOpenID).
			Update("balance", gorm.Expr("balance - ?", amount)).Error
		if err != nil {
			return err
		}

		// 加款（可能在不同的分片）
		toShardInfo, err := CalculateShardForTable("relate_user", toOpenID)
		if err != nil {
			return err
		}

		err = tx.Table(toShardInfo.TableName).
			Where("open_id = ?", toOpenID).
			Update("balance", gorm.Expr("balance + ?", amount)).Error
		if err != nil {
			return err
		}

		return nil
	})
}

// ✅ 推荐：批量插入（性能优化）
func BatchCreateUsers(users []*models.RelateUser) error {
	// 按分片分组
	shardGroups := make(map[string][]*models.RelateUser)

	for _, user := range users {
		shardInfo, err := CalculateShardForTable("relate_user", user.OpenID)
		if err != nil {
			return err
		}
		shardGroups[shardInfo.TableName] = append(shardGroups[shardInfo.TableName], user)
	}

	// 批量插入每个分片
	for tableName, shardUsers := range shardGroups {
		// 使用第一个用户的 OpenID 获取 DB（只要在同一分片即可）
		db := helpers.GetShardDB("relate_user", shardUsers[0].OpenID)
		err := db.CreateInBatches(shardUsers, 100).Error
		if err != nil {
			return fmt.Errorf("failed to batch create users in %s: %w", tableName, err)
		}
	}

	return nil
}
*/
