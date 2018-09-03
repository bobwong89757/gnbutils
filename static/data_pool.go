package static

import (
	"fmt"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type DataPool struct {
	db *sql.DB
}

func (d *DataPool) SetUp(username string,password string,dbName string) {
	var err error
	d.db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@/%s?charset=utf8&parseTime=True",username,password,dbName))
	if err != nil {
		defer d.db.Close()
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}

func (d *DataPool) GetDB() *sql.DB {
	return d.db
}
