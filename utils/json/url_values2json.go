package json

import (
	encoding_json "encoding/json"
	"net/url"
	"sort"

	"github.com/bitly/go-simplejson"
)

// urlValuesToJSON перетворює url.Values в JSON строку
func urlValuesToJSON(params url.Values, sorted ...bool) (string, error) {
	result := make(map[string]interface{})

	// Додаємо ключі та значення до мапи
	for key, values := range params {
		if len(values) > 1 {
			result[key] = values
		} else {
			result[key] = values[0]
		}
	}

	// Якщо параметр sorted встановлений, сортуємо ключі
	if len(sorted) > 0 && sorted[0] {
		sortedKeys := make([]string, 0, len(result))
		for key := range result {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)

		sortedResult := make(map[string]interface{})
		for _, key := range sortedKeys {
			sortedResult[key] = result[key]
		}
		result = sortedResult
	}

	// Створюємо об'єкт JSON з мапи
	jsonBytes, err := encoding_json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// Функція для перетворення структури в JSON з відсортованими ключами
func UrlValuesToSortedJSON(params url.Values) (string, error) {
	return urlValuesToJSON(params, true)
}

// Функція для перетворення структури в JSON з несортованими ключами
func UrlValuesToJSON(params url.Values) (string, error) {
	return urlValuesToJSON(params, false)
}

// urlValuesToSimpleJSON перетворює url.Values в об'єкт simplejson.Json
func urlValuesToSimpleJSON(params url.Values, sorted ...bool) (*simplejson.Json, error) {
	result := make(map[string]interface{})

	// Додаємо ключі та значення до мапи
	for key, values := range params {
		if len(values) > 1 {
			result[key] = values
		} else {
			result[key] = values[0]
		}
	}

	// Якщо параметр sorted встановлений, сортуємо ключі
	if len(sorted) > 0 && sorted[0] {
		sortedKeys := make([]string, 0, len(result))
		for key := range result {
			sortedKeys = append(sortedKeys, key)
		}
		sort.Strings(sortedKeys)

		sortedResult := make(map[string]interface{})
		for _, key := range sortedKeys {
			sortedResult[key] = result[key]
		}
		result = sortedResult
	}

	// Створюємо об'єкт simplejson.Json з мапи
	jsonObj := simplejson.New()
	jsonObj.SetPath(nil, result)

	return jsonObj, nil
}

// Функція для перетворення структури в JSON з відсортованими ключами
func UrlValuesToSortedSimpleJSON(params url.Values) (string, error) {
	fieldMap, err := urlValuesToSimpleJSON(params, true)
	if err != nil {
		return "", err
	}

	// Перетворюємо мапу в JSON
	jsonData, err := encoding_json.Marshal(fieldMap)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// Функція для перетворення структури в JSON з несортованими ключами
func UrlValuesToSimpleJSON(params url.Values) (string, error) {
	fieldMap, err := urlValuesToSimpleJSON(params, false)
	if err != nil {
		return "", err
	}

	// Перетворюємо мапу в JSON
	jsonData, err := encoding_json.Marshal(fieldMap)
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}
