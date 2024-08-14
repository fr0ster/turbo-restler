package rest_api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-signer/signature"
)

type (
	ApiBaseUrl string
	EndPoint   string
	HttpMethod string
)

// Функція виклику REST API
func CallRestAPI(baseUrl ApiBaseUrl, method HttpMethod, params *simplejson.Json, endpoint EndPoint, sign signature.Sign) (
	response *simplejson.Json, err error) {
	// Construct the URL
	apiUrl := string(baseUrl) + string(endpoint)

	// Prepare the query parameters
	v := url.Values{}
	if params != nil {
		for key, value := range params.MustMap() {
			v.Set(key, fmt.Sprintf("%v", value))
		}
	}

	// Add timestamp to the query parameters
	timestamp := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	v.Set("timestamp", timestamp)

	// Create the signature
	queryString := v.Encode()
	signature := sign.CreateSignature(queryString)
	v.Set("signature", signature)

	// Create the full URL with query parameters
	fullUrl := apiUrl + "?" + v.Encode()

	// Prepare the HTTP request
	req, err := http.NewRequest(string(method), fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add headers
	req.Header.Set("X-MBX-APIKEY", sign.GetAPIKey())

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	// Parse the response body as JSON
	response, err = simplejson.NewJson(body)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON response: %v", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return response, nil
}
