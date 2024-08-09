package web_api_test

import (
	"testing"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/json"
	web_api "github.com/fr0ster/turbo-restler/web_api"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCallWebAPI(t *testing.T) {
	request := &web_api.Request{
		ID:     uuid.New().String(),
		Method: "order.place",
		Params: simplejson.New(),
	}
	request.SetParameter("apiKey", "test")
	request.SetParameter("symbol", "BTCUSDT")
	request.SetParameter("side", "BUY")
	request.SetParameter("type", "LIMIT")
	request.SetParameter("quantity", "0.01")
	request.SetParameter("price", "10000")
	request.SetParameter("timeInForce", "GTC")
	params, err := json.ConvertSimpleJSONToString(request.Params)
	assert.Nil(t, err)
	assert.NotNil(t, params)
	assert.Equal(t, "apiKey=test&price=10000&quantity=0.01&side=BUY&symbol=BTCUSDT&timeInForce=GTC&type=LIMIT", params)
}
