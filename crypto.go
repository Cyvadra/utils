package utils

import (
	"crypto/md5"
	"encoding/hex"
)

func GetMd5(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func GetMd51(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func GetMd52(text string) string {
	hash := md5.Sum([]byte(text))
	hash = md5.Sum([]byte(hex.EncodeToString(hash[:])))
	return hex.EncodeToString(hash[:])
}
