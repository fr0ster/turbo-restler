package web_socket

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	strategy "github.com/fr0ster/turbo-restler/web_socket/strategy"
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
	IsReadStarted() bool
	IsWriteStarted() bool
	IsStarted() bool
	WaitStarted() bool
	IsStopped() bool
	WaitStopped() bool
	Resume()
	SetStartedHandler(f func())
	SetStoppedHandler(f func())
	SetConnectedHandler(f func())
	SetDisconnectHandler(f func())
	SetRemoteCloseHandler(fn func(code int, reason string) error)
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

	strategy      strategy.WrapperStrategy
	onRemoteClose func(code int, reason string) error
}

func NewWebSocketWrapper(d *websocket.Dialer, url string) (WebSocketClientInterface, error) {
	conn, _, err := d.Dial(url, nil)
	if err != nil {
		return nil, errors.New("failed to connect to WebSocket: " + err.Error())
	}
	strategy := strategy.NewDefaultStrategy()
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
		strategy:  strategy,
	}, nil
}

func WrapServerConn(conn *websocket.Conn) WebSocketServerInterface {
	strategy := strategy.NewDefaultStrategy()
	wrapper := &webSocketWrapper{
		conn:      conn,
		sendChan:  make(chan WriteEvent, 128),
		started:   make(chan struct{}, 1),
		stopped:   make(chan struct{}, 1),
		subs:      make(map[int]func(MessageEvent)),
		timeoutMu: sync.RWMutex{},
		timeout:   1000 * time.Millisecond,
		strategy:  strategy,
	}
	wrapper.isServer.Store(true)
	return wrapper
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

func (w *webSocketWrapper) IsReadStarted() bool {
	return w.readIsWorked.Load()
}

func (w *webSocketWrapper) IsWriteStarted() bool {
	return w.writeIsWorked.Load()
}

func (w *webSocketWrapper) IsStarted() bool {
	return w.loopsAreRunning.Load()
}

// func (w *webSocketWrapper) Stopped() <-chan struct{} {
// 	return w.stopped
// }

func (w *webSocketWrapper) IsStopped() bool {
	return !w.loopsAreRunning.Load()
}

func (w *webSocketWrapper) Open() {
	// Якщо цикли вже працюють — нічого не робимо
	if w.loopsAreRunning.Load() {
		return
	}
	w.conn.SetCloseHandler(func(code int, text string) error {
		w.strategy.OnRemoteClose(code, text)
		if w.onRemoteClose != nil {
			w.onRemoteClose(code, text)
		}
		return nil
	})
	w.ctx, w.cancel = context.WithCancel(context.Background())
	w.loopsAreRunning.Store(false)
	w.readIsWorked.Store(false)
	w.writeIsWorked.Store(false)
	w.started = make(chan struct{}, 1)
	w.stopped = make(chan struct{}, 1)
	go w.readLoop()
	go w.writeLoop()
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

// func (w *webSocketWrapper) checkStarted() {
// 	if !w.loopsAreRunning.Load() {
// 		if w.readIsWorked.Load() && w.writeIsWorked.Load() {
// 			w.loopsAreRunning.Store(true)
// 			select {
// 			case w.started <- struct{}{}:
// 			default:
// 			}
// 		}
// 	}
// }

// func (w *webSocketWrapper) checkStopped() {
// 	if w.loopsAreRunning.Load() {
// 		if !w.writeIsWorked.Load() && w.loopsAreRunning.Load() {
// 			w.loopsAreRunning.Store(false)
// 			select {
// 			case w.stopped <- struct{}{}:
// 			default:
// 			}
// 		}
// 	}
// }

func (w *webSocketWrapper) readLoop() {
	defer func() {
		w.readIsWorked.Store(false)
		strategy.MarkCycleStopped(
			&w.readIsWorked,
			&w.writeIsWorked,
			&w.loopsAreRunning,
			w.strategy,
			w.stopped,
		)
	}()

	for {
		strategy.MarkCycleStarted(
			"read",
			&w.readIsWorked,
			&w.writeIsWorked,
			&w.loopsAreRunning,
			w.strategy,
			w.started)

		select {
		case <-w.ctx.Done():
			return
		default:
		}

		if w.strategy.ShouldExitReadLoop() {
			return
		}
		w.readMu.Lock()
		w.SetReadTimeout(w.getTimeout() / 2)
		typ, msg, err := w.conn.ReadMessage()
		w.readMu.Unlock()

		select {
		case <-w.ctx.Done():
			return
		default:
		}

		if w.logger != nil {
			w.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
		}

		if err != nil {
			w.emit(MessageEvent{Kind: KindError, Error: err})
			if w.strategy.OnReadError(err) {
				if w.onDisconnect != nil {
					w.onDisconnect()
				}
				w.strategy.OnCloseFrame()
				w.conn.Close()
				return
			}

			continue
		}

		// Визначаємо тип повідомлення
		kind := KindControl
		if typ == websocket.TextMessage || typ == websocket.BinaryMessage {
			kind = KindData
		}

		// Еміт події
		w.emit(MessageEvent{Kind: kind, Body: msg})

		// 💡 коротка пауза або continue — запобігання spin loop
		time.Sleep(100 * time.Millisecond)
	}
}

func (w *webSocketWrapper) writeLoop() {
	defer func() {
		w.writeIsWorked.Store(false)
		w.strategy.OnBeforeWriteLoopExit()
		strategy.MarkCycleStopped(
			&w.readIsWorked, &w.writeIsWorked,
			&w.loopsAreRunning, w.strategy, w.stopped,
		)
	}()

	for {
		strategy.MarkCycleStarted(
			"write",
			&w.readIsWorked, &w.writeIsWorked,
			&w.loopsAreRunning, w.strategy, w.started,
		)

		select {
		case <-w.ctx.Done():
			return

		case evt := <-w.sendChan:
			// 💡 Щойно щось прийшло — одразу перевірка shutdown
			if w.strategy.ShouldExitWriteLoop(len(w.sendChan) == 0, w.strategy.IsShutdownRequested()) {
				return
			}

			w.writeMu.Lock()
			w.SetReadTimeout(w.getTimeout() / 2)
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

		default:
			// 💡 Порожня черга — перевірити, чи пора виходити
			if w.strategy.ShouldExitWriteLoop(true, w.strategy.IsShutdownRequested()) {
				return
			}

			// 💡 коротка пауза або continue — запобігання spin loop
			time.Sleep(100 * time.Millisecond)
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
	go w.readLoop()
	go w.writeLoop()
}

func (w *webSocketWrapper) WaitStarted() bool {
	if w.IsStarted() {
		return true
	}
	return w.strategy.WaitForStart(w.started, w.getTimeout())
}

func (w *webSocketWrapper) WaitStopped() bool {
	if w.IsStopped() {
		return true
	}
	return w.strategy.WaitForStop(w.stopped, w.getTimeout())
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

	// Сигналізуємо стратегії про необхідність завершення
	w.strategy.RequestShutdown()

	// Скидаємо канали, щоб мати актуальне очікування
	w.started = make(chan struct{}, 1)
	w.stopped = make(chan struct{}, 1)

	// Пробуємо мʼяко: без скасування контексту, без таймауту

	if !w.strategy.WaitForStop(w.stopped, w.getTimeout()) {
		// w.SetReadTimeout(w.getTimeout() / 5)
		// w.SetWriteTimeout(w.getTimeout() / 5)
		ok := w.strategy.WaitForStop(w.stopped, w.getTimeout())
		// Примусове завершення: скасовуємо контекст
		if !ok {
			if w.cancel != nil {
				w.cancel()
			}
			ok = w.strategy.WaitForStop(w.stopped, w.getTimeout())
		}
		// Відновлення поведінки по замовчуванню
		w.SetReadTimeout(0)
		w.SetWriteTimeout(0)
		return ok
	}

	// Відновлення I/O
	w.SetReadTimeout(0)
	w.SetWriteTimeout(0)
	return true
}

func (w *webSocketWrapper) Close() error {
	// // 💡 Ініціюємо зупинку, чекаємо з тайм-аутом
	// ok := w.Halt()
	// if !ok {
	// 	return errors.New("timeout waiting for loops to finish")
	// }

	// ✅ Пишемо CloseMessage (вихід ініційований клієнтом)
	_ = w.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client closing"),
	)

	// 🕒 Даємо серверу шанс відповісти (не читаємо відповідь, просто чекаємо)
	time.Sleep(w.getTimeout())

	// 🔔 Сповіщаємо про розрив
	if w.onDisconnect != nil {
		w.onDisconnect()
	}

	// 🔒 Фінальне закриття сокета
	return w.conn.Close()
}

func (w *webSocketWrapper) Reconnect() error {
	if w.readIsWorked.Load() || w.writeIsWorked.Load() {
		return errors.New("cannot reconnect: read/write loops are still running")
	}

	if !w.strategy.ShouldReconnect() {
		return errors.New("reconnect denied by strategy")
	}

	if err := w.strategy.ReconnectBefore(); err != nil {
		return err
	}

	conn, _, err := w.dialer.Dial(w.url, nil)
	if err != nil {
		w.strategy.HandleReconnectError(err)
		return err
	}

	old := w.conn
	w.conn = conn
	_ = old.Close()

	w.conn.SetCloseHandler(func(code int, text string) error {
		w.strategy.OnRemoteClose(code, text)
		if w.onRemoteClose != nil {
			w.onRemoteClose(code, text)
		}
		return nil
	})

	// Скидаємо флаги (цикли ще не запущені)
	w.readIsWorked.Store(false)
	w.writeIsWorked.Store(false)
	w.loopsAreRunning.Store(false)

	if err := w.strategy.ReconnectAfter(); err != nil {
		return err
	}

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

func (w *webSocketWrapper) SetRemoteCloseHandler(fn func(code int, reason string) error) {
	w.onRemoteClose = fn
}

func IgnoreExpectedErrorsLogger(base func(evt LogRecord)) func(evt LogRecord) {
	return func(evt LogRecord) {
		if evt.Err != nil {
			msg := evt.Err.Error()
			switch {
			case strings.Contains(msg, "close sent"),
				strings.Contains(msg, "use of closed network connection"),
				strings.Contains(msg, "going away"),
				strings.Contains(msg, "i/o timeout"),
				strings.Contains(msg, "EOF"):
				return // ✅ очікувана помилка — ігноруємо
			}
		}
		base(evt)
	}
}
