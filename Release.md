# Release Notes for Turbo-Restler v0.2.5

## Release Date: 2024-08-15

## Changes
- **Turbo-restler**:
  - Fixed date errors in `Release.md` to ensure accurate release information.

---

# Release Notes for Turbo-Restler v0.2.5

## Release Date: 2024-08-15

## Changes
- **Turbo-restler**:
  - Added a check to ensure that the test server is started only once. This improves testing efficiency and prevents conflicts when running multiple tests simultaneously.

## Example Usage
### Turbo-restler Server Initialization
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

# Release Notes for Turbo-Restler v0.2.4

## Release Date: 2024-08-15

### Introduction
Turbo-Restler is a library designed to facilitate RESTful API interactions. This release includes significant improvements and new features to enhance the functionality and reliability of the library.

### New Features
- **Updated Turbo-Signer Dependency**: Upgraded to `github.com/fr0ster/turbo-signer` version `v0.1.4`, which includes new features and improvements.

---
# Release Notes for Turbo-Restler v0.2.3

## Release Date: 2024-08-15

### Introduction
Turbo-Restler is a library designed to facilitate RESTful API interactions. This release includes significant improvements and new features to enhance the functionality and reliability of the library.

### New Features
- **Updated Turbo-Signer Dependency**: Upgraded to `github.com/fr0ster/turbo-signer` version `v0.1.3`, which includes new features and improvements.

---

# Release Notes for Turbo-Restler v0.2.2

## Release Date: 2024-08-15

### Introduction
Turbo-Restler is a library designed to facilitate RESTful API interactions. This release includes significant improvements and new features to enhance the functionality and reliability of the library.

### New Features
- **Web API Tests**: Added comprehensive tests for the `web_api` package to ensure the reliability and correctness of the API interactions.
- **Refactored WebApi Call Function**: The `Call` function in the `WebApi` package has been refactored into four separate functions:
  - `Send`: Handles sending the request.
  - `Read`: Handles reading the response.
  - `Serialise`: Handles serializing the request data.
  - `Deserialise`: Handles deserializing the response data.
- **Updated Turbo-Signer Dependency**: Upgraded to `github.com/fr0ster/turbo-signer` version `v0.1.2`, which includes new features and improvements.

### Usage
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

# Release Notes for Turbo-Restler v0.2.1

## Release Date: 2024-08-14

### Introduction
Turbo-Restler is a wrapper library designed to simplify interactions with RESTful APIs over HTTP and WebSocket APIs using `gorilla/websocket`. This library provides an intuitive and efficient way to integrate API calls into your applications, handling the complexities of HTTP requests and WebSocket connections.

### New Features
- **RESTful API Wrapper**: Simplifies making HTTP requests to RESTful APIs, including support for GET, POST, PUT, DELETE, and other HTTP methods.
- **WebSocket API Wrapper**: Provides an easy-to-use interface for establishing and managing WebSocket connections using `gorilla/websocket`.
- **JSON Handling**: Integrates with `go-simplejson` for easy manipulation of JSON data in API responses.
- **Logging**: Utilizes `logrus` for structured logging, making it easier to debug and monitor API interactions.
- **Testing Utilities**: Includes support for `stretchr/testify` to facilitate unit testing of API interactions.

### Dependencies
- **go-simplejson v0.5.1**: For easy JSON manipulation.
- **gorilla/websocket v1.5.3**: For WebSocket connections.
- **stretchr/testify v1.9.0**: For testing utilities.
- **logrus v1.9.3**: For structured logging.
- **turbo-signer v0.0.0-20240814090304-a777f0da004f**: For signing API request parameters.

### Usage
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