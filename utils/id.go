package utils

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"

	"github.com/golang-jwt/jwt"
)

func GenerateMD5ID(intent map[string]string) (string, error) {
	bytes, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	hasher := md5.New()
	_, err = hasher.Write(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func GenerateJWTID(intent map[string]string, salt string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	for key, value := range intent {
		claims[key] = value
	}

	tokenString, err := token.SignedString([]byte(salt))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
