package rest_api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bitly/go-simplejson"
)

type (
	ApiBaseUrl string
	EndPoint   string
	HttpMethod string
)

// Функція виклику REST API
func CallRestAPI(baseUrl ApiBaseUrl, method HttpMethod, params *simplejson.Json, endpoint EndPoint) (
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

	// Create the full URL with query parameters
	fullUrl := apiUrl + "?" + v.Encode()

	// Prepare the HTTP request
	req, err := http.NewRequest(string(method), fullUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	if params != nil && params.Get("apiKey").MustString() != "" {
		// Add headers
		req.Header.Set("X-MBX-APIKEY", params.Get("apiKey").MustString())
	}

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
		response = simplejson.New()
		response.Set("message", string(body))
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return response, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return response, nil
}
