package web_stream

import (
	"fmt"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_api"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type (
	// WsCallBackMap map of callback functions
	WsHandlerMap map[string]WsHandler

	// WsHandler handles messages
	WsHandler func(*simplejson.Json)

	// ErrHandler handles errors
	ErrHandler func(err error)
	WebStream  struct {
		stream      *web_api.WebApi
		callBackMap WsHandlerMap
		errHandler  ErrHandler
		quit        chan struct{}
		timeOut     time.Duration
	}
)

func (ws *WebStream) Socket() *web_api.WebApi {
	return ws.stream
}

func (ws *WebStream) Start() (err error) {
	if len(ws.callBackMap) != 0 {
		ws.stream.Socket().SetReadLimit(655350)
		go func() {
			for {
				select {
				case <-ws.quit:
					return
				default:
					response, err := ws.stream.Read()
					if err != nil {
						ws.errHandler(err)
					}
					for _, cb := range ws.callBackMap {
						cb(response)
					}
				}
			}
		}()
	} else {
		err = fmt.Errorf("no handlers")
	}

	return
}

func (ws *WebStream) Stop() {
	close(ws.quit)
}

func (ws *WebStream) Close() {
	ws.stream.Close()
}

func (ws *WebStream) SetHandler(handler WsHandler) *WebStream {
	ws.AddSubscriptions("default", handler)
	return ws
}

func (ws *WebStream) SetErrHandler(errHandler ErrHandler) *WebStream {
	ws.errHandler = errHandler
	return ws
}

func (ws *WebStream) SetTimerOut(duration time.Duration) {
	ws.timeOut = duration
}

func (ws *WebStream) AddSubscriptions(handlerId string, handler WsHandler) {
	ws.callBackMap[handlerId] = handler
}
func (ws *WebStream) RemoveSubscriptions(handlerId string) {
	ws.callBackMap[handlerId] = nil
	delete(ws.callBackMap, handlerId)
}

func New(
	host web_api.WsHost,
	path web_api.WsPath,
	scheme ...web_api.WsScheme) (stream *WebStream) {
	var err error
	if len(scheme) == 0 {
		scheme = append(scheme, web_api.SchemeWSS)
	}
	socket, err := web_api.New(host, path, scheme[0])
	if err != nil {
		return
	}
	// Встановлення обробника для ping повідомлень
	socket.Socket().SetPingHandler(func(appData string) error {
		logrus.Debug("Received ping:", appData)
		err := socket.Socket().WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		if err != nil {
			logrus.Debug("Error sending pong:", err)
		}
		return nil
	})
	stream = &WebStream{
		stream:      socket,
		callBackMap: make(WsHandlerMap, 0),
		quit:        make(chan struct{}),
		timeOut:     100 * time.Microsecond,
	}
	return
}
