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
	KindFatalError
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

// --- Socket-level interfaces ---
type WebApiControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

type WebApiReader interface {
	ReadMessage() (int, []byte, error)
}

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type WebSocketClientInterface interface {
	WebSocketCommonInterface
	Reconnect() error
}

type WebSocketServerInterface interface {
	WebSocketCommonInterface
}

type WebSocketCommonInterface interface {
	Open()
	Halt() bool
	Close() error
	Send(writeEvent WriteEvent) error
	Subscribe(f func(MessageEvent)) int
	Unsubscribe(id int)
	UnsubscribeAll()
	SetMessageLogger(f func(LogRecord))
	SetPingHandler(f func(string) error)
	SetPongHandler(f func(string) error)
	SetReadTimeout(time.Duration)
	SetWriteTimeout(time.Duration)
	SetTimeout(time.Duration)
	GetControl() WebApiControlWriter
	GetReader() WebApiReader
	GetWriter() WebApiWriter
	Started() <-chan struct{}
	IsStarted() bool
	WaitStarted() bool
	Stopped() <-chan struct{}
	IsStopped() bool
	WaitStopped() bool
	Resume()
	SetStartedHandler(f func())
	SetStoppedHandler(f func())
	SetConnectedHandler(f func())
	SetDisconnectHandler(f func())
}

type WriteCallback func(error)

type WriteEvent struct {
	Body     []byte
	Callback WriteCallback
	ErrChan  chan error
}

type webSocketWrapper struct {
	conn     *websocket.Conn
	dialer   *websocket.Dialer
	url      string
	isServer atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc

	readMu   sync.Mutex
	writeMu  sync.Mutex
	sendChan chan WriteEvent

	timeoutMu sync.RWMutex
	timeout   time.Duration

	readIsWorked    atomic.Bool
	writeIsWorked   atomic.Bool
	loopsAreRunning atomic.Bool

	started chan struct{}
	stopped chan struct{}

	logger func(LogRecord)

	subsMu   sync.RWMutex
	subs     map[int]func(MessageEvent)
	subIDGen atomic.Int32

	onStarted    func()
	onStopped    func()
	onConnected  func()
	onDisconnect func()
}

func NewWebSocketWrapper(d *websocket.Dialer, url string) (WebSocketClientInterface, error) {
	conn, _, err := d.Dial(url, nil)
	if err != nil {
		return nil, errors.New("failed to connect to WebSocket: " + err.Error())
	}
	return &webSocketWrapper{
		conn:      conn,
		dialer:    d,
		url:       url,
		sendChan:  make(chan WriteEvent, 128),
		started:   make(chan struct{}, 1),
		stopped:   make(chan struct{}, 1),
		subs:      make(map[int]func(MessageEvent)),
		timeoutMu: sync.RWMutex{},
		timeout:   1000 * time.Millisecond,
	}, nil
}

func WrapServerConn(conn *websocket.Conn) WebSocketServerInterface {
	is_server := &atomic.Bool{}
	is_server.Store(true)
	return &webSocketWrapper{
		conn:      conn,
		isServer:  *is_server,
		sendChan:  make(chan WriteEvent, 128),
		started:   make(chan struct{}, 1),
		stopped:   make(chan struct{}, 1),
		subs:      make(map[int]func(MessageEvent)),
		timeoutMu: sync.RWMutex{},
		timeout:   1000 * time.Millisecond,
	}
}

func (w *webSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	w.logger = f
}

func (w *webSocketWrapper) SetPingHandler(f func(string) error) {
	w.conn.SetPingHandler(f)
}

func (w *webSocketWrapper) SetPongHandler(f func(string) error) {
	w.conn.SetPongHandler(f)
}

func (w *webSocketWrapper) SetReadTimeout(timeout time.Duration) {
	if timeout != 0 {
		w.conn.SetReadDeadline(time.Now().Add(timeout))
	} else {
		// Зняти deadline, щоб ReadMessage блокувався
		w.conn.SetReadDeadline(time.Time{})
	}
}

func (w *webSocketWrapper) SetWriteTimeout(timeout time.Duration) {
	if timeout != 0 {
		w.conn.SetWriteDeadline(time.Now().Add(timeout))
	} else {
		// Зняти deadline, щоб WriteMessage блокувався
		w.conn.SetWriteDeadline(time.Time{})
	}
}

func (w *webSocketWrapper) getTimeout() time.Duration {
	w.timeoutMu.RLock()
	defer w.timeoutMu.RUnlock()
	return w.timeout
}

func (w *webSocketWrapper) SetTimeout(timeout time.Duration) {
	w.timeoutMu.Lock()
	defer w.timeoutMu.Unlock()
	w.timeout = timeout
}

func (w *webSocketWrapper) Subscribe(f func(MessageEvent)) int {
	id := int(w.subIDGen.Add(1))
	w.subsMu.Lock()
	w.subs[id] = f
	w.subsMu.Unlock()
	return id
}

func (w *webSocketWrapper) Unsubscribe(id int) {
	w.subsMu.Lock()
	delete(w.subs, id)
	w.subsMu.Unlock()
}

func (w *webSocketWrapper) UnsubscribeAll() {
	w.subsMu.Lock()
	defer w.subsMu.Unlock()
	w.subs = make(map[int]func(MessageEvent))
}

func (w *webSocketWrapper) emit(evt MessageEvent) {
	w.subsMu.RLock()
	defer w.subsMu.RUnlock()
	for _, handler := range w.subs {
		handler(evt)
	}
}

func (w *webSocketWrapper) Started() <-chan struct{} {
	return w.started
}

func (w *webSocketWrapper) IsStarted() bool {
	return w.loopsAreRunning.Load()
}

func (w *webSocketWrapper) Stopped() <-chan struct{} {
	return w.stopped
}

func (w *webSocketWrapper) IsStopped() bool {
	return !w.loopsAreRunning.Load()
}

func (w *webSocketWrapper) Open() {
	// Якщо цикли вже працюють — нічого не робимо
	if w.loopsAreRunning.Load() {
		return
	}
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.loopsAreRunning.Store(false)
	w.readIsWorked.Store(false)
	w.writeIsWorked.Store(false)
	w.started = make(chan struct{}, 1)
	w.stopped = make(chan struct{}, 1)
	go w.readLoop(w.ctx)
	go w.writeLoop(w.ctx)
}

func (w *webSocketWrapper) Send(evt WriteEvent) error {
	select {
	case w.sendChan <- evt:
		return nil
	case <-w.ctx.Done():
		return errors.New("connection is closed")
	}
}

func isFatalError(err error) bool {
	// Класифікація таймаутів
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Класифікація стандартних фатальних WebSocket/мережевих помилок
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

func (w *webSocketWrapper) checkStarted() {
	if !w.loopsAreRunning.Load() {
		if w.readIsWorked.Load() && w.writeIsWorked.Load() {
			w.loopsAreRunning.Store(true)
			select {
			case w.started <- struct{}{}:
			default:
			}
		}
	}
}

func (w *webSocketWrapper) checkStopped() {
	if w.loopsAreRunning.Load() {
		if !w.writeIsWorked.Load() && w.loopsAreRunning.Load() {
			w.loopsAreRunning.Store(false)
			select {
			case w.stopped <- struct{}{}:
			default:
			}
		}
	}
}

func (w *webSocketWrapper) readLoop(ctx context.Context) {
	defer func() {
		if w.isServer.Load() {
			w.loopsAreRunning.Store(false)
			select {
			case w.stopped <- struct{}{}:
			default:
			}
		} else {
			if w.readIsWorked.Load() {
				w.readIsWorked.Store(false)
			}
			w.checkStopped()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			// Вихід при скасуванні контексту
			return
		default:
		}

		if !w.readIsWorked.Load() {
			w.readIsWorked.Store(true)
		}
		w.checkStarted()

		w.readMu.Lock()
		// w.SetReadTimeout(w.getTimeout() / 2)
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
			if w.onDisconnect != nil {
				w.onDisconnect()
			}

			// Обробка очікуваних помилок
			if isFatalError(err) {
				w.emit(MessageEvent{Kind: KindFatalError, Error: err})
				w.conn.Close()
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

func (w *webSocketWrapper) writeLoop(ctx context.Context) {
	defer func() {
		if w.writeIsWorked.Load() {
			w.writeIsWorked.Store(false)
		}
		w.checkStopped()
	}()

	for {
		if !w.writeIsWorked.Load() {
			w.writeIsWorked.Store(true)
		}
		w.checkStarted()

		select {
		case <-ctx.Done():
			return
		case evt := <-w.sendChan:

			w.writeMu.Lock()
			// w.SetReadTimeout(w.getTimeout() / 2)
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

func (w *webSocketWrapper) Resume() {
	w.WaitStopped()
	w.loopsAreRunning.Store(false)
	w.readIsWorked.Store(false)
	w.writeIsWorked.Store(false)
	w.started = make(chan struct{}, 1)
	w.stopped = make(chan struct{}, 1)
	w.SetReadTimeout(0)
	w.SetWriteTimeout(0)
	w.ctx, w.cancel = context.WithCancel(context.Background())
	go w.readLoop(w.ctx)
	go w.writeLoop(w.ctx)
}

func (w *webSocketWrapper) WaitStarted() bool {
	if w.IsStarted() {
		return true
	}
	timer := time.NewTimer(w.getTimeout())
	defer timer.Stop()

	done := false

	for !done {
		select {
		case <-w.started:
			done = true
		case <-timer.C:
			return false
		}
	}
	return true
}

func (w *webSocketWrapper) WaitStopped() bool {
	if w.IsStopped() {
		return true
	}
	timer := time.NewTimer(w.getTimeout())
	defer timer.Stop()

	done := false

	for !done {
		select {
		case <-w.stopped:
			done = true
		case <-timer.C:
			return false
		}
	}
	return true
}

func (w *webSocketWrapper) GetControl() WebApiControlWriter {
	return w.conn
}

func (w *webSocketWrapper) GetReader() WebApiReader {
	return w.conn
}

func (w *webSocketWrapper) GetWriter() WebApiWriter {
	return w.conn
}

func (w *webSocketWrapper) Halt() bool {
	if !w.readIsWorked.Load() && !w.writeIsWorked.Load() {
		return true
	}
	w.started = make(chan struct{}, 1)
	w.stopped = make(chan struct{}, 1)
	f := func(timeOut time.Duration) bool {
		if timeOut != 0 {
			// w.SetReadTimeout(timeOut / 2)
			// w.SetWriteTimeout(timeOut / 2)
		} else {
			// Stop the read loop
			if w.cancel != nil {
				w.cancel()
			}
		}
		ok := w.WaitStopped()
		w.SetReadTimeout(0)
		w.SetWriteTimeout(0)
		return ok
	}
	if !f(0) {
		return f(w.getTimeout())
	} else {
		return true
	}
}

func (w *webSocketWrapper) Close() error {
	ok := w.Halt()
	if !ok {
		return errors.New("timeout waiting for loops to finish")
	}

	// 🔽 Надішли CloseMessage перед закриттям TCP-з'єднання
	_ = w.conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closing"))

	// Дай серверу час відповісти
	time.Sleep(w.getTimeout())

	if w.onDisconnect != nil {
		w.onDisconnect()
	}

	return w.conn.Close()
}

func (w *webSocketWrapper) Reconnect() error {
	conn, _, err := w.dialer.Dial(w.url, nil)
	if err != nil {
		return err
	}
	old := w.conn
	w.conn = conn
	_ = old.Close()
	if w.onConnected != nil {
		w.onConnected()
	}
	return nil
}

func (w *webSocketWrapper) SetStartedHandler(f func()) {
	w.onStarted = f
}

func (w *webSocketWrapper) SetStoppedHandler(f func()) {
	w.onStopped = f
}

func (w *webSocketWrapper) SetConnectedHandler(f func()) {
	w.onConnected = f
}

func (w *webSocketWrapper) SetDisconnectHandler(f func()) {
	w.onDisconnect = f
}
