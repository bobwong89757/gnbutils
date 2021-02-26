package util

import (
	"math/rand"
	"time"
)

func GetRandomRange(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	randNum := rand.Intn(max - min) + min
	return randNum
}


