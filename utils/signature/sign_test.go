package signature_test

import (
	"testing"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/json"
	"github.com/fr0ster/turbo-restler/utils/signature"
	"github.com/stretchr/testify/assert"
)

// Test 1: Sign HMAC
func TestParamsSignHMAC(t *testing.T) {
	func() {
		sign := signature.NewSignHMAC("apy_key", "apy_secret")
		params := simplejson.New()
		params.Set("timestamp", 1610612740000) // Час в мілісекундах, за звичай рахуємо як int64(time.Nanosecond)*time.Now().UnixNano()/int64(time.Millisecond)
		// Створення підпису
		signature, err := json.ConvertSimpleJSONToString(params)
		assert.Nil(t, err)
		params.Set("signature", sign.CreateSignature(signature))
		expected := `{"signature":"b9739a6b6322ff0490293f52807fc895cddf41cdb34c178b346589148fec3b66","timestamp":1610612740000}`
		result, err := params.MarshalJSON()
		assert.Nil(t, err)
		assert.Equal(t, expected, string(result))
	}()
}
