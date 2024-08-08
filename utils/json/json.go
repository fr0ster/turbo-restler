package json

import (
	encoding_json "encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"

	"github.com/bitly/go-simplejson"
)

func NewJSON(data []byte) (j *simplejson.Json, err error) {
	j, err = simplejson.NewJson(data)
	if err != nil {
		return nil, err
	}
	return j, nil
}

// Функція для перетворення структури в строку формату key=value&key=value&... з несортованими ключами
func StructToQueryString(data interface{}) (string, error) {
	params, err := structToUrlValues(data, false)
	if err != nil {
		return "", err
	}
	return params.Encode(), nil
}

// Функція для перетворення структури в строку формату key=value&key=value&... з відсортованими ключами
func StructToSortedQueryByteArr(data interface{}) ([]byte, error) {
	params, err := StructToSortedJSON(data)
	if err != nil {
		return nil, err
	}
	return []byte(params), nil
}

// Функція для перетворення структури в строку формату key=value&key=value&... з несортованими ключами
func StructToQueryByteArr(data interface{}) ([]byte, error) {
	params, err := StructToJSON(data)
	if err != nil {
		return nil, err
	}
	return []byte(params), nil
}

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

// Функція для перетворення структури в JSON
func structToJSON(data interface{}, sorted ...bool) (string, error) {
	fieldMap, err := structToQueryMap(data, sorted...)
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
func StructToSortedJSON(data interface{}) (string, error) {
	return structToJSON(data, true)
}

// Функція для перетворення структури в JSON з несортованими ключами
func StructToJSON(data interface{}) (string, error) {
	return structToJSON(data, false)
}

// Функція для перетворення структури в JSON, ігноруючи пусті поля та з можливістю сортування ключів
func structToSimpleJSON(data interface{}, sorted ...bool) (*simplejson.Json, error) {
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
func StructToSortedSimpleJSON(data interface{}) (string, error) {
	fieldMap, err := structToSimpleJSON(data, true)
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
func StructToSimpleJSON(data interface{}) (string, error) {
	fieldMap, err := structToSimpleJSON(data, false)
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

func Marshal(v interface{}) ([]byte, error) {
	return encoding_json.Marshal(v)
}

func Unmarshal(data []byte, v interface{}) error {
	return encoding_json.Unmarshal(data, v)
}

// Функція для перевірки, чи є значення пустим
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
