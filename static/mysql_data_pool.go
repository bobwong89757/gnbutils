package static

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type MysqlDataPool struct {
	db       *gorm.DB
	delegate func() *gorm.DB
}

// UseDelegate 复用外部连接池（如 ShardingDataPool.GetDefaultDB），避免重复建连。
// 设置 delegate 后 GetDB 优先返回 delegate 结果；可不再调用 InitMysqlWithConfig。
func (d *MysqlDataPool) UseDelegate(delegate func() *gorm.DB) {
	d.delegate = delegate
}

// InitMysql
//
//	@Description: 通过参数初始化数据库
//	@receiver d
//	@param host
//	@param port
//	@param username
//	@param password
//	@param dbName
func (d *MysqlDataPool) InitMysql(host string, port string, username string, password string, dbName string) {
	var err error
	url := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True", username, password, host, port, dbName)
	fmt.Println("init mysql with " + url)
	d.db, err = gorm.Open(mysql.Open(url), &gorm.Config{})
	if err != nil {
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}

// InitMysqlWithConfig
//
//	@Description: 通过配置初始化数据库
//	@receiver d
//	@param config
func (d *MysqlDataPool) InitMysqlWithConfig(config map[string]string) {
	var err error
	url := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True", config["username"], config["password"], config["host"], config["port"], config["database"])
	fmt.Println("init mysql with " + url)
	d.db, err = gorm.Open(mysql.Open(url), &gorm.Config{})
	if err != nil {
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}

// GetDB
//
//	@Description: 获取数据库连接
//	@receiver d
//	@return *gorm.DB
func (d *MysqlDataPool) GetDB() *gorm.DB {
	if d.delegate != nil {
		if db := d.delegate(); db != nil {
			return db
		}
	}
	return d.db
}
