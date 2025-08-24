package web_socket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

// Додаємо структуровані помилки
type WebSocketError struct {
	Op      LogOp
	Message string
	Err     error
	Code    int
}

func (e *WebSocketError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (code: %d) - %v", e.Op, e.Message, e.Code, e.Err)
	}
	return fmt.Sprintf("%s: %s (code: %d)", e.Op, e.Message, e.Code)
}

func (e *WebSocketError) Unwrap() error {
	return e.Err
}

// Покращений логер з рівнями
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

type LogRecord struct {
	Op      LogOp
	Level   LogLevel
	Body    []byte
	Err     error
	Time    time.Time
	Context map[string]interface{}
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

// Метрики та статистика
type WebSocketMetrics struct {
	MessagesSent     int64
	MessagesReceived int64
	BytesSent        int64
	BytesReceived    int64
	Errors           int64
	Reconnects       int64
	LastActivity     time.Time
	Uptime           time.Duration
}

type webSocketWrapper struct {
	conn          *websocket.Conn
	dialer        *websocket.Dialer
	url           string
	requestHeader http.Header
	isServer      atomic.Bool

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

	// Додаємо метрики
	metricsMu sync.RWMutex
	metrics   WebSocketMetrics
	startTime time.Time

	// Автоматичне перепідключення
	reconnectMu     sync.Mutex
	reconnectConfig *ReconnectConfig
	pingTicker      *time.Ticker
	lastPong        time.Time

	// Остання помилка для повторної доставки пізнім підписникам
	lastMu      sync.RWMutex
	lastErr     error
	lastErrKind MessageKind

	// Persisted config for consistent reconnect behavior
	cfgReadTimeout       time.Duration
	cfgWriteTimeout      time.Duration
	cfgPongWait          time.Duration
	cfgMaxMessageSize    int64
	cfgEnableCompression bool
}

// Конфігурація перепідключення
type ReconnectConfig struct {
	MaxAttempts         int
	InitialDelay        time.Duration
	MaxDelay            time.Duration
	BackoffMultiplier   float64
	EnableAutoReconnect bool
}

// Конфігурація WebSocket
type WebSocketConfig struct {
	// URL is the full WebSocket URL to dial.
	URL string
	// Dialer, when provided, is used as-is for the handshake (Proxy/TLS/HandshakeTimeout/Compression).
	// When nil, a clone of websocket.DefaultDialer is used.
	Dialer *websocket.Dialer
	// RequestHeader is passed to Dial for the initial handshake.
	RequestHeader http.Header
	// BufferSize sets the internal send buffer size. When 0, it is derived from Dialer Read/WriteBufferSize,
	// or defaults to 128 if both are zero.
	BufferSize int
	// ReadTimeout/WriteTimeout are wrapper-level socket deadlines applied after connect and used for Ping/Pong.
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	// PingInterval starts a keepalive ping loop when > 0. PongWait extends read deadline on pong.
	PingInterval time.Duration
	PongWait     time.Duration
	// MaxMessageSize applies SetReadLimit on the connection.
	MaxMessageSize int64
	// EnableCompression enables permessage-deflate on the connection. If false here but true on Dialer,
	// compression is still enabled.
	EnableCompression bool
	EnableMetrics     bool
	// ReconnectConfig describes optional auto-reconnect behavior.
	ReconnectConfig *ReconnectConfig
}

// Конструктор з конфігурацією
func NewWebSocketWrapperWithConfig(config WebSocketConfig) (WebSocketClientInterface, error) {
	// Використовуємо переданий Dialer як є; якщо не передали — клонуємо DefaultDialer
	if config.Dialer == nil {
		d := *websocket.DefaultDialer
		config.Dialer = &d
	}

	if config.BufferSize == 0 {
		// Derive buffer size from dialer if available; fallback to sane default
		rb := config.Dialer.ReadBufferSize
		wb := config.Dialer.WriteBufferSize
		if rb > 0 || wb > 0 {
			if rb >= wb {
				config.BufferSize = rb
			} else {
				config.BufferSize = wb
			}
			if config.BufferSize == 0 { // both zero
				config.BufferSize = 128
			}
		} else {
			config.BufferSize = 128
		}
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.PingInterval == 0 {
		config.PingInterval = 30 * time.Second
	}
	if config.PongWait == 0 {
		config.PongWait = 60 * time.Second
	}
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = 512 * 1024 // 512KB
	}

	// Використовуємо надані заголовки для початкового рукостискання (якщо є)
	conn, resp, err := config.Dialer.Dial(config.URL, config.RequestHeader)
	if err != nil {
		// Збагачуємо помилку деталями handhshake (HTTP статус + тіло, якщо є)
		code := 0
		msg := "failed to connect to WebSocket"
		if resp != nil {
			code = resp.StatusCode
			// Прочитаємо частину тіла, щоб не роздувати повідомлення
			var bodySnippet string
			if resp.Body != nil {
				b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
				_ = resp.Body.Close()
				if len(b) > 0 {
					bodySnippet = string(b)
				}
			}
			if bodySnippet != "" {
				msg = fmt.Sprintf("handshake failed: %s — %s", resp.Status, bodySnippet)
			} else {
				msg = fmt.Sprintf("handshake failed: %s", resp.Status)
			}
		}
		return nil, &WebSocketError{Op: OpSend, Message: msg, Err: err, Code: code}
	}

	// Налаштування з'єднання
	conn.SetReadLimit(config.MaxMessageSize)
	// Apply initial deadlines based on config
	if config.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(config.ReadTimeout))
	}
	if config.WriteTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout))
	}
	// Respect compression preference from either config or dialer (also reflect into config for consistency)
	if config.EnableCompression || config.Dialer.EnableCompression {
		config.EnableCompression = true
		conn.EnableWriteCompression(true)
	}

	// Побудова екземпляра, щоб ініціювати додаткові поля
	w := &webSocketWrapper{
		conn:                 conn,
		dialer:               config.Dialer,
		url:                  config.URL,
		requestHeader:        config.RequestHeader,
		sendChan:             make(chan WriteEvent, config.BufferSize),
		started:              make(chan struct{}, 1),
		stopped:              make(chan struct{}, 1),
		subs:                 make(map[int]func(MessageEvent)),
		timeoutMu:            sync.RWMutex{},
		timeout:              config.ReadTimeout,
		startTime:            time.Now(),
		cfgReadTimeout:       config.ReadTimeout,
		cfgWriteTimeout:      config.WriteTimeout,
		cfgPongWait:          config.PongWait,
		cfgMaxMessageSize:    config.MaxMessageSize,
		cfgEnableCompression: config.EnableCompression,
	}

	// Config persisted in wrapper fields (cfg*)

	// Зберігаємо конфіг перепідключення (на майбутнє)
	if config.ReconnectConfig != nil {
		w.reconnectConfig = config.ReconnectConfig
	}

	// Ініціюємо пінг-тикачу, якщо заданий інтервал
	if config.PingInterval > 0 {
		w.pingTicker = time.NewTicker(config.PingInterval)
	}

	// Обробник PONG: оновлює дедлайн і lastPong (override any pre-existing)
	w.conn.SetPongHandler(func(s string) error {
		w.lastPong = time.Now()
		if w.cfgPongWait > 0 {
			_ = w.conn.SetReadDeadline(time.Now().Add(w.cfgPongWait))
		}
		return nil
	})

	// Обробник PING: відправляє PONG контрол-фрейм (override any pre-existing)
	w.conn.SetPingHandler(func(s string) error {
		deadline := time.Time{}
		if w.cfgWriteTimeout > 0 {
			deadline = time.Now().Add(w.cfgWriteTimeout)
			_ = w.conn.SetWriteDeadline(deadline)
		}
		w.writeMu.Lock()
		defer w.writeMu.Unlock()
		return w.conn.WriteControl(websocket.PongMessage, []byte(s), deadline)
	})

	// Обробник CLOSE: зберігає останню подію (override any pre-existing)
	w.conn.SetCloseHandler(func(code int, text string) error {
		w.lastMu.Lock()
		w.lastErr = &WebSocketError{Op: OpReceive, Message: text, Code: code}
		w.lastErrKind = KindControl
		w.lastMu.Unlock()
		return nil // use default close behavior
	})

	return w, nil
}

// Оригінальний конструктор для зворотної сумісності
func NewWebSocketWrapper(d *websocket.Dialer, url string) (WebSocketClientInterface, error) {
	config := WebSocketConfig{
		URL:            url,
		Dialer:         d,
		BufferSize:     128,
		ReadTimeout:    1000 * time.Millisecond,
		WriteTimeout:   1000 * time.Millisecond,
		PingInterval:   30 * time.Second,
		PongWait:       60 * time.Second,
		MaxMessageSize: 512 * 1024, // 512KB
		EnableMetrics:  false,
	}
	return NewWebSocketWrapperWithConfig(config)
}

func WrapServerConn(conn *websocket.Conn) WebSocketServerInterface {
	ws := &webSocketWrapper{
		conn:      conn,
		isServer:  atomic.Bool{},
		sendChan:  make(chan WriteEvent, 128),
		started:   make(chan struct{}, 1),
		stopped:   make(chan struct{}, 1),
		subs:      make(map[int]func(MessageEvent)),
		timeoutMu: sync.RWMutex{},
		timeout:   1000 * time.Millisecond,
	}
	ws.isServer.Store(true)
	return ws
}

func (w *webSocketWrapper) SetMessageLogger(f func(LogRecord)) {
	w.logger = f
}

func (w *webSocketWrapper) SetPingHandler(f func(string) error) {
	w.conn.SetPingHandler(f)
}

func (w *webSocketWrapper) SetPongHandler(f func(string) error) {
	// Обгортаємо, щоб також оновлювати lastPong
	w.conn.SetPongHandler(func(s string) error {
		w.lastPong = time.Now()
		if f != nil {
			return f(s)
		}
		return nil
	})
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

	// Якщо вже була помилка до підписки — одразу повідомимо підписника
	w.lastMu.RLock()
	lastErr := w.lastErr
	lastKind := w.lastErrKind
	w.lastMu.RUnlock()
	if lastErr != nil && (lastKind == KindError || lastKind == KindFatalError) {
		go f(MessageEvent{Kind: lastKind, Error: lastErr})
	}
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

	// Запам'ятовуємо останню помилку
	if evt.Kind == KindError || evt.Kind == KindFatalError {
		w.lastMu.Lock()
		w.lastErr = evt.Error
		w.lastErrKind = evt.Kind
		w.lastMu.Unlock()
	}
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
	// Для клієнта — запускаємо keepalive тикачу, якщо вона є
	if !w.isServer.Load() && w.pingTicker != nil {
		go w.pingLoop(w.ctx)
	}
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
			w.logger(LogRecord{Op: OpReceive, Level: LogLevelInfo, Body: msg, Err: err, Time: time.Now()})
		}

		// Оновлюємо метрики
		w.updateMetrics(OpReceive, msg, err)

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
				w.logger(LogRecord{Op: OpSend, Level: LogLevelInfo, Body: evt.Body, Err: err, Time: time.Now()})
			}

			// Оновлюємо метрики
			w.updateMetrics(OpSend, evt.Body, err)

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

	// Зупиняємо тикачу пінгів, якщо була
	if w.pingTicker != nil {
		w.pingTicker.Stop()
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
	// Захист від конкурентних реконектів
	w.reconnectMu.Lock()
	defer w.reconnectMu.Unlock()
	conn, _, err := w.dialer.Dial(w.url, w.requestHeader)
	if err != nil {
		return err
	}
	old := w.conn
	w.conn = conn
	_ = old.Close()
	// Reapply limits, deadlines, compression and default handlers after reconnect
	if w.cfgMaxMessageSize > 0 {
		w.conn.SetReadLimit(w.cfgMaxMessageSize)
	}
	if w.cfgReadTimeout > 0 {
		_ = w.conn.SetReadDeadline(time.Now().Add(w.cfgReadTimeout))
	}
	if w.cfgWriteTimeout > 0 {
		_ = w.conn.SetWriteDeadline(time.Now().Add(w.cfgWriteTimeout))
	}
	if w.cfgEnableCompression {
		w.conn.EnableWriteCompression(true)
	}
	// Reinstall default handlers (they can be overridden later)
	w.conn.SetPongHandler(func(s string) error {
		w.lastPong = time.Now()
		if w.cfgPongWait > 0 {
			_ = w.conn.SetReadDeadline(time.Now().Add(w.cfgPongWait))
		}
		return nil
	})
	w.conn.SetPingHandler(func(s string) error {
		deadline := time.Time{}
		if w.cfgWriteTimeout > 0 {
			deadline = time.Now().Add(w.cfgWriteTimeout)
			_ = w.conn.SetWriteDeadline(deadline)
		}
		w.writeMu.Lock()
		defer w.writeMu.Unlock()
		return w.conn.WriteControl(websocket.PongMessage, []byte(s), deadline)
	})
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

// GetMetrics повертає поточні метрики WebSocket
func (w *webSocketWrapper) GetMetrics() WebSocketMetrics {
	w.metricsMu.RLock()
	defer w.metricsMu.RUnlock()

	// Оновлюємо uptime
	w.metrics.Uptime = time.Since(w.startTime)

	return w.metrics
}

// updateMetrics оновлює метрики
func (w *webSocketWrapper) updateMetrics(op LogOp, body []byte, err error) {
	w.metricsMu.Lock()
	defer w.metricsMu.Unlock()

	w.metrics.LastActivity = time.Now()

	if err != nil {
		w.metrics.Errors++
	} else {
		switch op {
		case OpSend:
			w.metrics.MessagesSent++
			w.metrics.BytesSent += int64(len(body))
		case OpReceive:
			w.metrics.MessagesReceived++
			w.metrics.BytesReceived += int64(len(body))
		}
	}
}

// Періодично відправляє Ping як keepalive (для клієнтських врапперів)
func (w *webSocketWrapper) pingLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.pingTicker.C:
			// Best-effort ping; помилки проявляться у read/write loops
			_ = w.conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(1*time.Second))
		}
	}
}
