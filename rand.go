package utils

import (
	"math/rand"
	"time"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ012356789")
var nums = []rune("1203546879")
var s []rune

func init() {
	rand.Seed((time.Now().UnixNano() % 12345678901) + rand.Int63n(123400000))
}

func RandString(n int) (str string) {
	s = make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	str = string(s)
	return
}

func RandNum(n int) (str string) {
	s = make([]rune, n)
	for i := range s {
		s[i] = nums[rand.Intn(len(nums))]
	}
	str = string(s)
	return
}

func RandInt(n int) (N int) {
	N = rand.Intn(n)
	return
}
