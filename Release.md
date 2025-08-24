# Release Notes for Turbo-Restler

## v0.14.27

### Release Date: 2025-08-24

### Docs/Test meta (no code changes)

- Added missing release notes for the previous version `v0.14.26`.
- Clarified test coverage around REST retries, Circuit Breaker states, and http.Client injection.
- No API or behavior changes.

Tag: v0.14.27

## v0.14.26

### Release Date: 2025-08-24

### Test/Docs + REST Enhancements (no breaking changes)

What's new:
- REST API tests: Added comprehensive tests for CallRestAPIWithConfig
  - Circuit Breaker behavior (Closed/Open/Half-Open transitions)
  - Retry/backoff semantics and HTTP 5xx handling
  - Network error path ensures response == nil
  - Backward compatibility with CallRestAPI
  - External http.Client injection (timeout/proxy)
- Test utilities: Introduced helpers used by tests
  - StartHandlerServer, StartStaticJSONServer
  - StartForwardProxy, ProxyHits
  - NewHTTPClientWithProxy (constructs client with proxy + timeout)
- Build hygiene: Aligned test package names and helpers to avoid undefined symbols during test compilation.

Notes:
- Public API remains unchanged. Enhancements are covered by tests and do not introduce breaking changes.
- Circuit Breaker and client-injection behavior are now thoroughly validated.

Tag: v0.14.26

## v0.14.25

### Release Date: 2024-08-20

### 🎉 Major Enhancement Release

**What's New:**
- **Structured Error Handling**: New `WebSocketError` type with structured error information
- **WebSocket Metrics**: Real-time metrics for messages, bytes, errors, and reconnects
- **Circuit Breaker Pattern**: REST API circuit breaker with configurable thresholds
- **Automatic Reconnection**: Smart reconnection with exponential backoff
- **Enhanced Testing**: Quiet test mode to filter out WebSocket noise
- **Comprehensive Makefile**: Build automation and development tools
- **Examples**: Working WebSocket examples with configuration

**Breaking Changes:** None - Full backward compatibility maintained.

**Go Version:** 1.23.0+

---

## v0.14.22

### Release Date: 2025-04-15

### Feat

 - Add SetPongHandler method to WebSocketInterface and WebSocketWrapper

---

## v0.14.21

### Release Date: 2025-04-15

### Feat

- Introduced a robust `WebSocketWrapper` built on top of `gorilla/websocket`, providing full lifecycle control and concurrency safety.
- **Lifecycle Methods**:
  - `Open()` – starts the read/write loops.
  - `Halt()` – gracefully stops the loops, optionally enforcing short read/write deadlines before force-stopping.
  - `Resume()` – restarts loops after a halt, reinitializes internal state.
  - `Close()` – cleanly closes the WebSocket by sending a `CloseMessage` and waiting for the peer to respond.
  - `Reconnect()` – replaces the internal connection with a new one from the same dialer and URL.
- **Subscription System**:
  - `Subscribe(func(MessageEvent))` – add a subscriber.
  - `Unsubscribe(id int)` – remove a subscriber by ID.
  - `UnsubscribeAll()` – clear all subscriptions.
  - Internally calls `emit` to deliver messages and errors to all subscribers.
- **Asynchronous Send**:
  - `Send(WriteEvent)` supports sending messages through a buffered channel.
  - `WriteEvent` includes support for:
    - `Callback func(error)`
    - `ErrChan chan error`
- **Logging**:
  - `SetMessageLogger(func(LogRecord))` allows structured logging of send/receive events, including errors.
- **Timeout Handling**:
  - `SetReadTimeout` / `SetWriteTimeout` directly configure socket-level deadlines.
  - `SetTimeout` / `GetTimeout` manage wrapper-level timeout for stopping loops and clean shutdowns.
- **Control Interfaces**:
  - `GetReader()`, `GetWriter()`, and `GetControl()` expose WebSocket interfaces for advanced usage (e.g., raw stream reading or control frame writes).
- **Loop Coordination**:
  - `WaitAllLoops(timeout)` waits for read/write loops to stop.
  - Atomic flags and sync primitives prevent race conditions between state transitions.

---

## v0.2.27

### Release Date: 2024-08-23

### Feat
- Added the ability to create a socket for sending messages in both text and binary formats.

---

## v0.2.26

### Release Date: 2024-08-21

### Feat
- Changed parameters in `CallRestAPI` to `req *http.Request`.

---

## v0.2.25

### Release Date: 2024-08-21

### Chore
- Upgraded to `turbo-signer` v0.1.6.

---

## v0.2.24

### Release Date: 2024-08-21

### Feat
- Added an optional parameter `apiKey` to `CallRestAPI` for cases where the `apikey` parameter cannot be passed.

---

## v0.2.23

### Release Date: 2024-08-21

### Fix
- Fixed a bug in `CallRestAPI` function, corrected the definition of the full URL when there are no parameters.

---

## v0.2.22

### Release Date: 2024-08-21

### Fix
- Fixed a bug in `CallRestAPI` function, corrected the key for `apiKey`.

---

## v0.2.21

### Release Date: 2024-08-21

### Chore
- Upgraded to `turbo-signer` v0.1.5.

---

## v0.2.20

### Release Date: 2024-08-21

### Refactor
- Refactor `WebSocketWrapper` to consolidate functionality and improve code organization.
  - This commit merges the functionality of `WebApi` and `WebStream` into a single `WebSocketWrapper` struct. This consolidation improves the organization and readability of the codebase.

---

## v0.2.19

### Release Date: 2024-08-19

### Changes
- **Turbo-Restler**:
  - **WebSocketWrapper**:
    - Discontinued the use of context with timeout for managing the read loop.

---

## v0.2.18

### Release Date: 2024-08-19

### Changes
- **Turbo-Restler**:
  - **WebSocketWrapper**:
    - Removed `Start` and `Stop` methods, making the start of the read loop from the WebSocket implicit when a handler is added.
    - Made the stop of the read loop implicit when the last handler is removed.
    - Added waiting for the start and stop of the loop.
    - Added a mutex to ensure that only one loop reads from the WebSocket at a time.

---

## v0.2.17

### Release Date: 2024-08-18

### Changes
- **Turbo-Restler**:
  - **WebSocketWrapper**:
    - Added `Lock` and `Unlock` functions.

---

## v0.2.16

### Release Date: 2024-08-18

### Changes
- **Turbo-Restler**:
  - **WebSocketWrapper**:
    - Added update to the quit channel to stop the loop, allowing for the possibility of restarting the loop.

---

## v0.2.15

### Release Date: 2024-08-18

### Changes
- **Turbo-Restler**:
  - **WebSocketWrapper**:
    - Merged the functionality of `WebApi` and `WebStream` into a single `WebSocketWrapper` to streamline and simplify the codebase.

---

## v0.2.14

### Release Date: 2024-08-18

### Changes
- **Turbo-Restler**:
  - **WebApi**:
    - Removed `Call` method as it assumes that the WebSocket is used exclusively for this request and no external information can appear in the WebSocket.
  - **WebStream**:
    - Removed `Subscribe`, `ListOfSubscriptions`, and `Unsubscribe` methods as they are unnecessary at this level of abstraction and will be moved to `turbo-cambitor`.

---

## v0.2.13

### Release Date: 2024-08-18

### Changes
- **Turbo-Restler**:
  - **WebApi**:
    - Removed default logging and added the ability to set a custom error logger for handling ping messages.
    - Removed the default ping message handler setup in `New` and added the ability to set it if needed.
    - Added the ability to check the socket state (open or closed).
  - **WebStream**:
    - Removed default error logging; errors are now only returned in the error variable.
    - Added the ability to check the socket state.

---

## v0.2.12

### Release Date: 2024-08-17

### Changes
- **Turbo-restler**:
  - **chore**: Update WebStream Socket method to use the 'socket' variable
    - This commit updates the `Socket` method in the `WebStream` struct to use the `socket` variable instead of the deprecated `stream` variable. This change ensures consistency and clarity in the codebase.
    - **Note**: This commit is based on recent user commits and repository commits.

---

## v0.2.11

### Release Date: 2024-08-17

### Changes
- **Turbo-restler**:
  - **chore**: Update WebStream Subscribe method to support multiple subscriptions
    - This commit modifies the `Subscribe` method in the `WebStream` struct to accept multiple subscriptions as variadic parameters. It checks if there are any subscriptions provided and sends a subscription request for each one. This change improves the flexibility and usability of the `Subscribe` method.

---

## v0.2.10

### Release Date: 2024-08-17

### Changes
- **Turbo-restler**:
  - Added functions for managing stream readers to improve flexibility and control over stream handling.

### Commits
#### commit 772e8c65cf818c8c5cb35db5902af1267cfb83f9
- **Author**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Commit**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Description**:
  - **chore**: Update WebStream Subscribe method to support multiple subscriptions
  - This commit modifies the `Subscribe` method in the `WebStream` struct to accept multiple subscriptions as variadic parameters. It checks if there are any subscriptions provided and sends a subscription request for each one. This change improves the flexibility and usability of the `Subscribe` method.
  - **Note**: This commit is based on recent user commits and repository commits.

#### commit edc29ea9881bf13782aeff45abf06b2de2925030
- **Author**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Commit**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Description**:
  - **chore**: Update SUBSCRIBE_ID constant value in stream.go
  - This commit updates the value of the SUBSCRIBE_ID constant in the stream.go file. The new value is SUBSCRIBE_ID + 1, which ensures that the constant is incremented correctly. This change is necessary to align with recent user and repository commits related to the WebSocket connection.
  - **Note**: This commit is based on recent user commits and repository commits.

#### commit 2f0bc200afe06c77b4442d686f4dca6ba5fa2c87
- **Author**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Commit**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Description**:
  - **chore**: Update WebSocket connection to handle ping and pong messages
  - This commit modifies the `web_api.go` file to handle ping and pong messages in the WebSocket connection. It sets up handlers for ping and pong messages, and sends a pong message in response to a ping message. This improves the reliability and responsiveness of the WebSocket connection.
  - **Note**: This commit is based on recent user commits and repository commits.

#### commit 6ffb830553324d67028cb061a67be894511a6b1d
- **Author**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Commit**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Description**:
  - **chore**: Update WebSocket connection to use wss scheme

#### commit 2a1e12b2cf97bbccbbe5218c15f635e216d04e40
- **Author**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Commit**: Aleksey Kislitsa <aleksey.kislitsa@gmail.com>
- **Description**:
  - **chore**: Refactor WebSocket connection to use wss scheme

---

## v0.2.9

### Release Date: 2024-08-17

### Changes
- **Turbo-restler**:
  - Added functions for managing stream readers to improve flexibility and control over stream handling.

### Example Usage
#### New Functions for Managing Stream Readers
```go
package main

import (
    "fmt"
    "log"
    "github.com/fr0ster/turbo-restler/web_api"
    "github.com/fr0ster/turbo-restler/web_stream"
)

func main() {
		// Start the streamer
	stream, err := web_stream.New(
		web_api.WsHost("localhost:8080"),
		web_api.WsPath("/stream"),
		web_api.SchemeWS)
	if err != nil {
		logrus.Fatal(err)
	}

	err = stream.Start()
	if err != nil {
		logrus.Fatal(err)
	}

	err = stream.AddHandler("stream", mockHandler)
	if err != nil {
		logrus.Fatal(err)
	}
	err = stream.Subscribe("stream")
	if err != nil {
		logrus.Fatal(err)
	}

	err = stream.Start()
	if err != nil {
		logrus.Fatal(err)
	}

	stream.RemoveHandler("default")

	// Stop the streamer after some time
	time.Sleep(timeOut)
	stream.Stop()
}
```
---

## v0.2.8

### Release Date: 2024-08-16

### Changes
- **Turbo-restler**:
  - Removed timeout functionality from `WebStream.Start` to simplify connection management.


---

## v0.2.7

### Release Date: 2024-08-16

### Changes
- **Turbo-restler**:
  - Added functions for managing stream readers:
    - `func (ws *WebStream) SetHandler(handler WsHandler) *WebStream`
    - `func (ws *WebStream) SetErrHandler(errHandler ErrHandler) *WebStream`
    - `func (ws *WebStream) AddSubscriptions(handlerId string, handler WsHandler)`
    - `func (ws *WebStream) RemoveSubscriptions(handlerId string)`

#### Example Usage
##### Managing Stream Readers
```go
package main

import (
    "fmt"
    "log"
    "github.com/fr0ster/turbo-restler/web_stream"
)

func main() {
    ws := web_stream.NewWebStream("wss://example.com/stream")

    // Set handler
    ws.SetHandler(func(message []byte) {
        fmt.Println("Received message:", string(message))
    })

    // Set error handler
    ws.SetErrHandler(func(err error) {
        log.Println("Error:", err)
    })

    // Add subscription
    ws.AddSubscriptions("handler1", func(message []byte) {
        fmt.Println("Handler1 received message:", string(message))
    })

    // Remove subscription
    ws.RemoveSubscriptions("handler1")
}
```

---

## v0.2.6

### Release Date: 2024-08-15

### Changes
- **Turbo-restler**:
  - Fixed date errors in `Release.md` to ensure accurate release information.

---

## Release Notes for Turbo-Restler v0.2.5

### Release Date: 2024-08-15

### Changes
- **Turbo-restler**:
  - Added a check to ensure that the test server is started only once. This improves testing efficiency and prevents conflicts when running multiple tests simultaneously.

### Example Usage
#### Turbo-restler Server Initialization
```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "sync"
)

var once sync.Once

func startTestServer() {
    once.Do(func() {
        http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
            fmt.Printf(w, "Hello, World!")
        })
        log.Fatal(http.ListenAndServe(":8080", nil))
    })
}

func main() {
    startTestServer()
    // Your test code here
}
```

---

## v0.2.4

### Release Date: 2024-08-15

#### Introduction
Turbo-Restler is a library designed to facilitate RESTful API interactions. This release includes significant improvements and new features to enhance the functionality and reliability of the library.

#### New Features
- **Updated Turbo-Signer Dependency**: Upgraded to `github.com/fr0ster/turbo-signer` version `v0.1.4`, which includes new features and improvements.

---
## v0.2.3

### Release Date: 2024-08-15

#### Introduction
Turbo-Restler is a library designed to facilitate RESTful API interactions. This release includes significant improvements and new features to enhance the functionality and reliability of the library.

#### New Features
- **Updated Turbo-Signer Dependency**: Upgraded to `github.com/fr0ster/turbo-signer` version `v0.1.3`, which includes new features and improvements.

---

## v0.2.2

### Release Date: 2024-08-15

#### Introduction
Turbo-Restler is a library designed to facilitate RESTful API interactions. This release includes significant improvements and new features to enhance the functionality and reliability of the library.

#### New Features
- **Web API Tests**: Added comprehensive tests for the `web_api` package to ensure the reliability and correctness of the API interactions.
- **Refactored WebApi Call Function**: The `Call` function in the `WebApi` package has been refactored into four separate functions:
  - `Send`: Handles sending the request.
  - `Read`: Handles reading the response.
  - `Serialise`: Handles serializing the request data.
  - `Deserialise`: Handles deserializing the response data.
- **Updated Turbo-Signer Dependency**: Upgraded to `github.com/fr0ster/turbo-signer` version `v0.1.2`, which includes new features and improvements.

#### Usage
To use Turbo-Restler, import the library and call the appropriate functions to interact with your RESTful API.

Example:
```go
package main

import (
    "fmt"
    "log"
    "github.com/fr0ster/turbo-restler/web_api"
)

func main() {
    // Create a new WebApi instance
	api, err := web_api.New(web_api.WsHost("localhost:8080"), web_api.WsPath("/ws"), web_api.SchemeWS)
	if err != nil {
		log.Fatal("New error:", err)
	}
	// Example usage of the refactored functions
	// Create a sample request JSON
	request := simplejson.New()
	request.Set("id", 1)
	request.Set("method", "with-params")
	request.Set("params", "Hello, World!")
	// Відправка запиту з параметрами в simplejson
	err = api.Send(request)
	if err != nil {
		log.Fatal("Send error:", err)
	}

	// Читання відповіді в simplejson
	response, err := api.Read()
	if err != nil {
		log.Fatal("Read error:", err)
	}
	fmt.Println("Response data:", response)

	// Відправка запиту з параметрами в JSON
	requestBody := api.Serialize(request)
	err = api.Socket().WriteMessage(websocket.TextMessage, requestBody)
	if err != nil {
		logrus.Errorf("error sending message: %v", err)
		return
	}

	// Десеріалізація відповіді JSON в simplejson
	_, body, err := api.Socket().ReadMessage()
	if err != nil {
		logrus.Errorf("error reading message: %v", err)
		return
	}
	data := api.Deserialize(body)
	fmt.Println("Response data:", data)
}
```

## v0.2.1

### Release Date: 2024-08-14

#### Introduction
Turbo-Restler is a wrapper library designed to simplify interactions with RESTful APIs over HTTP and WebSocket APIs using `gorilla/websocket`. This library provides an intuitive and efficient way to integrate API calls into your applications, handling the complexities of HTTP requests and WebSocket connections.

#### New Features
- **RESTful API Wrapper**: Simplifies making HTTP requests to RESTful APIs, including support for GET, POST, PUT, DELETE, and other HTTP methods.
- **WebSocket API Wrapper**: Provides an easy-to-use interface for establishing and managing WebSocket connections using `gorilla/websocket`.
- **JSON Handling**: Integrates with `go-simplejson` for easy manipulation of JSON data in API responses.
- **Logging**: Utilizes `logrus` for structured logging, making it easier to debug and monitor API interactions.
- **Testing Utilities**: Includes support for `stretchr/testify` to facilitate unit testing of API interactions.

#### Dependencies
- **go-simplejson v0.5.1**: For easy JSON manipulation.
- **gorilla/websocket v1.5.3**: For WebSocket connections.
- **stretchr/testify v1.9.0**: For testing utilities.
- **logrus v1.9.3**: For structured logging.
- **turbo-signer v0.0.0-20240814090304-a777f0da004f**: For signing API request parameters.

#### Usage
To use Turbo-Restler, import the library and call the appropriate functions to interact with RESTful and WebSocket APIs.

Example for RESTful API:
```go
package main

import (
	"fmt"
	"net/http"

	rest_api "github.com/fr0ster/turbo-restler/rest_api"
)

func main() {
    sign := signature.NewSignHMAC(signature.PublicKey("api_key"), signature.SecretKey("api_secret"))
	response, err := rest_api.CallRestAPI(ra.apiBaseUrl, http.MethodPost, nil, "/fapi/v1/listenKey", ra.sign)
	if err != nil {
		err = fmt.Errorf("error calling API: %v", err)
		return
	}
	listenKey = response.Get("listenKey").MustString()
    fmt.Println("listenKey:", listenKey)
}
```