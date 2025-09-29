package utils

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

func HashObject(any interface{}) (string, error) {
	out, err := json.Marshal(any)
	if err != nil {
		return "", err
	}
	sha := sha256.Sum256(out)
	return base64.RawURLEncoding.EncodeToString(sha[:]), nil
}
