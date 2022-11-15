package mimirtool

import (
	"crypto/sha256"
	"encoding/hex"
)

func hash(s string) string {
	sha := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sha[:])
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
