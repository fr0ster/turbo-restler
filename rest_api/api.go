package common

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	signature "github.com/fr0ster/go-trading-utils/low_level/utils/signature"
)

// Функція для отримання масиву всіх спотових ордерів
func CallAPI(baseUrl, method string, params url.Values, endpoint string, sign signature.Sign) (body []byte, err error) {
	var (
		v           url.Values
		queryString string
	)

	if params != nil && sign == nil {
		return nil, fmt.Errorf("sign is required")
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
		queryString = params.Encode()
		v = url.Values{}
		signature := sign.CreateSignature(queryString)
		v.Set("signature", signature)
		// Додавання параметрів до URL
		req.URL.RawQuery = fmt.Sprintf("%s&%s", queryString, v.Encode())

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
