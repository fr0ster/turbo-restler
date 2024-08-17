package web_api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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

// Функція виклику Web API
func (wa *WebApi) Call(request *simplejson.Json) (response *simplejson.Json, err error) {
	// Відправка запиту
	err = wa.Send(request)
	if err != nil {
		return
	}

	// Читання відповіді
	response, err = wa.Read()
	return
}

// Серіалізація запиту в JSON
func (wa *WebApi) Serialize(request *simplejson.Json) (requestBody []byte) {
	requestBody, _ = request.MarshalJSON()
	return
}

// Десеріалізація відповіді
func (wa *WebApi) Deserialize(body []byte) (response *simplejson.Json) {
	response, err := simplejson.NewJson(body)
	if err != nil {
		response = simplejson.New()
		response.Set("response", string(body))
	}
	return
}

// Відправка запиту
func (wa *WebApi) Send(request *simplejson.Json) (err error) {
	// Серіалізація запиту в JSON
	requestBody := wa.Serialize(request)

	// Відправка запиту
	err = wa.socket.WriteMessage(websocket.TextMessage, requestBody)
	if err != nil {
		err = fmt.Errorf("error sending message: %v", err)
		return
	}
	return
}

// Читання відповіді
func (wa *WebApi) Read() (response *simplejson.Json, err error) {
	var (
		body []byte
	)
	_, body, err = wa.socket.ReadMessage()
	if err != nil {
		err = fmt.Errorf("error reading message: %v", err)
		return
	}
	response = wa.Deserialize(body)
	return
}

func (wa *WebApi) Socket() *websocket.Conn {
	return wa.socket
}

func (wa *WebApi) Close() (err error) {
	err = wa.socket.Close()
	if err != nil {
		err = fmt.Errorf("error closing connection: %v", err)
		return
	}
	wa.socket = nil
	wa = nil
	return
}

func New(
	host WsHost,
	path WsPath,
	scheme ...WsScheme) (socket *WebApi, err error) { // Підключення до WebSocket
	if len(scheme) == 0 {
		scheme = append(scheme, SchemeWSS)
	}
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: false,
	}
	conn, _, err := Dialer.Dial(string(scheme[0])+"://"+string(host)+string(path), nil)
	if err != nil {
		return
	}
	// Встановлення обробника для ping повідомлень
	conn.SetPingHandler(func(appData string) error {
		logrus.Debug("Received ping:", appData)
		err := conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		panic(err)
	})
	socket = &WebApi{socket: conn}
	return
}
