package json

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// MapToQueryString перетворює мапу в рядок у форматі query string
func MapToQueryString(data map[string]interface{}) (string, error) {
	if data == nil {
		return "", fmt.Errorf("input map cannot be nil")
	}

	var queryString []string
	keys := make([]string, 0, len(data))

	for k := range data {
		keys = append(keys, k)
	}

	// Сортуємо ключі для стабільного порядку
	sort.Strings(keys)

	for _, k := range keys {
		v := data[k]
		strValue := fmt.Sprintf("%v", v)
		escapedKey := url.QueryEscape(k)
		escapedValue := url.QueryEscape(strValue)
		queryString = append(queryString, fmt.Sprintf("%s=%s", escapedKey, escapedValue))
	}

	return strings.Join(queryString, "&"), nil
}

// Функція для перетворення структури в строку формату key=value&key=value&... з несортованими ключами
func MapToQueryByteArr(data map[string]interface{}) ([]byte, error) {
	str, err := MapToQueryString(data)
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}

func UrlValuesToQueryString(params url.Values) (string, error) {
	if params == nil {
		return "", fmt.Errorf("input map cannot be nil")
	}

	var queryString []string
	keys := make([]string, 0, len(params))

	for k := range params {
		keys = append(keys, k)
	}

	// Сортуємо ключі для стабільного порядку
	sort.Strings(keys)

	for _, k := range keys {
		v := params[k]
		strValue := v[0]
		escapedKey := url.QueryEscape(k)
		escapedValue := url.QueryEscape(strValue)
		queryString = append(queryString, fmt.Sprintf("%s=%s", escapedKey, escapedValue))
	}

	return strings.Join(queryString, "&"), nil
}

func UrlValuesToByteArr(params url.Values) ([]byte, error) {
	str, err := UrlValuesToQueryString(params)
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}
