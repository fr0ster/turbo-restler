package signature

type (
	PublicKey string
	SecretKey string
	Sign      interface {
		CreateSignature(queryString string) string
		GetAPIKey() string
	}
)
