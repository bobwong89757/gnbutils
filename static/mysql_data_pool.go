package static

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type MysqlDataPool struct {
	db *gorm.DB
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

	d.db, err = gorm.Open(sqlite.Open(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True", username, password, host, port, dbName)), &gorm.Config{})
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
	d.db, err = gorm.Open(sqlite.Open(fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True", config["username"], config["password"], config["host"], config["port"], config["database"])))
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
	return d.db
}
