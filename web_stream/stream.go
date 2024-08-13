package web_stream

import (
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_api"
	"github.com/gorilla/websocket"
)

type (
	// WsSubscribe handle subscribe messages
	WsSubscribe func() (*simplejson.Json, error)

	// WsUnsubscribe handle unsubscribe messages
	WsUnsubscribe func() (*simplejson.Json, error)

	// WsHandler handle raw websocket message
	WsHandler func(message *simplejson.Json)

	// ErrHandler handles errors
	ErrHandler func(err error)
	WebStream  struct {
		stream             *web_api.WebApi
		handler            WsHandler
		errHandler         ErrHandler
		websocketTimeout   time.Duration
		websocketKeepalive bool
	}
)

func (ws *WebStream) Socket() *web_api.WebApi {
	return ws.stream
}

func (ws *WebStream) Start() (doneC, stopC chan struct{}, err error) {
	ws.stream.Socket().SetReadLimit(655350)
	doneC = make(chan struct{})
	stopC = make(chan struct{})
	go func() {
		// This function will exit either on error from
		// websocket.Conn.ReadMessage or when the stopC channel is
		// closed by the client.
		defer close(doneC)
		if ws.websocketKeepalive {
			ws.keepAlive(ws.websocketTimeout)
		}
		// Wait for the stopC channel to be closed.  We do that in a
		// separate goroutine because ReadMessage is a blocking
		// operation.
		silent := false
		go func() {
			select {
			case <-stopC:
				silent = true
			case <-doneC:
			}
			ws.stream.Socket().Close()
		}()
		for {
			_, message, err := ws.stream.Socket().ReadMessage()
			if err != nil {
				if !silent && ws.errHandler != nil {
					ws.errHandler(err)
				}
				return
			}
			json, err := simplejson.NewJson([]byte(message))
			if err != nil {
				json = simplejson.New()
				json.Set("message", string(message))
			}
			if ws.handler != nil {
				ws.handler(json)
			}
		}
	}()

	return
}

func (ws *WebStream) keepAlive(timeout time.Duration) {
	ticker := time.NewTicker(timeout)

	lastResponse := time.Now()
	ws.stream.Socket().SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})

	go func() {
		defer ticker.Stop()
		for {
			deadline := time.Now().Add(10 * time.Second)
			err := ws.stream.Socket().WriteControl(websocket.PingMessage, []byte{}, deadline)
			if err != nil {
				return
			}
			<-ticker.C
			if time.Since(lastResponse) > timeout {
				ws.stream.Socket().Close()
				return
			}
		}
	}()
}

func New(
	host web_api.WsHost,
	path web_api.WsPath,
	handler WsHandler,
	errHandler ErrHandler,
	websocketKeepalive bool,
	websocketTimeout time.Duration,
	scheme ...web_api.WsScheme) (stream *WebStream, err error) {
	if len(scheme) == 0 {
		scheme = append(scheme, web_api.SchemeWSS)
	}
	socket, err := web_api.New(host, path, scheme[0])
	if err != nil {
		return
	}
	stream = &WebStream{
		stream:             socket,
		handler:            handler,
		errHandler:         errHandler,
		websocketTimeout:   websocketTimeout,
		websocketKeepalive: websocketKeepalive,
	}
	return
}
