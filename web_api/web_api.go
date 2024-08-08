package common

import (
	encoding_json "encoding/json"
	"fmt"
	"net/url"

	"github.com/fr0ster/turbo-restler/utils/json"
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

func ParseResponse(data []byte) (*Response, error) {
	var response Response
	err := encoding_json.Unmarshal(data, &response)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}
	return &response, nil
}

func ParseLimit(data []byte) ([]RateLimit, error) {
	var response []RateLimit
	err := encoding_json.Unmarshal(data, &response)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %v", err)
	}
	return response, nil
}

// Функція для розміщення ордера через WebSocket
func CallWebAPI(host, path string, method string, params interface{}) (response *Response, err error) {
	var requestBody []byte
	request := Request{
		ID:     uuid.New().String(),
		Method: method,
		Params: params,
	}
	// Серіалізація запиту в JSON
	requestBody, err = json.StructToSortedQueryByteArr(request)
	if err != nil {
		err = fmt.Errorf("error marshaling request: %v", err)
		return
	}
	// }

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
	response, err = ParseResponse(body)
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
