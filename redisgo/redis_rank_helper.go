package redisgo

import "time"

// redis最小因子
const MIN_REDIS_FACTOR float64 = 1000000000

// redis最大因子
const MAX_REDIS_FACTOR float64 = 2000000000

// 设置有序结合的score
func SetScore(raw float64) float64 {
	return raw*MIN_REDIS_FACTOR + MAX_REDIS_FACTOR - float64(time.Now().Unix())
}

// 获取有序集合的score
func GetScore(data float64) float64 {
	return data / MIN_REDIS_FACTOR
}
