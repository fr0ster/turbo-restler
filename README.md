# 🔸 WebSocketWrapper for Go

A robust and thread-safe WebSocket wrapper built around `gorilla/websocket`, featuring:

- 🧵 Concurrent-safe read/write
- ⏸ Pause/Resume capability
- 📩 Event-based message dispatching
- 🧪 Built-in logger
- 🛁 Control frame handlers (Ping/Pong/Close)
- 🚀 Clean shutdown and loop lifecycle signaling

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

## 📜 License

MIT

