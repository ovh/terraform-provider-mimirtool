package mimirtool

import (
	"math/rand"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func stringValueMap(src map[string]interface{}) map[string]string {
	dst := make(map[string]string)
	for k, val := range src {
		if val, ok := val.(string); ok {
			dst[k] = val
		}
	}
	return dst
}
