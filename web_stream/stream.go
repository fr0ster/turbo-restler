package web_stream

import (
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_api"
)

type (
	// WsCallBackMap map of callback functions
	WsHandlerMap map[string]WsHandler

	// WsHandler handles messages
	WsHandler func(*simplejson.Json)

	// ErrHandler handles errors
	ErrHandler func(err error)
	WebStream  struct {
		socket      *web_api.WebApi
		callBackMap WsHandlerMap
		errHandler  ErrHandler
		quit        chan struct{}
		timeOut     time.Duration
	}
)

func (ws *WebStream) Socket() *web_api.WebApi {
	return ws.socket
}

func (ws *WebStream) Start() (err error) {
	ws.socket.Socket().SetReadLimit(655350)
	go func() {
		for {
			select {
			case <-ws.quit:
				return
			default:
				response, err := ws.socket.Read()
				if err != nil {
					ws.errHandler(err)
				} else {
					for _, cb := range ws.callBackMap {
						cb(response)
					}
				}
			}
		}
	}()

	return
}

func (ws *WebStream) Stop() {
	close(ws.quit)
}

func (ws *WebStream) Close() {
	ws.socket.Close()
}

func (ws *WebStream) SetErrHandler(errHandler ErrHandler) *WebStream {
	ws.errHandler = errHandler
	return ws
}

func (ws *WebStream) SetTimerOut(duration time.Duration) *WebStream {
	ws.timeOut = duration
	return ws
}

func (ws *WebStream) AddHandler(handlerId string, handler WsHandler) *WebStream {
	if _, ok := ws.callBackMap[handlerId]; !ok {
		ws.callBackMap[handlerId] = handler
	}
	return ws
}

func (ws *WebStream) RemoveHandler(handlerId string) *WebStream {
	if _, ok := ws.callBackMap[handlerId]; ok {
		ws.callBackMap[handlerId] = nil
		delete(ws.callBackMap, handlerId)
	}
	return ws
}

func (ws *WebStream) SetPingHandler() {
	ws.socket.SetPingHandler()
}

func (ws *WebStream) SetErrorHandler(handler func(err error)) {
	ws.errHandler = handler
}

func (ws *WebStream) IsOpen() bool {
	return ws.socket.IsOpen()
}

func (ws *WebStream) IsClosed() bool {
	return ws.socket.IsClosed()
}

func New(
	host web_api.WsHost,
	path web_api.WsPath,
	scheme ...web_api.WsScheme) (stream *WebStream, err error) {
	if len(scheme) == 0 {
		scheme = append(scheme, web_api.SchemeWSS)
	}
	socket, err := web_api.New(host, path, scheme[0])
	if err != nil {
		return
	}
	stream = &WebStream{
		socket:      socket,
		callBackMap: make(WsHandlerMap, 0),
		quit:        make(chan struct{}),
		timeOut:     5 * time.Second,
	}
	stream.socket.SetPingHandler()
	return
}
