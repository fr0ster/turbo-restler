package rest_api_test

import (
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/bitly/go-simplejson"

	rest_api "github.com/fr0ster/turbo-restler/rest_api"
	signature "github.com/fr0ster/turbo-restler/utils/signature"

	"github.com/stretchr/testify/assert"
)

type Signature interface {
	CreateSignature(queryString string) string
	GetAPIKey() string
}

type Server struct {
	sign Signature
}

func NewServer(sign Signature) *Server {
	return &Server{
		sign: sign,
	}
}

func (s *Server) validateHMACSignature(message, signature string) bool {
	expectedSignature := s.sign.CreateSignature(message)
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

func (s *Server) noParamsHandler(w http.ResponseWriter, r *http.Request) {
	timestamp := r.URL.Query().Get("timestamp")
	signature := r.URL.Query().Get("signature")

	if timestamp == "" || signature == "" {
		http.Error(w, "Missing timestamp or signature", http.StatusBadRequest)
		return
	}

	message := "timestamp=" + timestamp
	if !s.validateHMACSignature(message, signature) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	response := map[string]string{"message": "No params endpoint"}
	json.NewEncoder(w).Encode(response)
}

func (s *Server) paramsHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	param1 := query.Get("param1")
	param2 := query.Get("param2")
	param3 := query.Get("param3")
	param4 := query.Get("param4")
	param5 := query.Get("param5")
	timestamp := query.Get("timestamp")
	signature := query.Get("signature")

	if param1 == "" || param2 == "" || param3 == "" || param4 == "" || param5 == "" || timestamp == "" || signature == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	// Validate timestamp
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		http.Error(w, "Invalid timestamp", http.StatusBadRequest)
		return
	}
	currentTime := time.Now().UnixNano() / int64(time.Millisecond)
	if currentTime-ts > 1000 { // Allow a maximum of 60 seconds difference
		http.Error(w, "Timestamp too old", http.StatusUnauthorized)
		return
	}

	// Validate signature
	message := fmt.Sprintf("param1=%s&param2=%s&param3=%s&param4=%s&param5=%s&timestamp=%s", param1, param2, param3, param4, param5, timestamp)
	if !s.validateHMACSignature(message, signature) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	response := map[string]string{"message": "Params endpoint"}
	json.NewEncoder(w).Encode(response)
}

func (s *Server) Start() {
	http.HandleFunc("/no-params", s.noParamsHandler)
	http.HandleFunc("/params", s.paramsHandler)
	http.ListenAndServe(":8080", nil)
}

func TestCallRestAPI(t *testing.T) {
	apiKey := "your_api_key"
	secretKey := "your_secret_key"
	sign := signature.NewSignHMAC(signature.PublicKey(apiKey), signature.SecretKey(secretKey))
	server := NewServer(sign)
	go server.Start()
	response1, err := rest_api.CallRestAPI("http://localhost:8080", "GET", nil, "/no-params", sign)
	assert.Nil(t, err)
	assert.Equal(t, "No params endpoint", response1.Get("message").MustString())
	params := simplejson.New()
	params.Set("param1", "value1")
	params.Set("param2", "value2")
	params.Set("param3", "value3")
	params.Set("param4", "value4")
	params.Set("param5", "value5")
	response2, err := rest_api.CallRestAPI("http://localhost:8080", "GET", params, "/params", sign)
	assert.Nil(t, err)
	if response2 != nil {
		assert.Equal(t, "Params endpoint", response2.Get("message").MustString())
	}
}
