package signature

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"log"
)

type SignRSA struct {
	apiKey     string
	privateKey rsa.PrivateKey
	publicKey  rsa.PublicKey
}

// Функція для створення підпису RSA
func (sign *SignRSA) CreateSignature(queryString string) string {
	hashed := sha256.Sum256([]byte(queryString))
	signature, err := rsa.SignPKCS1v15(rand.Reader, &sign.privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		log.Fatalf("Error signing query string: %v", err)
	}
	return base64.StdEncoding.EncodeToString(signature)
}

func (sign *SignRSA) GetAPIKey() string {
	return sign.apiKey
}

func NewSignRSA(apiKey string, publicKeyFile string, privateKeyFile string) (sign *SignRSA, err error) {
	private, err := loadRSAPrivateKeyFromPEM(privateKeyFile)
	if err != nil {
		return
	}
	public, err := loadRSAPublicKeyFromPEM(publicKeyFile)
	if err != nil {
		return
	}

	sign = &SignRSA{
		apiKey:     apiKey,
		privateKey: *private,
		publicKey:  *public,
	}
	return
}

// Функція для завантаження приватного ключа з PEM рядка
func loadRSAPrivateKeyFromPEM(pemFile string) (*rsa.PrivateKey, error) {
	// Конвертуємо байтовий зріз у строку
	content, err := loadFile(pemFile)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(content))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// Функція для завантаження публічного ключа з PEM рядка
func loadRSAPublicKeyFromPEM(pemFile string) (*rsa.PublicKey, error) {
	// Конвертуємо байтовий зріз у строку
	content, err := loadFile(pemFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode([]byte(content))
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return publicKey, nil
}
