package static

import (
	"fmt"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type MysqlDataPool struct {
	db *sql.DB
}

func (d *MysqlDataPool) InitMysql(host string,port string,username string,password string,dbName string) {
	var err error
	d.db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True", username, password, host, port, dbName))
	if err != nil {
		defer d.db.Close()
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}

// 初始化冷数据库
func (d *MysqlDataPool) InitMysqlWithConfig(config map[string]string) {
	var err error
	d.db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True", config["username"], config["password"], config["host"], config["port"], config["database"]))
	if err != nil {
		defer d.db.Close()
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}


func (d *MysqlDataPool) GetDB() *sql.DB {
	return d.db
}
