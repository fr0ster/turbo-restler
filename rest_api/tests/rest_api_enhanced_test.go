package rest_api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	rest_api "github.com/fr0ster/turbo-restler/rest_api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker(t *testing.T) {
	t.Parallel()

	// Тест 1: Створення Circuit Breaker з кастомним конфігом
	cb := rest_api.NewCircuitBreaker(&rest_api.CircuitBreakerConfig{
		FailureThreshold: 5,
		RecoveryTimeout:  100 * time.Millisecond,
		HalfOpenLimit:    3,
	})
	assert.NotNil(t, cb)

	// Тест 2: Перевірка стану Closed
	assert.True(t, cb.CanExecute())

	// Тест 3: Симуляція невдач (FailureThreshold = 5)
	for i := 0; i < 5; i++ {
		cb.OnFailure()
	}
	// Після 5 невдач виклики мають блокуватись
	assert.False(t, cb.CanExecute())

	// Тест 4: Очікування відновлення
	time.Sleep(150 * time.Millisecond)
	// Після RecoveryTimeout дозволяємо спробу (Half-Open)
	assert.True(t, cb.CanExecute())

	// Тест 5: Успішне відновлення
	cb.OnSuccess()
	assert.True(t, cb.CanExecute())
}

func TestRestAPIConfig(t *testing.T) {
	t.Parallel()

	// Тест 1: Створення конфігурації з дефолтними значеннями
	config := &rest_api.RestAPIConfig{}
	// Створюємо валідний request замість nil
	req, _ := http.NewRequest("GET", "http://invalid-url", nil)
	response, err := rest_api.CallRestAPIWithConfig(req, config)
	assert.Error(t, err)
	assert.Nil(t, response)

	// Тест 2: Тест з retry логікою
	server := StartHandlerServer(func(w http.ResponseWriter, r *http.Request) {
		// Перші 2 запити повертають помилку, третій - успіх
		if r.Header.Get("X-Attempt") == "3" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success"}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
		}
	})
	defer server.Close()

	req, err = http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	config = &rest_api.RestAPIConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 3,
		RetryDelay: 100 * time.Millisecond,
	}

	// Відправляємо запити з лічильником спроб
	for attempt := 1; attempt <= 3; attempt++ {
		req.Header.Set("X-Attempt", fmt.Sprintf("%d", attempt))
		response, err = rest_api.CallRestAPIWithConfig(req, config)
		if attempt == 3 {
			assert.NoError(t, err)
			assert.NotNil(t, response)
		} else {
			assert.Error(t, err)
		}
	}
}

func TestRestAPIConfigWithCircuitBreaker(t *testing.T) {
	t.Parallel()

	// Тест 1: Створення конфігурації з Circuit Breaker
	config := &rest_api.RestAPIConfig{
		Timeout:    1 * time.Second,
		MaxRetries: 0, // Без retry, щоб кожен виклик = 1 помилка
		RetryDelay: 100 * time.Millisecond,
		CircuitBreaker: &rest_api.CircuitBreakerConfig{
			FailureThreshold: 3, // Після 3 викликів = 3 помилки → OPEN
			RecoveryTimeout:  200 * time.Millisecond,
			HalfOpenLimit:    2,
		},
	}

	// Тест 2: Симуляція сервера, який завжди повертає помилки
	server := StartStaticJSONServer(http.StatusInternalServerError, `{"error": "always failing"}`)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	// Відправляємо запити до спрацьовування Circuit Breaker
	// Використовуємо ту саму конфігурацію для збереження стану Circuit Breaker
	for i := 0; i < 4; i++ {
		response, err := rest_api.CallRestAPIWithConfig(req, config)
		if i < 3 {
			// Перші 3 запити мають провалитися з HTTP помилкою
			// За стандартами HTTP, при помилці 5xx response body має бути
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "HTTP error 500")
			assert.NotNil(t, response) // Response має бути з деталями помилки

			// Перевіряємо, що response містить деталі помилки
			errorMsg, _ := response.Get("error").String()
			assert.Equal(t, "always failing", errorMsg)
		} else {
			// 4-й запит має бути заблокований Circuit Breaker
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "circuit breaker is open")
			// При Circuit Breaker помилці response має бути nil
			assert.Nil(t, response)
		}
	}

	// Тест 3: Очікування відновлення Circuit Breaker
	time.Sleep(300 * time.Millisecond)

	// Тест 4: Перевірка переходу в Half-Open стан з успішним сервером
	// Після відновлення створюємо успішний сервер
	successServer := StartStaticJSONServer(http.StatusOK, `{"status": "recovered"}`)
	defer successServer.Close()

	req, err = http.NewRequest("GET", successServer.URL, nil)
	require.NoError(t, err)

	// Використовуємо ту саму конфігурацію для тестування відновлення
	response, err := rest_api.CallRestAPIWithConfig(req, config)
	assert.NoError(t, err) // Після відновлення має бути успіх
	assert.NotNil(t, response)

	// Перевіряємо успішну відповідь
	status, _ := response.Get("status").String()
	assert.Equal(t, "recovered", status)

}

func TestBackwardCompatibility(t *testing.T) {
	t.Parallel()

	// Тест: Перевірка зворотної сумісності з оригінальною функцією
	server := StartStaticJSONServer(http.StatusOK, `{"message": "hello"}`)
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	// Виклик оригінальної функції
	response, err := rest_api.CallRestAPI(req)
	assert.NoError(t, err)
	assert.NotNil(t, response)

	// Перевірка вмісту
	message, err := response.Get("message").String()
	assert.NoError(t, err)
	assert.Equal(t, "hello", message)
}

// Тест для перевірки, що при мережевих помилках response є nil
func TestNetworkErrorResponseIsNil(t *testing.T) {
	t.Parallel()

	// Створюємо request до неіснуючого сервера
	req, _ := http.NewRequest("GET", "http://invalid-server-that-does-not-exist.com", nil)

	config := &rest_api.RestAPIConfig{
		Timeout:    100 * time.Millisecond, // Короткий таймаут для швидкого тесту
		MaxRetries: 1,
		RetryDelay: 10 * time.Millisecond,
	}

	response, err := rest_api.CallRestAPIWithConfig(req, config)

	// При мережевій помилці response має бути nil
	assert.Error(t, err)
	assert.Nil(t, response)
	assert.Contains(t, err.Error(), "request failed")
}

// Тест: Можливість передати зовнішній http.Client (таймаути/проксі)
func TestExternalHTTPClientInjection(t *testing.T) {
	t.Parallel()

	// Піднімаємо простий тестовий сервер
	server := StartStaticJSONServer(http.StatusOK, `{"ok":true}`)
	defer server.Close()

	// Піднімаємо HTTP-проксі (мінімалістичний forward proxy)
	proxy, hits, purl := StartForwardProxy()
	defer proxy.Close()

	// Користувацький транспорт із проксі
	customClient := NewHTTPClientWithProxy(purl, 777*time.Millisecond, nil)

	cfg := &rest_api.RestAPIConfig{HTTPClient: customClient}

	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := rest_api.CallRestAPIWithConfig(req, cfg)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Перевіряємо, що таймаут у конфіг синхронізувався з клієнта
	assert.Equal(t, customClient.Timeout, cfg.Timeout)
	// Перевіряємо, що запит пішов через проксі
	assert.GreaterOrEqual(t, ProxyHits(hits), int32(1))
}
