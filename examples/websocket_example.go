package main

import (
	"fmt"
	"log"
	"time"

	"github.com/fr0ster/turbo-restler/web_socket"
	"github.com/gorilla/websocket"
)

func main() {
	// Приклад 1: Базове використання з конфігурацією
	config := web_socket.WebSocketConfig{
		URL:            "wss://stream.binance.com:9443/ws/btcusdt@trade",
		Dialer:         websocket.DefaultDialer,
		BufferSize:     256,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   10 * time.Second,
		PingInterval:   30 * time.Second,
		PongWait:       60 * time.Second,
		MaxMessageSize: 1024 * 1024, // 1MB
		EnableMetrics:  true,
		ReconnectConfig: &web_socket.ReconnectConfig{
			MaxAttempts:         5,
			InitialDelay:        1 * time.Second,
			MaxDelay:            30 * time.Second,
			BackoffMultiplier:   2.0,
			EnableAutoReconnect: true,
		},
	}

	ws, err := web_socket.NewWebSocketWrapperWithConfig(config)
	if err != nil {
		log.Fatal("Failed to create WebSocket:", err)
	}

	// Налаштування логера
	ws.SetMessageLogger(func(log web_socket.LogRecord) {
		switch log.Level {
		case web_socket.LogLevelError:
			fmt.Printf("❌ [%s] ERROR: %v\n", log.Op, log.Err)
		case web_socket.LogLevelWarn:
			fmt.Printf("⚠️  [%s] WARN: %s\n", log.Op, string(log.Body))
		case web_socket.LogLevelInfo:
			fmt.Printf("ℹ️  [%s] INFO: %s\n", log.Op, string(log.Body))
		default:
			fmt.Printf("🔍 [%s] DEBUG: %s\n", log.Op, string(log.Body))
		}
	})

	// Підписка на повідомлення
	subID := ws.Subscribe(func(evt web_socket.MessageEvent) {
		switch evt.Kind {
		case web_socket.KindData:
			fmt.Printf("📨 Received data: %s\n", string(evt.Body))
		case web_socket.KindError:
			fmt.Printf("❌ Error: %v\n", evt.Error)
		case web_socket.KindControl:
			fmt.Printf("🎛️  Control message: %s\n", string(evt.Body))
		}
	})

	// Обробники подій
	ws.SetConnectedHandler(func() {
		fmt.Println("✅ Connected to WebSocket")
	})

	ws.SetDisconnectHandler(func() {
		fmt.Println("❌ Disconnected from WebSocket")
	})

	// Запуск WebSocket
	ws.Open()

	// Очікування підключення
	if !ws.WaitStarted() {
		log.Fatal("Failed to start WebSocket")
	}

	// Відправка тестового повідомлення
	err = ws.Send(web_socket.WriteEvent{
		Body: []byte("ping"),
		Callback: func(err error) {
			if err != nil {
				fmt.Printf("❌ Send error: %v\n", err)
			} else {
				fmt.Println("✅ Message sent successfully")
			}
		},
	})
	if err != nil {
		log.Printf("Failed to send message: %v", err)
	}

	// Робота протягом 30 секунд
	time.Sleep(30 * time.Second)

	// Закриття з'єднання
	fmt.Println("🔄 Closing WebSocket...")
	ws.Close()
	ws.WaitStopped()

	// Відписка
	ws.Unsubscribe(subID)

	fmt.Println("✅ WebSocket example completed")
}

// Приклад 2: Використання з метриками
func metricsExample() {
	config := web_socket.WebSocketConfig{
		URL:           "wss://example.com/ws",
		Dialer:        websocket.DefaultDialer,
		EnableMetrics: true,
	}

	ws, err := web_socket.NewWebSocketWrapperWithConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	// Запуск метрик збору
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Тут можна додати логіку збору метрик
			fmt.Println("📊 Collecting metrics...")
		}
	}()

	ws.Open()
	time.Sleep(10 * time.Second)
	ws.Close()
}
