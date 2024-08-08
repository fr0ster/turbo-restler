package json

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"
)

// Функція для перетворення структури в мапу, ігноруючи пусті поля та з можливістю сортування ключів
func structToQueryMap(data interface{}, sorted ...bool) (params map[string]interface{}, err error) {
	if len(sorted) == 0 {
		sorted = append(sorted, true)
	}
	v := reflect.ValueOf(data)

	// Перевірка, чи є вхідний параметр структурою
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got %s", v.Kind())
	}

	t := v.Type()
	params = make(map[string]interface{})
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

	// Додаємо відсортовані ключі до мапи
	for _, k := range keys {
		params[k] = fieldMap[k]
	}

	return params, nil
}

// Функція для перетворення структури в мапу з відсортованими ключами
func StructToSortedQueryMap(data interface{}) (map[string]interface{}, error) {
	return structToQueryMap(data, true)
}

// Функція для перетворення структури в мапу з несортованими ключами
func StructToQueryMap(data interface{}) (map[string]interface{}, error) {
	return structToQueryMap(data, false)
}

// Функція для перетворення структури в map[string]string формату key=value для подальшого використання в url.Values
func structToUrlValues(data interface{}, sorted ...bool) (params url.Values, err error) {
	if len(sorted) == 0 {
		sorted = append(sorted, true)
	}
	params = url.Values{}
	v := reflect.ValueOf(data)

	// Перевірка, чи є вхідний параметр структурою
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got %s", v.Kind())
	}

	t := v.Type()
	fieldMap := make(map[string]string)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Пропускаємо пусті поля
		if isEmptyValue(field) {
			continue
		}

		// Додаємо поле у форматі key=value
		fieldMap[fieldType.Tag.Get("json")] = fmt.Sprintf("%v", field.Interface())
	}

	// Сортуємо ключі
	keys := make([]string, 0, len(fieldMap))
	for k := range fieldMap {
		keys = append(keys, k)
	}
	if sorted[0] {
		sort.Strings(keys)
	}

	// Додаємо відсортовані ключі до url.Values
	for _, k := range keys {
		params.Add(k, fieldMap[k])
	}

	return params, nil
}

// Функція для перетворення структури в map[string]string формату key=value з відсортованими ключами
func StructToSortedUrlValues(data interface{}) (params url.Values, err error) {
	return structToUrlValues(data, true)
}

// Функція для перетворення структури в map[string]string формату key=value з несортованими ключами
func StructToUrlValues(data interface{}) (params url.Values, err error) {
	return structToUrlValues(data, false)
}
