package json

import (
	encoding_json "encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/bitly/go-simplejson"
)

// Функція для перетворення структури в JSON
func mapToJSON(data interface{}, sorted ...bool) (string, error) {
	fieldMap, err := structToUrlValues(data, sorted...)
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

// Функція для перетворення структури в JSON з відсортованими ключами
func MapToSortedJSON(data map[string]interface{}) (string, error) {
	return mapToJSON(data, true)
}

// Функція для перетворення структури в JSON з несортованими ключами
func MapToJSON(data map[string]interface{}) (string, error) {
	return mapToJSON(data, false)
}

// Функція для перетворення структури в JSON, ігноруючи пусті поля та з можливістю сортування ключів
func mapToSimpleJSON(data map[string]interface{}, sorted ...bool) (*simplejson.Json, error) {
	if len(sorted) == 0 {
		sorted = append(sorted, true)
	}
	v := reflect.ValueOf(data)

	// Перевірка, чи є вхідний параметр структурою
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got %s", v.Kind())
	}

	t := v.Type()
	jsonObj := simplejson.New()
	fieldMap := make(map[string]interface{})

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Пропускаємо пусті поля
		if isEmptyValue(field) {
			continue
		}

		// Додаємо поле у форматі key=value
		fieldMap[fieldType.Tag.Get("json")] = field.Interface()
	}

	// Сортуємо ключі, якщо потрібно
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	if sorted[0] {
		sort.Strings(keys)
	}

	// Додаємо відсортовані ключі до JSON об'єкта
	for _, k := range keys {
		jsonObj.Set(k, fieldMap[k])
	}

	return jsonObj, nil
}

// Функція для перетворення структури в JSON з відсортованими ключами
func MapToSortedSimpleJSON(data map[string]interface{}) (string, error) {
	fieldMap, err := mapToSimpleJSON(data, true)
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
func MapToSimpleJSON(data map[string]interface{}) (string, error) {
	fieldMap, err := mapToSimpleJSON(data, false)
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
