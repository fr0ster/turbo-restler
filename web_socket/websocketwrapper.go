package web_socket

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type LogOp string

const (
	OpReceive LogOp = "receive"
	OpSend    LogOp = "send"
)

type MessageKind int

const (
	KindData MessageKind = iota
	KindError
	KindControl
)

type LogRecord struct {
	Op   LogOp
	Body []byte
	Err  error
}

type MessageEvent struct {
	Kind  MessageKind
	Body  []byte
	Error error
}

type WriteCallback func(error)

type WriteEvent struct {
	Body     []byte
	Callback WriteCallback
	ErrChan  chan error
}

type WebApiControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

type WebApiReader interface {
	ReadMessage() (int, []byte, error)
}

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type WebSocketInterface interface {
	Open()
	Close()
	Send(writeEvent WriteEvent) error
	Subscribe(f func(MessageEvent)) int
	Unsubscribe(id int)
	SetMessageLogger(f func(LogRecord))
	SetPingHandler(f func(string) error)
	SetReadTimeout(time.Duration)
	SetWriteTimeout(time.Duration)
	GetControl() WebApiControlWriter
	GetReader() WebApiReader
	GetWriter() WebApiWriter
	Started() <-chan struct{}
	WaitAllLoops(timeout time.Duration) bool
	ResetLoops()
	Done() <-chan struct{}
}

type WebSocketWrapper struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	readMu   sync.Mutex
	writeMu  sync.Mutex
	sendChan chan WriteEvent

	readTimeout  *time.Duration
	writeTimeout *time.Duration

	readIsWorked  atomic.Bool
	writeIsWorked atomic.Bool

	startedOnce sync.Once
	started     chan struct{}

	logger func(LogRecord)

	subsMu    sync.RWMutex
	subs      map[int]func(MessageEvent)
	subIDGen  atomic.Int32
	readPause atomic.Bool

	readLoopDone  chan struct{}
	writeLoopDone chan struct{}
}

func NewWebSocketWrapper(conn *websocket.Conn) *WebSocketWrapper {
	return &WebSocketWrapper{
		conn:          conn,
		sendChan:      make(chan WriteEvent, 128),
		started:       make(chan struct{}),
		subs:          make(map[int]func(MessageEvent)),
		readLoopDone:  make(chan struct{}),
		writeLoopDone: make(chan struct{}),
	}
}

func (w *WebSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	w.logger = f
}

func (w *WebSocketWrapper) SetPingHandler(f func(string) error) {
	w.conn.SetPingHandler(f)
}

func (w *WebSocketWrapper) SetReadTimeout(timeout time.Duration) {
	if w.readTimeout == nil {
		w.readTimeout = new(time.Duration)
	}
	*w.readTimeout = timeout
}

func (w *WebSocketWrapper) SetWriteTimeout(timeout time.Duration) {
	if w.writeTimeout == nil {
		w.writeTimeout = new(time.Duration)
	}
	*w.writeTimeout = timeout
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

func (w *WebSocketWrapper) emit(evt MessageEvent) {
	w.subsMu.RLock()
	defer w.subsMu.RUnlock()
	for _, handler := range w.subs {
		handler(evt)
	}
}

func (w *WebSocketWrapper) Started() <-chan struct{} {
	return w.started
}

func (w *WebSocketWrapper) Done() <-chan struct{} {
	if w.ctx != nil {
		return w.ctx.Done()
	}
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (w *WebSocketWrapper) Open() {
	w.ctx, w.cancel = context.WithCancel(context.Background())
	go w.readLoop(w.ctx)
	go w.writeLoop(w.ctx)
	w.startedOnce.Do(func() {
		close(w.started)
	})
}

func (w *WebSocketWrapper) Send(evt WriteEvent) error {
	select {
	case w.sendChan <- evt:
		return nil
	case <-w.ctx.Done():
		return errors.New("connection is closed")
	}
}

func (w *WebSocketWrapper) readLoop(ctx context.Context) {
	w.readIsWorked.Store(true)
	defer func() {
		w.readIsWorked.Store(false)
		close(w.readLoopDone)
	}()

	for {
		if w.readPause.Load() {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		w.readMu.Lock()
		if w.readTimeout != nil {
			_ = w.conn.SetReadDeadline(time.Now().Add(*w.readTimeout))
		}
		typ, msg, err := w.conn.ReadMessage()
		w.readMu.Unlock()

		if w.logger != nil {
			w.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
		}

		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			w.emit(MessageEvent{Kind: KindError, Error: err})
			return
		}

		kind := KindControl
		if typ == websocket.TextMessage || typ == websocket.BinaryMessage {
			kind = KindData
		}
		w.emit(MessageEvent{Kind: kind, Body: msg})
	}
}

func (w *WebSocketWrapper) writeLoop(ctx context.Context) {
	w.writeIsWorked.Store(true)
	defer func() {
		w.writeIsWorked.Store(false)
		close(w.writeLoopDone)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-w.sendChan:
			w.writeMu.Lock()
			if w.writeTimeout != nil {
				_ = w.conn.SetWriteDeadline(time.Now().Add(*w.writeTimeout))
			}
			err := w.conn.WriteMessage(websocket.TextMessage, evt.Body)
			w.writeMu.Unlock()

			if w.logger != nil {
				w.logger(LogRecord{Op: OpSend, Body: evt.Body, Err: err})
			}

			if evt.Callback != nil {
				evt.Callback(err)
			}
			if evt.ErrChan != nil {
				evt.ErrChan <- err
			}
		}
	}
}

func (w *WebSocketWrapper) PauseLoops() {
	w.readPause.Store(true)
}

func (w *WebSocketWrapper) ResumeLoops() {
	w.readPause.Store(false)
}

func (w *WebSocketWrapper) ResetLoops() {
	w.PauseLoops()
	w.ResumeLoops()
}

func (w *WebSocketWrapper) WaitAllLoops(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-w.readLoopDone:
		select {
		case <-w.writeLoopDone:
			return true
		case <-timer.C:
			return false
		}
	case <-timer.C:
		return false
	}
}

func (w *WebSocketWrapper) GetControl() WebApiControlWriter {
	return w.conn
}

func (w *WebSocketWrapper) GetReader() WebApiReader {
	return w.conn
}

func (w *WebSocketWrapper) GetWriter() WebApiWriter {
	return w.conn
}

func (w *WebSocketWrapper) Close() {
	w.cancel()
	_ = w.conn.Close()
}
