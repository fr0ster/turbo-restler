package common

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	signature "github.com/fr0ster/turbo-restler/utils/signature"
)

// Функція виклику REST API
func CallRestAPI(baseUrl, method string, params url.Values, endpoint string, sign signature.Sign) (body []byte, err error) {
	var (
		signature   = url.Values{}
		queryString string
	)

	if params != nil && sign == nil {
		err = fmt.Errorf("sign is required")
		return
	}

	// Створення HTTP клієнта
	client := &http.Client{}

	// Створення нового GET запиту
	req, err := http.NewRequest(method, baseUrl+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if params != nil {
		timestamp := int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
		params.Set("timestamp", strconv.FormatInt(timestamp, 10))
		// Створення підпису
		signature.Set("signature", sign.CreateSignature(params.Encode()))
		// Додавання параметрів до URL
		req.URL.RawQuery = fmt.Sprintf("%s&%s", queryString, signature.Encode())

		// Додавання заголовків
		req.Header.Set("X-MBX-APIKEY", sign.GetAPIKey())
	} else if sign != nil {
		// Додавання заголовків
		req.Header.Set("X-MBX-APIKEY", sign.GetAPIKey())
	}

	// Виконання запиту
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Перевірка статусу відповіді
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error: received non-200 response code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Читання тіла відповіді
	body, err = io.ReadAll(resp.Body)
	return
}
