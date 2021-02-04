package redisgo

import "time"

// redis最小因子
const MIN_REDIS_FACTOR int64 = 1000000000

// redis最大因子
const MAX_REDIS_FACTOR int64 = 2000000000

// 设置有序结合的score
func SetScoreInt32(raw int64) int64 {
	return raw * MIN_REDIS_FACTOR + MAX_REDIS_FACTOR - time.Now().Unix()
}

// 设置有序结合的score
func SetScoreInt64(raw int64) int64 {
	return raw * MIN_REDIS_FACTOR + MAX_REDIS_FACTOR - time.Now().Unix()
}

// 获取有序集合的score
func GetScore( data int64) int64 {
	return data / MIN_REDIS_FACTOR
}
