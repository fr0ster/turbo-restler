package web_api_test

import (
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/bitly/go-simplejson"
	web_api "github.com/fr0ster/turbo-restler/web_api"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var (
	quit chan struct{}
)

// Піднімаємо WebSocket сервер
func startWebSocketServer() (err error) {
	http.HandleFunc("/ws", handleWithParams)

	logrus.Println("Starting server on :8080")
	err = http.ListenAndServe(":8080", nil)
	return
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Обробник для методу з параметрами
func handleWithParams(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		select {
		case <-quit:
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				break
			}
			log.Printf("Received message: %s", message)
			js, err := simplejson.NewJson(message)
			if err != nil {
				log.Println("Error parsing JSON:", err)
				break
			}
			method, _ := js.Get("method").String()
			if method == "with-params" {
				params := js.Get("params")
				log.Printf("Received method: %s, params: %s", method, params)
			} else {
				logrus.Errorf("Unknown method: %s", method)
			}

			// Тут можна обробити параметри з повідомлення
			response := fmt.Sprintf("Response from with-params method: received %s", message)
			err = conn.WriteMessage(websocket.TextMessage, []byte(response))
			if err != nil {
				log.Println("Write error:", err)
				return
			}
		}
	}
}

func TestWebApi(t *testing.T) {
	go func() {
		err := startWebSocketServer()
		if err != nil {
			log.Fatal("Error starting server:", err)
		}
	}()
	// Create a new WebApi instance
	wa, err := web_api.New(web_api.WsHost("localhost:8080"), web_api.WsPath("/ws"), web_api.SchemeWS)
	assert.NoError(t, err)

	// Create a sample request JSON
	request := simplejson.New()
	request.Set("id", 1)
	request.Set("method", "with-params")
	request.Set("params", "Hello, World!")

	// Send the request
	err = wa.Send(request)
	assert.NoError(t, err)

	// Read the response
	response, err := wa.Read()
	assert.NoError(t, err)
	assert.NotNil(t, response)

	// TODO: Add more assertions for the request sending process
	time.Sleep(1 * time.Second)
	logrus.Println("Server stopped")
}

func TestWebApi_Send(t *testing.T) {
	go func() {
		err := startWebSocketServer()
		if err != nil {
			log.Fatal("Error starting server:", err)
		}
	}()
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
