package static

import (
	"fmt"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type DataPool struct {
	db *sql.DB
}

func (d *DataPool) InitMysql(username string,password string,dbName string,host string,port string) {
	var err error
	d.db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@protocol(%s):%s/%s?charset=utf8&parseTime=True", username, password, host, port, dbName))
	if err != nil {
		defer d.db.Close()
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}

// 初始化冷数据库
func (d *DataPool) InitMysqlWithConfig(config map[string]string) {
	var err error
	d.db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@protocol(%s):%s/%s?charset=utf8&parseTime=True", config["username"], config["password"], config["host"], config["port"], config["database"]))
	if err != nil {
		defer d.db.Close()
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}


func (d *DataPool) GetDB() *sql.DB {
	return d.db
}
