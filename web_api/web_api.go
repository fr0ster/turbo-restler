package web_api

import (
	encoding_json "encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/json"
	"github.com/fr0ster/turbo-restler/utils/signature"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// WsScheme is a scheme for WebSocket
	SchemeWSS   WsScheme = "wss"
	SchemeWS    WsScheme = "ws"
	SchemeHTTP  WsScheme = "http"
	SchemeHTTPS WsScheme = "https"
)

type (
	WsScheme string
	WsHost   string
	WsPath   string
	Method   string
	Request  struct {
		ID     string           `json:"id"`
		Method Method           `json:"method"`
		Params *simplejson.Json `json:"params"`
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
		rq.Params = simplejson.New()
	}
	rq.Params.Set(name, value)
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
func CallWebAPI(
	host WsHost,
	path WsPath,
	method Method,
	params *simplejson.Json,
	sign signature.Sign) (response *Response, err error) {
	var (
		signature   string
		requestBody []byte
	)
	if params != nil && sign == nil {
		err = fmt.Errorf("sign is required")
		return
	}
	if params != nil {
		params.Set("timestamp", int64(time.Nanosecond)*time.Now().UnixNano()/int64(time.Millisecond))
		// Створення підпису
		signature, err = json.ConvertSimpleJSONToString(params)
		if err != nil {
			err = fmt.Errorf("error encoding params: %v", err)
			return
		}
		params.Set("signature", sign.CreateSignature(signature))
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
	u := url.URL{Scheme: "wss", Host: string(host), Path: string(path)}
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: false,
	}
	conn, _, err := Dialer.Dial(u.String(), nil)
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
