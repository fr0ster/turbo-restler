package websocket

import (
	"sync"
	"sync/atomic"
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

type MessageKind int

const (
	KindData MessageKind = iota
	KindError
)

type MessageEvent struct {
	Kind  MessageKind
	Body  []byte
	Error error
}

type WebSocketWrapper struct {
	conn            *websocket.Conn
	readLoopStopper chan struct{}
	readLoopDone    chan struct{}
	readLoopStarted chan struct{}

	subs     map[int]func(MessageEvent)
	subsMu   sync.Mutex
	subIDGen atomic.Int32
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
		subs:            make(map[int]func(MessageEvent)),
	}, nil
}

func (w *WebSocketWrapper) readLoop() {
	w.readLoopStarted <- struct{}{}
	for {
		select {
		case <-w.readLoopStopper:
			w.readLoopDone <- struct{}{}
			return
		default:
		}

		_, body, err := w.conn.ReadMessage()
		if err != nil {
			logrus.Errorf("ReadMessage error: %v", err)
			w.emitToSubscribers(MessageEvent{Kind: KindError, Error: err})
			break
		}

		logrus.Debugf("Read message: %s", string(body))
		w.emitToSubscribers(MessageEvent{Kind: KindData, Body: body})

		select {
		case <-w.readLoopStopper:
			w.readLoopDone <- struct{}{}
			return
		default:
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (w *WebSocketWrapper) emitToSubscribers(event MessageEvent) {
	w.subsMu.Lock()
	defer w.subsMu.Unlock()
	for _, f := range w.subs {
		go f(event) // notify each subscriber in a goroutine
	}
}

func (w *WebSocketWrapper) Open() {
	w.readLoopStopper = make(chan struct{}, 1)
	w.readLoopDone = make(chan struct{}, 1)
	w.readLoopStarted = make(chan struct{}, 1)
	go w.readLoop()
	<-w.readLoopStarted
}

func (w *WebSocketWrapper) Halt() {
	f := func(timeOut time.Duration) bool {
		if timeOut != 0 {
			_ = w.conn.SetReadDeadline(time.Now().Add(timeOut))
		} else {
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
	w.conn.Close()
}

func (w *WebSocketWrapper) Subscribe(f func(MessageEvent)) int {
	id := int(w.subIDGen.Add(1))
	w.subsMu.Lock()
	w.subs[id] = f
	w.subsMu.Unlock()
	return id
}

func (w *WebSocketWrapper) Unsubscribe(id int) {
	w.subsMu.Lock()
	delete(w.subs, id)
	w.subsMu.Unlock()
}
