package web_socket_test

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/web_socket"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// Піднімаємо WebSocket сервер
func startWebSocketServer() {
	onceSync.Do(func() {
		http.HandleFunc("/ws", handleWithParams)

		logrus.Println("Starting server on :8081")
		log.Fatal(http.ListenAndServe(":8081", nil))
	})
}

var upgraderSync = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Обробник для методу з параметрами
func handleWithParams(w http.ResponseWriter, r *http.Request) {
	conn, err := upgraderSync.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			return
		}
		log.Printf("Received message: %s", message)

		js, err := simplejson.NewJson(message)
		if err != nil {
			log.Println("Error parsing JSON:", err)
			return
		}
		method, _ := js.Get("method").String()

		switch method {
		case "with-params":
			params := js.Get("params")
			log.Printf("Received method: %s, params: %s", method, params)
			response := fmt.Sprintf("Response from with-params method: received %s", message)
			err = conn.WriteMessage(websocket.TextMessage, []byte(response))
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		case "break":
			log.Println("Simulating abrupt close on client request")
			conn.Close() // Раптовий обрив без CloseMessage
			return
		case "close-normally":
			log.Println("Simulating NORMAL close on client request")
			conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing per client request"),
				time.Now().Add(time.Second))
			conn.Close() // нормальне закриття з CloseMessage
			return
		default:
			logrus.Errorf("Unknown method: %s", method)
		}
	}
}

func TestWebApiTextMessage(t *testing.T) {
	go startWebSocketServer()
	time.Sleep(timeOut)
	// Create a new WebApi instance
	api, err := web_socket.New(
		web_socket.WsHost("localhost:8081"),
		web_socket.WsPath("/ws"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		true)
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
	err = api.Send(request)
	if err != nil {
		logrus.Errorf("error sending message: %v", err)
		return
	}

	// Десеріалізація відповіді JSON в simplejson
	data, err := api.Read()
	if err != nil {
		logrus.Errorf("error reading message: %v", err)
		return
	}
	fmt.Println("Response data:", data)

	// TODO: Add more assertions for the request sending process
}

func TestWebApiBinaryMessage(t *testing.T) {
	go startWebSocketServer()
	time.Sleep(timeOut)
	// Create a new WebApi instance
	api, err := web_socket.New(
		web_socket.WsHost("localhost:8081"),
		web_socket.WsPath("/ws"),
		web_socket.SchemeWS,
		web_socket.BinaryMessage,
		true)
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
	err = api.Send(request)
	if err != nil {
		logrus.Errorf("error sending message: %v", err)
		return
	}

	// Десеріалізація відповіді JSON в simplejson
	data, err := api.Read()
	if err != nil {
		logrus.Errorf("error reading message: %v", err)
		return
	}
	fmt.Println("Response data:", data)

	// TODO: Add more assertions for the request sending process
}

func TestWebApiAbruptServerClose(t *testing.T) {
	go startWebSocketServer()
	time.Sleep(timeOut)

	api, err := web_socket.New(
		web_socket.WsHost("localhost:8081"),
		web_socket.WsPath("/ws"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		true)
	if err != nil {
		t.Fatal("New error:", err)
	}

	closeCalled := false
	api.SetCloseHandler(func(code int, text string) error {
		t.Logf("CLOSE HANDLER: code=%d, msg=%s", code, text)
		closeCalled = true
		return nil
	})

	// Надсилаємо запит на раптове завершення
	req := simplejson.New()
	req.Set("id", 1)
	req.Set("method", "break")

	err = api.Send(req)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	_, err = api.Read()
	if err == nil {
		t.Error("Expected error due to abrupt close, got nil")
	} else {
		t.Logf("Got expected abrupt close error: %v", err)
	}

	_, err = api.Read()
	if err == nil {
		t.Error("Expected error due to abrupt close, got nil")
	} else {
		t.Logf("Got expected abrupt close error: %v", err)
	}

	if closeCalled {
		t.Error("CloseHandler should NOT be called for abrupt close (code 1006)")
	}
}

func TestWebApiNormalClose(t *testing.T) {
	go startWebSocketServer()
	time.Sleep(timeOut)

	api, err := web_socket.New(
		web_socket.WsHost("localhost:8081"),
		web_socket.WsPath("/ws"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		true)
	if err != nil {
		t.Fatal("New error:", err)
	}

	closeCalled := false
	api.SetCloseHandler(func(code int, text string) error {
		t.Logf("CLOSE HANDLER: code=%d, msg=%s", code, text)
		closeCalled = true
		return nil
	})

	req := simplejson.New()
	req.Set("id", 1)
	req.Set("method", "close-normally")

	err = api.Send(req)
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	_, _ = api.Read()

	if !closeCalled {
		t.Error("Expected CloseHandler to be called on normal close")
	}
}

func TestWebApiParallelRequests(t *testing.T) {
	go startWebSocketServer()
	time.Sleep(timeOut)

	const parallelClients = 2
	var wg sync.WaitGroup
	wg.Add(parallelClients)

	for i := 0; i < parallelClients; i++ {
		go func(id int) {
			defer wg.Done()

			api, err := web_socket.New(
				web_socket.WsHost("localhost:8081"),
				web_socket.WsPath("/ws"),
				web_socket.SchemeWS,
				web_socket.TextMessage,
				true)
			if err != nil {
				t.Errorf("Client %d: error creating WebSocket API: %v", id, err)
				return
			}

			request := simplejson.New()
			request.Set("id", id)
			request.Set("method", "with-params")
			request.Set("params", fmt.Sprintf("Hello from client %d", id))

			err = api.Send(request)
			if err != nil {
				t.Errorf("Client %d: Send error: %v", id, err)
				return
			}

			response, err := api.Read()
			if err != nil {
				t.Errorf("Client %d: Read error: %v", id, err)
				return
			}

			t.Logf("Client %d received: %s", id, response.MustString())

			expectedParam := fmt.Sprintf("Hello from client %d", id)
			actualParam, _ := response.Get("response").String()
			if !contains(actualParam, expectedParam) {
				t.Errorf("Client %d: expected '%s', got '%s'", id, expectedParam, actualParam)
			}
			// }
		}(i)
	}

	wg.Wait()
}

func TestWebApiParallelRequestsV2(t *testing.T) {
	go startWebSocketServer()
	time.Sleep(timeOut)

	const parallelClients = 20
	var wg sync.WaitGroup
	wg.Add(parallelClients)

	api, err := web_socket.New(
		web_socket.WsHost("localhost:8081"),
		web_socket.WsPath("/ws"),
		web_socket.SchemeWS,
		web_socket.TextMessage,
		true)
	if err != nil {
		t.Errorf("Client: error creating WebSocket API: %v", err)
		return
	}

	lock := sync.Mutex{}

	for i := 0; i < parallelClients; i++ {
		go func(id int) {
			defer wg.Done()

			request := simplejson.New()
			request.Set("id", id)
			request.Set("method", "with-params")
			request.Set("params", fmt.Sprintf("Hello from client %d", id))

			lock.Lock()
			err = api.Send(request)
			if err != nil {
				t.Errorf("Client %d: Send error: %v", id, err)
				return
			}

			response, err := api.Read()
			lock.Unlock()
			if err != nil {
				t.Errorf("Client %d: Read error: %v", id, err)
				return
			}

			t.Logf("Client %d received: %s", id, response.MustString())

			expectedParam := fmt.Sprintf("Hello from client %d", id)
			actualParam, _ := response.Get("response").String()
			if !contains(actualParam, expectedParam) {
				t.Errorf("Client %d: expected '%s', got '%s'", id, expectedParam, actualParam)
			}
			// }
		}(i)
	}

	wg.Wait()
}

// contains - простий хелпер для перевірки підрядка
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
