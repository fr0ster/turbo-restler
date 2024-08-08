package json

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

func UrlValuesToString(params url.Values) (string, error) {
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

func UrlValuesToBytes(params url.Values) ([]byte, error) {
	str, err := UrlValuesToString(params)
	if err != nil {
		return nil, err
	}
	return []byte(str), nil
}
