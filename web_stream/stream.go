package web_stream

import (
	"net/http"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_api"
	"github.com/gorilla/websocket"
)

var (
	// WebsocketTimeout is an interval for sending ping/pong messages if WebsocketKeepalive is enabled
	WebsocketTimeout = time.Second * 60
	// WebsocketKeepalive enables sending ping/pong messages to check the connection stability
	WebsocketKeepalive = true
)

// WsHandler handle raw websocket message
type WsHandler func(message *simplejson.Json)

// ErrHandler handles errors
type ErrHandler func(err error)

func StartStreamer(
	host web_api.WsHost,
	path web_api.WsPath,
	handler WsHandler,
	errHandler ErrHandler,
	scheme ...web_api.WsScheme) (doneC, stopC chan struct{}, err error) {
	if len(scheme) == 0 {
		scheme = append(scheme, web_api.SchemeWSS)
	}

	// Підключення до WebSocket
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: false,
	}
	c, _, err := Dialer.Dial(string(scheme[0])+"://"+string(host)+string(path), nil)
	if err != nil {
		return nil, nil, err
	}
	c.SetReadLimit(655350)
	doneC = make(chan struct{})
	stopC = make(chan struct{})
	go func() {
		// This function will exit either on error from
		// websocket.Conn.ReadMessage or when the stopC channel is
		// closed by the client.
		defer close(doneC)
		if WebsocketKeepalive {
			keepAlive(c, WebsocketTimeout)
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
			c.Close()
		}()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				if !silent {
					errHandler(err)
				}
				return
			}
			json, err := simplejson.NewJson(message)
			if err != nil {
				json = simplejson.New()
				json.Set("message", message)
				return
			}
			handler(json)
		}
	}()
	return
}

func keepAlive(c *websocket.Conn, timeout time.Duration) {
	ticker := time.NewTicker(timeout)

	lastResponse := time.Now()
	c.SetPongHandler(func(msg string) error {
		lastResponse = time.Now()
		return nil
	})

	go func() {
		defer ticker.Stop()
		for {
			deadline := time.Now().Add(10 * time.Second)
			err := c.WriteControl(websocket.PingMessage, []byte{}, deadline)
			if err != nil {
				return
			}
			<-ticker.C
			if time.Since(lastResponse) > timeout {
				c.Close()
				return
			}
		}
	}()
}
