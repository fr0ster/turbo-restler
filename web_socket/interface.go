package web_socket

// WebSocketInterface defines the public interface for a WebSocket wrapper
// that provides event-based message handling, control frame handlers,
// and safe shutdown capabilities.
type WebSocketInterface interface {
	// Open starts the read and write goroutines for handling messages.
	Open()

	// Close shuts down the WebSocket connection and terminates all routines.
	Close()

	// Send queues a message to be sent over the WebSocket connection.
	// Returns an error if the queue is full or the connection is closed.
	Send(msg []byte) error

	// Subscribe registers a new message event handler.
	// Returns a unique ID that can be used to unsubscribe later.
	Subscribe(f func(MessageEvent)) int

	// Unsubscribe removes a previously registered event handler by ID.
	Unsubscribe(id int)

	// Done returns a read-only channel that is closed when the connection is closed.
	Done() <-chan struct{}

	// SetPingHandler registers a callback to be invoked when a Ping frame is received.
	SetPingHandler(f func(string, ControlWriter) error)

	// SetPongHandler registers a callback to be invoked when a Pong frame is received.
	SetPongHandler(f func(string, ControlWriter) error)

	// SetCloseHandler registers a callback to be invoked when a Close frame is received.
	SetCloseHandler(f func(int, string, ControlWriter) error)

	// GetReader returns the underlying Reader interface for receiving messages.
	// It can be used for low-level operations if needed.
	GetReader() WebApiReader
	// GetWriter returns the underlying Writer interface for sending messages.
	// It can be used for low-level operations if needed.
	GetWriter() WebApiWriter

	// SetMessageLogger allows injecting a logger function for received messages.
	// The function will be called for every received MessageEvent.
	SetMessageLogger(f func(LogRecord) error)
}
