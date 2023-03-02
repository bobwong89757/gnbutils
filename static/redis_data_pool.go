package static

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"strconv"
)

type RedisDataPool struct {
	db *redis.Client
}

// 初始化redis
func (d *RedisDataPool) InitRedisWithConfig(config map[string]string) (bool, error) {
	address := fmt.Sprintf("%s:%s", config["host"], config["port"])
	db, _ := strconv.Atoi(config["db"])
	pwd := config["pwd"]
	maxIdle, _ := strconv.Atoi(config["maxIdle"])

	rdb := redis.NewClient(&redis.Options{
		Network:      "tcp",
		Addr:         address,
		Password:     pwd,
		DB:           db,
		MaxIdleConns: maxIdle,
	})
	d.db = rdb
	return true, nil
}

func (d *RedisDataPool) GetDB() *redis.Client {
	return d.db
}
