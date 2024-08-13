package web_api_test

import (
	"testing"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/json"
	"github.com/fr0ster/turbo-restler/utils/signature"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCallWebAPI(t *testing.T) {
	params := simplejson.New()
	params.Set("apiKey", "test")
	params.Set("symbol", "BTCUSDT")
	params.Set("side", "BUY")
	params.Set("type", "LIMIT")
	params.Set("quantity", "0.01")
	params.Set("price", "10000")
	params.Set("timeInForce", "GTC")

	signature, err := json.ConvertSimpleJSONToString(params)
	expected := `apiKey=test&price=10000&quantity=0.01&side=BUY&symbol=BTCUSDT&timeInForce=GTC&type=LIMIT`
	assert.Nil(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, expected, signature)
	expected = `{"apiKey":"test","price":"10000","quantity":"0.01","side":"BUY","symbol":"BTCUSDT","timeInForce":"GTC","type":"LIMIT"}`
	js, err := params.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, expected, string(js))

	request := simplejson.New()
	request.Set("ID", uuid.New().String())
	request.Set("Method", "order.place")
	request.Set("Params", params)
	assert.NotNil(t, request)
	js, err = request.MarshalJSON()
	assert.Nil(t, err)
	expected = `{"ID":"` + request.Get("ID").MustString() + `","Method":"order.place","Params":{"apiKey":"test","price":"10000","quantity":"0.01","side":"BUY","symbol":"BTCUSDT","timeInForce":"GTC","type":"LIMIT"}}`
	assert.Equal(t, expected, string(js))
}

func TestSignWithParameters(t *testing.T) {
	params := simplejson.New()
	params.Set("apiKey", "test")
	params.Set("symbol", "BTCUSDT")
	params.Set("side", "BUY")
	params.Set("type", "LIMIT")
	params.Set("quantity", "0.01")
	params.Set("price", "10000")
	params.Set("timeInForce", "GTC")

	sign := signature.NewSignHMAC("test", "test")
	params.Set("timestamp", "1610612740000")
	signature, err := json.ConvertSimpleJSONToString(params)
	assert.Nil(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, "apiKey=test&price=10000&quantity=0.01&side=BUY&symbol=BTCUSDT&timeInForce=GTC&timestamp=1610612740000&type=LIMIT", signature)
	params.Set("signature", sign.CreateSignature(signature))
	expected := `{"apiKey":"test","price":"10000","quantity":"0.01","side":"BUY","signature":"6d9a0950b85305b15641d2c5ef1dfbecfd49589686111de513a0f9a710fdb6f4","symbol":"BTCUSDT","timeInForce":"GTC","timestamp":"1610612740000","type":"LIMIT"}`
	result, err := params.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, expected, string(result))
}

func TestSignWithOutParameters(t *testing.T) {
	params := simplejson.New()
	sign := signature.NewSignHMAC("test", "test")
	params.Set("timestamp", "1610612740000")
	signature, err := json.ConvertSimpleJSONToString(params)
	assert.Nil(t, err)
	assert.NotNil(t, signature)
	assert.Equal(t, "timestamp=1610612740000", signature)
	params.Set("signature", sign.CreateSignature(signature))
	expected := `{"signature":"3f4ec1bd61d3b84b61242cf6ca7b2248402e4ea9d4b78f42917847aa2c577082","timestamp":"1610612740000"}`
	result, err := params.MarshalJSON()
	assert.Nil(t, err)
	assert.Equal(t, expected, string(result))
}
