package common

import (
	"errors"
	"fmt"
	"io"
	"net/http"

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
	// Construct the URL
	apiUrl := string(baseUrl) + string(endpoint)

	// Prepare the query string
	var queryString string
	if params != nil {
		queryString, err = json.ConvertSimpleJSONToString(params)
		if err != nil {
			err = fmt.Errorf("error encoding params: %v", err)
			return
		}
	}

	// Sign the query string
	signature := sign.CreateSignature(queryString)

	// Add the signature to the query string
	if queryString != "" {
		queryString += "&signature=" + signature
	} else {
		queryString = "signature=" + signature
	}

	// Create the full URL with query parameters
	fullUrl := apiUrl + "?" + queryString

	// Prepare the HTTP request
	req, err := http.NewRequest(string(method), fullUrl, nil)
	if err != nil {
		return nil, err
	}

	// Execute the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse the response body as JSON
	response, err = simplejson.NewJson(body)
	if err != nil {
		return nil, err
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP error: " + resp.Status)
	}

	return response, nil
}
