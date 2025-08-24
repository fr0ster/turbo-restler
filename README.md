# 🔸 Turbo-Restler for Go

A comprehensive WebSocket and REST API library built around `gorilla/websocket`, featuring:

- 🧵 Concurrent-safe read/write
- ⏸ Pause/Resume capability
- 📩 Event-based message dispatching
- 🧪 Built-in logger with structured levels
- 🛁 Control frame handlers (Ping/Pong/Close)
- 🚀 Clean shutdown and loop lifecycle signaling
- 📊 **NEW**: WebSocket metrics and monitoring
- 🛡️ **NEW**: Circuit Breaker pattern for REST API
- 🔄 **NEW**: Automatic reconnection with exponential backoff
- 🧪 **NEW**: Quiet test mode for better debugging

---

## 📦 Features

| Feature         | Description |
|----------------|-------------|
| 🔐 Mutex locking | Ensures no concurrent access to `conn.ReadMessage` / `conn.WriteMessage` |
| 🧵 Loop channels | `readLoopDone`, `writeLoopDone`, `doneChan` for safe lifecycle management |
| ⏸ Pause/resume | `PauseLoops()` / `ResumeLoops()` block access for direct socket operations |
| 📨 Send queue | Buffered channel `sendQueue` decouples producers and the write loop |
| 📋 Subscribers | Multiple handlers via `Subscribe(f func(MessageEvent))` |
| 🦪 Control handlers | Custom `SetPingHandler`, `SetPongHandler`, `SetCloseHandler` |
| 📄 Logging | Custom `SetMessageLogger(func(LogRecord))` to trace all operations |

---

## 🥪 Example

```go
conn, _, _ := websocket.DefaultDialer.Dial("ws://example.com/ws", nil)
ws := NewWebSocketWrapper(conn)

ws.SetMessageLogger(func(log LogRecord) {
    fmt.Printf("[%s] %s\n", log.Op, string(log.Body))
})

ws.Subscribe(func(evt MessageEvent) {
    if evt.Kind == KindData {
        fmt.Println("Received:", string(evt.Body))
    }
})

ws.Open()

_ = ws.Send(WriteEvent{Body: []byte("hello")})
<-ws.Done()
```

---

## 🧐 Why Use This?

Most `gorilla/websocket` examples rely on simplistic single-threaded goroutines. In real-world systems, especially event-driven ones (e.g. Binance/Kraken clients), you need:

- Controlled shutdown
- Protected concurrent access to the socket
- Non-blocking message dispatching
- Lifecycle observability and logging

This wrapper gives you all that **without race conditions**.

---

## 📔 Structs

```go
type MessageEvent struct {
    Kind  MessageKind // KindData, KindError, KindControl
    Body  []byte
    Error error
}

type WriteEvent struct {
    Body  []byte
    Await WriteCallback // optional
    Done  SendResult     // optional channel-based result
}
```

---

## 🗰 Clean Shutdown

```go
ws.Close() // graceful
ws.Halt()  // forced

if ws.WaitAllLoops(2 * time.Second) {
    fmt.Println("All done.")
}
```

---

## 🆕 New Features (v0.14.25+)

### 📊 WebSocket Metrics
```go
ws := NewWebSocketWrapperWithConfig(WebSocketConfig{
    URL:           "wss://example.com/ws",
    EnableMetrics: true,
})

// Get real-time metrics
metrics := ws.GetMetrics()
fmt.Printf("Messages sent: %d, received: %d\n", 
    metrics.MessagesSent, metrics.MessagesReceived)
```

### 🛡️ Circuit Breaker for REST API
```go
config := &RestAPIConfig{
    Timeout:    5 * time.Second,
    MaxRetries: 3,
    CircuitBreaker: &CircuitBreakerConfig{
        FailureThreshold: 5,
        RecoveryTimeout:  30 * time.Second,
    },
}

response, err := CallRestAPIWithConfig(req, config)
```

### 🌐 Custom http.Client (timeouts, proxy)
Proxy support for REST is available only when you pass a custom `*http.Client` (with your Transport/Proxy). The default REST client does not configure a proxy. When provided, Restler will use your client as-is and sync its `Timeout` back into `RestAPIConfig`:

```go
proxyURL, _ := url.Parse("http://localhost:8080")
tr := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
cli := &http.Client{Timeout: 2 * time.Second, Transport: tr}

cfg := &RestAPIConfig{HTTPClient: cli}
resp, err := CallRestAPIWithConfig(req, cfg)
```

### � Retry and Error Semantics (v0.14.26)
- Retries: request is attempted `MaxRetries + 1` times; delay grows linearly by `RetryDelay * (attempt+1)`.
- Network/transport errors: returns `err` and `response == nil`.
- HTTP errors (status >= 400):
    - Returns `err` describing HTTP status.
    - `response` contains parsed JSON body if available; otherwise `message` field contains raw body.
    - For 5xx and if retries remain, it will retry before returning.

### 🧰 Defaults and Backward Compatibility
- `CallRestAPI(req)` uses sane defaults: `Timeout=30s`, `MaxRetries=3`, `RetryDelay=1s`, no Circuit Breaker.
- Passing `nil` config to `CallRestAPIWithConfig` applies the same defaults.
- When `HTTPClient` is provided in config, it is used as-is and its `Timeout` is synced back to `RestAPIConfig.Timeout`.

### 🧲 Circuit Breaker States (quick note)
- Closed: all requests pass; failures are counted.
- Open: requests are blocked until `RecoveryTimeout` elapses.
- Half-Open: allows up to `HalfOpenLimit` trial requests; on success → Closed, on failure → Open.

### 🌐 REST proxy usage example (via injected client)
```go
proxyURL, _ := url.Parse("http://127.0.0.1:8080")
transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
client := &http.Client{Timeout: 2 * time.Second, Transport: transport}

cfg := &RestAPIConfig{HTTPClient: client}
req, _ := http.NewRequest("GET", "https://httpbin.org/get", nil)

resp, err := CallRestAPIWithConfig(req, cfg)
if err != nil {
    // handle error
}
_ = resp // use response JSON
```

### 🌐 WebSocket proxy & dialer usage (v0.15.0)
Pass a preconfigured `*websocket.Dialer` — the wrapper uses it as-is for the handshake. If `Dialer` is nil, we clone `websocket.DefaultDialer`.

Imported from Dialer at connect time:
- Proxy, TLSClientConfig, HandshakeTimeout, EnableCompression
- ReadBufferSize/WriteBufferSize (used to derive wrapper BufferSize when `BufferSize==0`)

Wrapper-config controls (preferred over Dialer defaults):
- ReadTimeout, WriteTimeout, PingInterval, PongWait, MaxMessageSize, BufferSize

```go
dialer := &websocket.Dialer{
    Proxy:           http.ProxyURL(proxyURL),
    HandshakeTimeout: 10 * time.Second,
    EnableCompression: true,
}
ws, err := NewWebSocketWrapper(dialer, "wss://example.com/ws")
```

Notes:
- On connect we always set connection-level handlers: Pong updates read deadline (PongWait), Ping replies with Pong using write deadline, Close records the last error. You can override them later via `SetPingHandler` / `SetPongHandler`.
- `RequestHeader` in `WebSocketConfig` (when using `NewWebSocketWrapperWithConfig`) is passed to Dial.
- `Reconnect()` reuses the same Dialer instance and URL and the same `RequestHeader` (your Proxy/TLS and headers stay in effect). After reconnect we also reapply wrapper-level controls: `ReadTimeout`, `WriteTimeout`, `PongWait`-based read-deadline bump, `MaxMessageSize` (read limit), and `EnableCompression`.

Migration (from < v0.15.0):
- If you relied on wrapper to tweak Dialer fields, now set them on the Dialer before passing it, or provide explicit values in `WebSocketConfig`.
- If you need a specific BufferSize regardless of Dialer buffers, set `BufferSize` in config.

### 🛡️ Circuit Breaker recovery example
```go
cfg := &RestAPIConfig{
    Timeout:    1 * time.Second,
    MaxRetries: 0,
    CircuitBreaker: &CircuitBreakerConfig{
        FailureThreshold: 3,
        RecoveryTimeout:  2 * time.Second,
        HalfOpenLimit:    1,
    },
}

// Assume reqFailing → server returns 500, reqSuccess → server returns 200
for i := 0; i < 3; i++ {
    _, err := CallRestAPIWithConfig(reqFailing, cfg) // increments failures
    _ = err // expect HTTP 500 error
}

// Breaker now Open → blocks requests
_, err := CallRestAPIWithConfig(reqFailing, cfg)
// err contains: "circuit breaker is open"

// Wait for recovery window
time.Sleep(3 * time.Second)

// First call after timeout is allowed in Half-Open
_, err = CallRestAPIWithConfig(reqSuccess, cfg)
// success → breaker transitions to Closed
```

### �🔄 Automatic Reconnection
```go
config := WebSocketConfig{
    ReconnectConfig: &ReconnectConfig{
        MaxAttempts:        5,
        InitialDelay:       1 * time.Second,
        MaxDelay:           30 * time.Second,
        BackoffMultiplier:  2.0,
        EnableAutoReconnect: true,
    },
}
```

### 🧪 Quiet Testing
```bash
# Run tests without WebSocket noise
make test-quiet

# Or use short command
make tq
```

## 🛠️ Development

```bash
# Build everything
make all

# Run tests
make test

# Run quiet tests
make test-quiet

# Build examples
make examples

# Clean build artifacts
make clean
```

## 📚 Documentation

- **CHANGELOG.md**: Detailed change history
- **RELEASE.md**: Release notes and migration guide
- **examples/**: Working code samples
- **Makefile**: Build automation commands

## 📜 License

MIT

