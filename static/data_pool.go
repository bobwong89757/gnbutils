/////////////////////////////////////////////////////////////////////////////////
// @desc sqlite数据池
// @copyright ©2018 iGG
// @release 2018年9月3日 星期一
// @author BobWong
// @mail 15959187562@qq.com
/////////////////////////////////////////////////////////////////////////////////

package static

import (
	"fmt"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type DataPool struct {
	Db *sql.DB
}

func (d *DataPool) SetUp(username string,password string,dbName string) {
	var err error
	d.Db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@/%s?charset=utf8&parseTime=True",username,password,dbName))
	if err != nil {
		defer d.Db.Close()
		fmt.Println("could not init db " + err.Error())
		panic("db error")
	}
}

func (d *DataPool) GetDB() *sql.DB {
	return d.Db
}
