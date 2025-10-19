package helper

import (
	"time"

	"golang.org/x/exp/rand"
)

func GenerateRandNum(min, max int) int {
	rand.Seed(uint64(time.Now().UnixNano()))

	return min + rand.Intn(max-min)
}
