package web_api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bitly/go-simplejson"
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
		silent     bool
		conn       *websocket.Conn
		errHandler func(err error)
	}
)

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
	err = wa.conn.WriteMessage(websocket.TextMessage, requestBody)
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
	_, body, err = wa.conn.ReadMessage()
	if err != nil {
		err = fmt.Errorf("error reading message: %v", err)
		return
	}
	response = wa.Deserialize(body)
	return
}

func (wa *WebApi) Socket() *websocket.Conn {
	return wa.conn
}

func (wa *WebApi) Close() (err error) {
	err = wa.conn.Close()
	if err != nil {
		err = fmt.Errorf("error closing connection: %v", err)
		return
	}
	wa.conn = nil
	wa = nil
	return
}

func (wa *WebApi) SetSilentMode(silent bool) {
	wa.silent = silent
}

func (wa *WebApi) SetPingHandler(handler ...func(appData string) error) {
	// Встановлення обробника для ping повідомлень
	if len(handler) == 0 {
		wa.conn.SetPingHandler(func(appData string) error {
			err := wa.conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
			if err != nil {
				wa.errorHandler(fmt.Errorf("error sending pong: %v", err))
			}
			return nil
		})
	} else {
		wa.conn.SetPingHandler(handler[0])
	}
}

func (wa *WebApi) SetErrorHandler(handler func(err error)) {
	wa.errHandler = handler
}

func (wa *WebApi) errorHandler(err error) {
	if wa.errHandler != nil && !wa.silent {
		wa.errHandler(err)
	}
}

func (wa *WebApi) IsOpen() bool {
	return wa.conn != nil
}

func (wa *WebApi) IsClosed() bool {
	return wa.conn == nil
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
	socket = &WebApi{
		silent: true,
		conn:   conn,
	}
	return
}
