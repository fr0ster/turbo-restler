package rest_api

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/bitly/go-simplejson"
)

type (
	ApiBaseUrl string
	EndPoint   string
	HttpMethod string
)

// Конфігурація REST API
type RestAPIConfig struct {
	// HTTPClient дозволяє передати зовнішній http.Client (із власними Timeout/Proxy/Transport)
	// Якщо встановлено, Restler використовує цей клієнт і синхронізує Timeout у конфігурацію.
	HTTPClient     *http.Client
	Timeout        time.Duration
	MaxRetries     int
	RetryDelay     time.Duration
	CircuitBreaker *CircuitBreakerConfig
	cbInstance     *CircuitBreaker // Внутрішній екземпляр Circuit Breaker
}

// Circuit Breaker конфігурація
type CircuitBreakerConfig struct {
	FailureThreshold int
	RecoveryTimeout  time.Duration
	HalfOpenLimit    int
}

// Circuit Breaker стан
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

type CircuitBreaker struct {
	state        CircuitBreakerState
	failureCount int
	lastFailure  time.Time
	config       *CircuitBreakerConfig
	mu           sync.RWMutex
}

// Створення нового Circuit Breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = &CircuitBreakerConfig{
			FailureThreshold: 5,
			RecoveryTimeout:  30 * time.Second,
			HalfOpenLimit:    3,
		}
	}
	return &CircuitBreaker{
		state:  StateClosed,
		config: config,
	}
}

// Перевірка чи можна виконати запит
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.config.RecoveryTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.failureCount = 0 // Скидаємо лічильник помилок
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case StateHalfOpen:
		return cb.failureCount < cb.config.HalfOpenLimit
	default:
		return false
	}
}

// Реєстрація успішного виконання
func (cb *CircuitBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failureCount = 0
	}
}

// Реєстрація невдачі
func (cb *CircuitBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.failureCount >= cb.config.FailureThreshold {
		cb.state = StateOpen
	}
}

// Функція виклику REST API з покращеною обробкою помилок
func CallRestAPIWithConfig(req *http.Request, config *RestAPIConfig) (
	response *simplejson.Json, err error) {

	if config == nil {
		config = &RestAPIConfig{
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
		}
	}

	// Визначення HTTP клієнта
	var client *http.Client
	if config.HTTPClient != nil {
		// Використовуємо зовнішній клієнт. Синхронізуємо таймаут у конфіг.
		client = config.HTTPClient
		config.Timeout = client.Timeout
	} else {
		// Створюємо дефолтний клієнт з таймаутом з конфігурації
		client = &http.Client{Timeout: config.Timeout}
	}

	// Створення або отримання Circuit Breaker якщо потрібно
	var cb *CircuitBreaker
	if config.CircuitBreaker != nil {
		if config.cbInstance == nil {
			config.cbInstance = NewCircuitBreaker(config.CircuitBreaker)
		}
		cb = config.cbInstance
	}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Перевірка Circuit Breaker
		if cb != nil && !cb.CanExecute() {
			return nil, fmt.Errorf("circuit breaker is open")
		}

		// Виконання запиту
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d): %v", attempt+1, err)
			if cb != nil {
				cb.OnFailure()
			}

			if attempt < config.MaxRetries {
				time.Sleep(config.RetryDelay * time.Duration(attempt+1))
				continue
			}
			return nil, lastErr
		}

		// Читання відповіді
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body (attempt %d): %v", attempt+1, err)
			if cb != nil {
				cb.OnFailure()
			}

			if attempt < config.MaxRetries {
				time.Sleep(config.RetryDelay * time.Duration(attempt+1))
				continue
			}
			return nil, lastErr
		}

		// Парсинг JSON
		response, err = simplejson.NewJson(body)
		if err != nil {
			response = simplejson.New()
			response.Set("message", string(body))
		}

		// Перевірка HTTP помилок
		if resp.StatusCode >= 400 {
			if cb != nil {
				cb.OnFailure()
			}

			if attempt < config.MaxRetries && resp.StatusCode >= 500 {
				lastErr = fmt.Errorf("HTTP error %d (attempt %d): %s", resp.StatusCode, attempt+1, resp.Status)
				time.Sleep(config.RetryDelay * time.Duration(attempt+1))
				continue
			}

			return response, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, resp.Status)
		}

		// Успішне виконання
		if cb != nil {
			cb.OnSuccess()
		}
		return response, nil
	}

	return nil, lastErr
}

// Функція виклику REST API (зворотна сумісність)
func CallRestAPI(req *http.Request) (
	response *simplejson.Json, err error) {
	return CallRestAPIWithConfig(req, nil)
}
