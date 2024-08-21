package rest_api

import (
	"fmt"
	"io"
	"net/http"

	"github.com/bitly/go-simplejson"
)

type (
	ApiBaseUrl string
	EndPoint   string
	HttpMethod string
)

// Функція виклику REST API
func CallRestAPI(req *http.Request) (
	response *simplejson.Json, err error) {
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
