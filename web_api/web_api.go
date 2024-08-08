package common

import (
	encoding_json "encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/signature"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type (
	Request struct {
		ID     string      `json:"id"`
		Method string      `json:"method"`
		Params interface{} `json:"params"`
	}
	Response struct {
		ID         string      `json:"id"`
		Status     int         `json:"status"`
		Error      ErrorDetail `json:"error"`
		Result     interface{} `json:"result"`
		RateLimits []RateLimit `json:"rateLimits"`
	}

	ErrorDetail struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	RateLimit struct {
		RateLimitType string `json:"rateLimitType"`
		Interval      string `json:"interval"`
		IntervalNum   int    `json:"intervalNum"`
		Limit         int    `json:"limit"`
		Count         int    `json:"count"`
	}
)

func (rq *Request) SetParameter(name string, value string) {
	if rq.Params == nil {
		rq.Params = url.Values{}
	}
	params := rq.Params.(url.Values)
	params.Set(name, value)
}

func parseResponse(data []byte) (*Response, error) {
	var response Response
	err := encoding_json.Unmarshal(data, &response)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}
	return &response, nil
}

// Функція виклику Web API
func CallWebAPI(host, path string, method string, params *simplejson.Json, sign signature.Sign) (response *Response, err error) {
	var (
		signature   []byte
		requestBody []byte
	)
	if params != nil && sign == nil {
		err = fmt.Errorf("sign is required")
		return
	}
	if params != nil {
		params.Set("timestamp", int64(time.Nanosecond)*time.Now().UnixNano()/int64(time.Millisecond))
		// Створення підпису
		signature, err = params.Encode()
		if err != nil {
			err = fmt.Errorf("error encoding params: %v", err)
			return
		}
		params.Set("signature", sign.CreateSignature(string(signature)))
	}
	request := Request{
		ID:     uuid.New().String(),
		Method: method,
		Params: params,
	}
	// Серіалізація запиту в JSON
	requestBody, err = encoding_json.Marshal(request)
	if err != nil {
		err = fmt.Errorf("error marshaling request: %v", err)
		return
	}

	// Підключення до WebSocket
	u := url.URL{Scheme: "wss", Host: host, Path: path}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		err = fmt.Errorf("error connecting to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	// Відправка запиту на розміщення ордера
	err = conn.WriteMessage(websocket.TextMessage, requestBody)
	if err != nil {
		err = fmt.Errorf("error sending message: %v", err)
		return
	}

	// Читання відповіді
	_, body, err := conn.ReadMessage()
	response, err = parseResponse(body)
	if err != nil {
		err = fmt.Errorf("error parsing response: %v", err)
		return
	}
	if response.Status != 200 {
		err = fmt.Errorf("error request: %v", response.Error)
		return
	}
	return
}
