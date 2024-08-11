package common

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/json"
	"github.com/fr0ster/turbo-restler/utils/signature"
)

type (
	ApiBaseUrl string
	EndPoint   string
	HttpMethod string
)

// Функція виклику REST API
func CallRestAPI(baseUrl ApiBaseUrl, method HttpMethod, params *simplejson.Json, endpoint EndPoint, sign signature.Sign) (response *simplejson.Json, err error) {
	var (
		signature  string
		parameters *simplejson.Json
	)

	if params == nil && sign == nil {
		err = fmt.Errorf("sign is required")
		return
	}

	// Створення HTTP клієнта
	client := &http.Client{}

	// Створення нового GET запиту
	req, err := http.NewRequest(string(method), string(baseUrl)+string(endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if params == nil {
		parameters = simplejson.New()
	} else {
		parameters = params
	}
	// Додавання параметрів до URL коли вони відсутні
	timestamp := int64(time.Nanosecond) * time.Now().UnixNano() / int64(time.Millisecond)
	parameters.Set("timestamp", strconv.FormatInt(timestamp, 10))
	// Створення підпису та додавання параметрів до URL
	req.URL.RawQuery, err = json.ConvertSimpleJSONToString(parameters)
	if err != nil {
		return nil, fmt.Errorf("error converting parameters to string: %v", err)
	}
	req.URL.RawQuery = fmt.Sprintf("%s&signature=%s", req.URL.RawQuery, sign.CreateSignature(signature))

	// Додавання заголовків
	req.Header.Set("X-MBX-APIKEY", sign.GetAPIKey())

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
	body, err := io.ReadAll(resp.Body)
	response, err = simplejson.NewJson(body)
	return
}
