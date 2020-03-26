package static

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"strconv"
	_ "github.com/go-sql-driver/mysql"
)

type RedisDataPool struct {
	db *redis.Conn
}

// 初始化冷数据库
func (d *RedisDataPool) InitRedisWithConfig(config map[string]string) (bool, error) {
	address := fmt.Sprintf("%s:%s", config["host"], config["port"])
	db, _ := strconv.Atoi(config["db"])
	pwd := config["pwd"]
	conn, err := redis.Dial("tcp", address, redis.DialDatabase(db), redis.DialPassword(pwd))
	if err != nil {
		fmt.Println("could not init redis " + err.Error())
		panic(err)
		return false, err
	} else {
		d.db = &conn
		return true, nil
	}
}


func (d *RedisDataPool) GetDB() *redis.Conn {
	return d.db
}
