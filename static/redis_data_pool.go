package static

import (
	"fmt"
	"github.com/bobwong89757/gnbutils/redisgo"
	_ "github.com/go-sql-driver/mysql"
	"strconv"
)

type RedisDataPool struct {
	db *redisgo.Cacher
}

// 初始化冷数据库
func (d *RedisDataPool) InitRedisWithConfig(config map[string]string) (bool, error) {
	address := fmt.Sprintf("%s:%s", config["host"], config["port"])
	db, _ := strconv.Atoi(config["db"])
	pwd := config["pwd"]
	prefix := config["prefix"]
	maxActive,_ := strconv.Atoi(config["maxActive"])
	maxIdle,_  := strconv.Atoi(config["maxIdle"])
	options := redisgo.Options{
		Network:     "tcp",
		Addr:        address,
		Password:    pwd,
		Db:          db,
		MaxActive:   maxActive,
		MaxIdle:     maxIdle,
		Prefix:      prefix,
	}
	cacher,err := redisgo.New(options)
	if err != nil {
		fmt.Println("could not init redisgo " + err.Error())
		panic(err)
		return false, err
	}else {
		d.db = cacher
		return true, nil
	}
}


func (d *RedisDataPool) GetDB() *redisgo.Cacher {
	return d.db
}
