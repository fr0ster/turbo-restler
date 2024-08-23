package web_socket_test

import (
	"fmt"
	"log"
	"net/http"
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

func TestWebApiTextMessage(t *testing.T) {
	go startWebSocketServer()
	time.Sleep(timeOut)
	// Create a new WebApi instance
	api, err := web_socket.New(web_socket.WsHost("localhost:8081"), web_socket.WsPath("/ws"), web_socket.SchemeWS, web_socket.TextMessage)
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
	api, err := web_socket.New(web_socket.WsHost("localhost:8081"), web_socket.WsPath("/ws"), web_socket.SchemeWS, web_socket.BinaryMessage)
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
