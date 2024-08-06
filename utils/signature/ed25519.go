package signature

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
)

type SignEd25519 struct {
	apiKey     string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// Функція для створення підпису Ed25519
func (sign *SignEd25519) CreateSignature(queryString string) string {
	signature := ed25519.Sign(sign.privateKey, []byte(queryString))
	return base64.StdEncoding.EncodeToString(signature)
}

func (sign *SignEd25519) GetAPIKey() string {
	return sign.apiKey
}

func NewSignEd25519(apiKey string, publicKeyFile string, privateKeyFile string) (signer *SignEd25519, err error) {
	private, err := loadEd25519PrivateKeyFromPEM(privateKeyFile)
	if err != nil {
		return
	}
	public, err := loadEd25519PublicKeyFromPEM(publicKeyFile)
	if err != nil {
		return
	}
	signer = &SignEd25519{
		apiKey:     apiKey,
		privateKey: ed25519.PrivateKey(private),
		publicKey:  ed25519.PublicKey(public),
	}
	return
}

// Функція для завантаження приватного ключа з PEM рядка
func loadEd25519PrivateKeyFromPEM(pemFile string) (ed25519.PrivateKey, error) {
	content, err := loadFile(pemFile)
	if err != nil {
		return nil, err
	}
	// Конвертуємо байтовий зріз у строку
	block, _ := pem.Decode([]byte(content))
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ed25519PrivateKey, ok := privateKey.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("not an Ed25519 private key")
	}

	return ed25519PrivateKey, nil
}

// Функція для завантаження публічного ключа з PEM рядка
func loadEd25519PublicKeyFromPEM(pemFile string) (ed25519.PublicKey, error) {
	content, err := loadFile(pemFile)
	if err != nil {
		return nil, err
	}
	// Конвертуємо байтовий зріз у строку
	block, _ := pem.Decode([]byte(content))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ed25519PublicKey, ok := publicKey.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("not an Ed25519 public key")
	}

	return ed25519PublicKey, nil
}
