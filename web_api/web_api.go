package web_api

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/json"
	"github.com/fr0ster/turbo-restler/utils/signature"
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
	WebApi   struct {
		socket *websocket.Conn
	}
)

func (wa *WebApi) SignParameters(
	params *simplejson.Json,
	sign signature.Sign) (*simplejson.Json, error) {
	params.Set("timestamp", int64(time.Nanosecond)*time.Now().UnixNano()/int64(time.Millisecond))
	// Створення підпису
	signature, err := json.ConvertSimpleJSONToString(params)
	if err != nil {
		return nil, fmt.Errorf("error encoding params: %v", err)
	}
	params.Set("signature", sign.CreateSignature(signature))
	return params, nil
}

// Функція виклику Web API
func (wa *WebApi) Call(request *simplejson.Json) (response *simplejson.Json, err error) {
	var (
		requestBody []byte
	)
	// Серіалізація запиту в JSON
	requestBody, err = request.MarshalJSON()
	if err != nil {
		err = fmt.Errorf("error marshaling request: %v", err)
		return
	}
	defer wa.socket.Close()

	// Відправка запиту
	err = wa.socket.WriteMessage(websocket.TextMessage, requestBody)
	if err != nil {
		err = fmt.Errorf("error sending message: %v", err)
		return
	}

	// Читання відповіді
	_, body, err := wa.socket.ReadMessage()
	response, err = simplejson.NewJson(body)
	return
}

func (wa *WebApi) Socket() *websocket.Conn {
	return wa.socket
}

func New(
	host WsHost,
	path WsPath,
	scheme ...WsScheme) (socket *WebApi, err error) { // Підключення до WebSocket
	if len(scheme) == 0 {
		scheme = append(scheme, SchemeWSS)
	}
	u := url.URL{Scheme: string(scheme[0]), Host: string(host), Path: string(path)}
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
	socket = &WebApi{socket: conn}
	return
}
