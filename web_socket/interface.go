package web_socket

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- Event types ---
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

type dualContextWS struct {
	conn        *websocket.Conn
	readMu      sync.Mutex
	emit        func(MessageEvent)
	logger      func(LogRecord)
	readTimeout time.Duration
	started     chan struct{}
	done        chan struct{}
	globalCtx   context.Context
	pauseCtx    context.Context
	pauseCancel context.CancelFunc
}

// --- Core interface ---
type WebSocketCoreInterface interface {
	Start(ctx context.Context)
	Halt() bool
	Resume()
	Done() <-chan struct{}
}

// --- Callback-style interface ---
type WebSocketCallbackInterface interface {
	SetMessageLogger(func(LogRecord))
}

// --- Context-style interface ---
type WebSocketContextInterface interface {
	GetPauseContext() context.Context
	GetGlobalContext() context.Context
}

// --- Channel-style interface (stubbed for now) ---
type WebSocketChannelInterface interface {
	MessageChannel() <-chan MessageEvent
	ErrorChannel() <-chan error
}

// --- Unified interface ---
type WebSocketInterface interface {
	WebSocketCoreInterface
	WebSocketCallbackInterface
	WebSocketContextInterface
	WebSocketChannelInterface
	WebApiControlWriterProvider
	WebApiReaderProvider
	WebApiWriterProvider
}

// --- Support interfaces for socket-level access ---
type WebApiControlWriter interface {
	WriteControl(messageType int, data []byte, deadline time.Time) error
}

type WebApiReader interface {
	ReadMessage() (int, []byte, error)
}

type WebApiWriter interface {
	WriteMessage(messageType int, data []byte) error
}

type WebApiControlWriterProvider interface {
	GetControl() WebApiControlWriter
}

type WebApiReaderProvider interface {
	GetReader() WebApiReader
}

type WebApiWriterProvider interface {
	GetWriter() WebApiWriter
}

func NewDualContextWS(conn *websocket.Conn, emit func(MessageEvent)) *dualContextWS {
	return &dualContextWS{
		conn:        conn,
		emit:        emit,
		started:     make(chan struct{}),
		done:        make(chan struct{}),
		readTimeout: 5 * time.Second,
	}
}

func (d *dualContextWS) Start(ctx context.Context) {
	d.globalCtx = ctx
	d.pauseCtx, d.pauseCancel = context.WithCancel(ctx)
	go d.readLoop()
	close(d.started)
}

func (d *dualContextWS) readLoop() {
	defer close(d.done)

	for {
		select {
		case <-d.globalCtx.Done():
			return
		case <-d.pauseCtx.Done():
			return
		default:
		}

		d.readMu.Lock()
		if d.readTimeout > 0 {
			d.conn.SetReadDeadline(time.Now().Add(d.readTimeout))
		}
		typ, msg, err := d.conn.ReadMessage()
		d.readMu.Unlock()

		select {
		case <-d.globalCtx.Done():
			return
		case <-d.pauseCtx.Done():
			return
		default:
		}

		if d.logger != nil {
			d.logger(LogRecord{Op: OpReceive, Body: msg, Err: err})
		}

		if err != nil {
			d.emit(MessageEvent{Kind: KindError, Error: err})
			return
		}

		kind := KindControl
		if typ == websocket.TextMessage || typ == websocket.BinaryMessage {
			kind = KindData
		}

		d.emit(MessageEvent{Kind: kind, Body: msg})
	}
}

func (d *dualContextWS) Halt() bool {
	if d.pauseCancel != nil {
		d.pauseCancel()
		return true
	}
	return false
}

func (d *dualContextWS) Resume() {
	ctx := d.globalCtx
	d.pauseCtx, d.pauseCancel = context.WithCancel(ctx)
	go d.readLoop()
}

func (d *dualContextWS) Done() <-chan struct{} {
	return d.done
}

// --- Stub methods ---
func (d *dualContextWS) SetMessageLogger(f func(LogRecord)) {
	d.logger = f
}

func (d *dualContextWS) GetPauseContext() context.Context {
	return d.pauseCtx
}

func (d *dualContextWS) GetGlobalContext() context.Context {
	return d.globalCtx
}

func (d *dualContextWS) MessageChannel() <-chan MessageEvent {
	ch := make(chan MessageEvent, 64)
	d.emit = func(evt MessageEvent) {
		select {
		case ch <- evt:
		default:
			// drop or log overflow
		}
	}
	return ch
}

func (d *dualContextWS) ErrorChannel() <-chan error {
	errCh := make(chan error, 16)
	// wrap the current emit function
	prevEmit := d.emit
	d.emit = func(evt MessageEvent) {
		if evt.Kind == KindError && evt.Error != nil {
			select {
			case errCh <- evt.Error:
			default:
				// drop or log overflow
			}
		}
		if prevEmit != nil {
			prevEmit(evt)
		}
	}
	return errCh
}

func (d *dualContextWS) GetControl() WebApiControlWriter {
	return d.conn
}

func (d *dualContextWS) GetReader() WebApiReader {
	return d.conn
}

func (d *dualContextWS) GetWriter() WebApiWriter {
	return d.conn
}
