package websocket

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

func init() {
	// Configure logrus with a default formatter and log level
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetLevel(logrus.DebugLevel)
}

type WebSocketWrapper struct {
	conn            *websocket.Conn
	readLoopStopper chan struct{}
	readLoopDone    chan struct{}
	readLoopStarted chan struct{}
	// writeLoopStopper chan struct{}
	// writeLoopDone    chan struct{}
	// writeLoopStarted chan struct{}
}

func New(d *websocket.Dialer, url string) (*WebSocketWrapper, error) {
	conn, _, err := d.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	return &WebSocketWrapper{
		conn:            conn,
		readLoopStopper: make(chan struct{}, 1),
		readLoopDone:    make(chan struct{}, 1),
		readLoopStarted: make(chan struct{}, 1),
		// writeLoopStopper: make(chan struct{}),
		// writeLoopDone:    make(chan struct{}),
		// writeLoopStarted: make(chan struct{}),
	}, nil
}

// Privet functions

// read loops
func (w *WebSocketWrapper) readLoop() {
	w.readLoopStarted <- struct{}{}
	for {
		select {
		case <-w.readLoopStopper:
			w.readLoopDone <- struct{}{}
			return
		default:
		}

		// err := w.conn.SetReadDeadline(time.Now().Add(1000 * time.Millisecond))
		// if err != nil {
		// 	logrus.Errorf("SetReadDeadline error: %v", err)
		// 	break
		// }

		_, body, err := w.conn.ReadMessage()
		if err != nil {
			logrus.Errorf("ReadMessage error: %v", err)
			break
		}
		logrus.Debugf("Read message: %s", string(body))

		// 🔽 Перевіримо ще раз — **readLoopStopper** могло бути закрито під час читання
		select {
		case <-w.readLoopStopper:
			_, body, err := w.conn.ReadMessage()
			if err != nil {
				logrus.Errorf("ReadMessage final error: %v", err)
				break
			}
			logrus.Debugf("Read final message: %s", string(body))
			w.readLoopDone <- struct{}{}
			return
		default:
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// Public functions
func (w *WebSocketWrapper) Open() {
	// Regenerate control channels
	w.readLoopStopper = make(chan struct{}, 1)
	w.readLoopDone = make(chan struct{}, 1)
	w.readLoopStarted = make(chan struct{}, 1)
	// Start the read loop
	go w.readLoop()
	<-w.readLoopStarted
}
func (w *WebSocketWrapper) Halt() {
	f := func(timeOut time.Duration) bool {
		if timeOut != 0 {
			_ = w.conn.SetReadDeadline(time.Now().Add(timeOut))
		} else {
			// Stop the read loop
			close(w.readLoopStopper)
		}
		select {
		case <-w.readLoopDone:
			return true
		case <-time.After(2 * time.Second):
			return false
		}
	}
	if !f(0) {
		f(200 * time.Millisecond)
	}
}
func (w *WebSocketWrapper) Close() {
	// Close the connection
	w.conn.Close()
}
