package signature

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

type SignHMAC struct {
	apiSecret string
	apiKey    string
}

// Функція для створення підпису
func (sign *SignHMAC) CreateSignature(queryString string) string {
	h := hmac.New(sha256.New, []byte(sign.apiSecret))
	h.Write([]byte(queryString))
	return hex.EncodeToString(h.Sum(nil))
}

func (sign *SignHMAC) GetAPIKey() string {
	return sign.apiKey
}

func NewSignHMAC(apiKey PublicKey, apiSecret SecretKey) *SignHMAC {
	return &SignHMAC{
		apiSecret: string(apiSecret),
		apiKey:    string(apiKey),
	}
}
