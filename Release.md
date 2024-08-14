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