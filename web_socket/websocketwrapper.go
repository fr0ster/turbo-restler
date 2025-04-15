package web_socket

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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
	Halt() bool
	Close() error
	Send(writeEvent WriteEvent) error
	Subscribe(f func(MessageEvent)) int
	Unsubscribe(id int)
	UnsubscribeAll()
	SetMessageLogger(f func(LogRecord))
	SetPingHandler(f func(string) error)
	SetReadTimeout(time.Duration)
	SetWriteTimeout(time.Duration)
	GetTimeout() time.Duration
	SetTimeout(time.Duration)
	GetControl() WebApiControlWriter
	GetReader() WebApiReader
	GetWriter() WebApiWriter
	Started() <-chan struct{}
	WaitAllLoops(timeout time.Duration) bool
	Resume()
	Done() <-chan struct{}
}

type WebSocketWrapper struct {
	conn   *websocket.Conn
	dialer *websocket.Dialer
	url    string

	ctx    context.Context
	cancel context.CancelFunc

	readMu   sync.Mutex
	writeMu  sync.Mutex
	sendChan chan WriteEvent

	readTimeout  *time.Duration
	writeTimeout *time.Duration
	timeoutMu    sync.RWMutex
	timeout      time.Duration

	readIsWorked  atomic.Bool
	writeIsWorked atomic.Bool

	startedOnce sync.Once
	started     chan struct{}

	logger func(LogRecord)

	subsMu   sync.RWMutex
	subs     map[int]func(MessageEvent)
	subIDGen atomic.Int32
	// readPause atomic.Bool

	readLoopDone  chan struct{}
	writeLoopDone chan struct{}
}

func NewWebSocketWrapper(d *websocket.Dialer, url string) (*WebSocketWrapper, error) {
	conn, _, err := d.Dial(url, nil)
	if err != nil {
		return nil, errors.New("failed to connect to WebSocket: " + err.Error())
	}
	return &WebSocketWrapper{
		conn:          conn,
		dialer:        d,
		url:           url,
		sendChan:      make(chan WriteEvent, 128),
		started:       make(chan struct{}),
		subs:          make(map[int]func(MessageEvent)),
		readLoopDone:  make(chan struct{}),
		writeLoopDone: make(chan struct{}),
		timeoutMu:     sync.RWMutex{},
		timeout:       0 * time.Millisecond,
	}, nil
}

func (w *WebSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	w.logger = f
}

func (w *WebSocketWrapper) SetPingHandler(f func(string) error) {
	w.conn.SetPingHandler(f)
}

func (w *WebSocketWrapper) GetReadTimeout() time.Duration {
	if w.readTimeout == nil {
		return 0
	}
	return *w.readTimeout
}

func (w *WebSocketWrapper) SetReadTimeout(timeout time.Duration) {
	if w.readTimeout == nil && timeout != 0 {
		w.readTimeout = new(time.Duration)
	}
	*w.readTimeout = timeout
}

func (w *WebSocketWrapper) GetWriteTimeout() time.Duration {
	if w.writeTimeout == nil && w.readTimeout == nil {
		return 0
	}
	return *w.writeTimeout
}

func (w *WebSocketWrapper) SetWriteTimeout(timeout time.Duration) {
	if w.writeTimeout == nil {
		w.writeTimeout = new(time.Duration)
	}
	*w.writeTimeout = timeout
}

func (w *WebSocketWrapper) GetTimeout() time.Duration {
	w.timeoutMu.RLock()
	defer w.timeoutMu.RUnlock()
	return w.timeout
}

func (w *WebSocketWrapper) SetTimeout(timeout time.Duration) {
	w.timeoutMu.Lock()
	defer w.timeoutMu.Unlock()
	w.timeout = timeout
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

func (w *WebSocketWrapper) UnsubscribeAll() {
	w.subsMu.Lock()
	defer w.subsMu.Unlock()
	w.subs = make(map[int]func(MessageEvent))
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
	// Якщо цикли вже працюють — нічого не робимо
	if w.readIsWorked.Load() || w.writeIsWorked.Load() {
		return
	}
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

func isSocketClosedByServerError(err error) bool {
	if websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.ClosePolicyViolation,
		websocket.CloseAbnormalClosure) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, io.EOF) {
		return true
	}
	return false
}

func (w *WebSocketWrapper) readLoop(ctx context.Context) {
	w.readIsWorked.Store(true)
	defer func() {
		w.readIsWorked.Store(false)
		close(w.readLoopDone)
	}()

	for {
		select {
		case <-ctx.Done():
			// Вихід при скасуванні контексту
			return
		default:
		}

		w.readMu.Lock()
		if w.GetReadTimeout() != 0 {
			w.conn.SetReadDeadline(time.Now().Add(w.GetReadTimeout()))
		} else {
			// ❗️ Зняти deadline, щоб ReadMessage блокувався
			w.conn.SetReadDeadline(time.Time{})
		}
		logrus.Debugf("Timeout: %v", w.GetReadTimeout())
		typ, msg, err := w.conn.ReadMessage()
		w.readMu.Unlock()

		select {
		case <-ctx.Done():
			return
		default:
		}

		if w.logger != nil {
			w.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
		}

		if err != nil {
			// Обробка очікуваних помилок
			if isSocketClosedByServerError(err) {
				w.emit(MessageEvent{Kind: KindError, Error: err})
				return
			}

			// Неочікувані помилки — теж вихід
			w.emit(MessageEvent{Kind: KindError, Error: err})
			return
		}

		// Визначаємо тип повідомлення
		kind := KindControl
		if typ == websocket.TextMessage || typ == websocket.BinaryMessage {
			kind = KindData
		}

		// Еміт події
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

func (w *WebSocketWrapper) Resume() {
	w.readLoopDone = make(chan struct{}, 1)
	w.writeLoopDone = make(chan struct{}, 1)
	w.started = make(chan struct{})
	w.startedOnce = sync.Once{}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.conn.SetReadDeadline(time.Time{})
	go w.readLoop(w.ctx)
	go w.writeLoop(w.ctx)
	w.startedOnce.Do(func() { close(w.started) })
}

func (w *WebSocketWrapper) WaitAllLoops(timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	readDone := false
	writeDone := false

	for !(readDone && writeDone) {
		select {
		case <-w.readLoopDone:
			readDone = true
		case <-w.writeLoopDone:
			writeDone = true
		case <-timer.C:
			return false
		}
	}
	return true
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

func (w *WebSocketWrapper) Halt() bool {
	f := func(timeOut time.Duration) bool {
		if timeOut != 0 {
			_ = w.conn.SetReadDeadline(time.Now().Add(timeOut / 2))
		} else {
			// Stop the read loop
			if w.cancel != nil {
				w.cancel()
			}
		}
		ok := w.WaitAllLoops(timeOut)
		w.conn.SetReadDeadline(time.Time{})
		return ok
	}
	if !f(0) {
		return f(w.GetTimeout())
	} else {
		return true
	}
}

func (w *WebSocketWrapper) Close() error {
	ok := w.Halt()
	if !ok {
		return errors.New("timeout waiting for loops to finish")
	}

	// 🔽 Надішли CloseMessage перед закриттям TCP-з'єднання
	_ = w.conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closing"))

	// Дай серверу час відповісти
	time.Sleep(100 * time.Millisecond)

	return w.conn.Close()
}
