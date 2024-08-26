package web_stream

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_api"
	"github.com/sirupsen/logrus"
)

const (
	SUBSCRIBE_ID StreamAction = iota
	LIST_SUBSCRIPTIONS_ID
	UNSUBSCRIBE_ID
)

type (
	// Define the enum type
	StreamAction int
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
					} else {
						for _, cb := range ws.callBackMap {
							cb(response)
						}
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

func (ws *WebStream) SetDefaultHandler(handler WsHandler) *WebStream {
	ws.AddHandler("default", handler)
	return ws
}

func (ws *WebStream) RemoveDefaultHandler() {
	ws.RemoveHandler("default")
}

func (ws *WebStream) SetErrHandler(errHandler ErrHandler) *WebStream {
	ws.errHandler = errHandler
	return ws
}

func (ws *WebStream) SetTimerOut(duration time.Duration) {
	ws.timeOut = duration
}

func (ws *WebStream) AddHandler(handlerId string, handler WsHandler) (err error) {
	if _, ok := ws.callBackMap[handlerId]; !ok {
		ws.callBackMap[handlerId] = handler
	} else {
		err = fmt.Errorf("handler already exists")
	}
	return
}

func (ws *WebStream) RemoveHandler(handlerId string) (err error) {
	if _, ok := ws.callBackMap[handlerId]; ok {
		ws.callBackMap[handlerId] = nil
		delete(ws.callBackMap, handlerId)
	} else {
		err = fmt.Errorf("handler not found")
	}
	return
}

func (ws *WebStream) Subscribe(handlerId string) (err error) {
	if _, ok := ws.callBackMap[handlerId]; ok {
		// Send subscription request
		rq := simplejson.New()
		rq.Set("method", "SUBSCRIBE")
		rq.Set("id", SUBSCRIBE_ID)
		rq.Set("params", handlerId)
		err = ws.stream.Send(rq)
	} else {
		err = fmt.Errorf("handler not found")
	}
	return
}

func (ws *WebStream) ListOfSubscriptions(handler WsHandler) (err error) {
	if _, ok := ws.callBackMap[strconv.Itoa(int(LIST_SUBSCRIPTIONS_ID))]; ok {
		// Send subscription request
		rq := simplejson.New()
		rq.Set("method", "LIST_SUBSCRIPTIONS")
		rq.Set("id", LIST_SUBSCRIPTIONS_ID)
		err := ws.stream.Send(rq)
		if err != nil {
			logrus.Fatalf("Error: %v", err)
		}
	} else {
		err = fmt.Errorf("handler not found")
	}
	return
}

func (ws *WebStream) Unsubscribe(handlerId string, handler WsHandler) {
	if _, ok := ws.callBackMap[handlerId]; !ok {
		// Send subscription request
		rq := simplejson.New()
		rq.Set("method", "UNSUBSCRIBE")
		rq.Set("id", UNSUBSCRIBE_ID)
		rq.Set("params", handlerId)
		err := ws.stream.Send(rq)
		if err != nil {
			logrus.Fatalf("Error: %v", err)
		}
	}
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
		stream:      socket,
		callBackMap: make(WsHandlerMap, 0),
		quit:        make(chan struct{}),
		timeOut:     100 * time.Microsecond,
	}
	return
}
