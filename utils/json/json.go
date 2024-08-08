package json

import (
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
} // Функція для перетворення структури в map[string]string формату key=value для подальшого використання в url.Values
func StructToUrlValues(data interface{}, sorted ...bool) (params url.Values, err error) {
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
	fieldMap := url.Values{}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Пропускаємо пусті поля
		if isEmptyValue(field) {
			continue
		}

		// Додаємо поле у форматі key=value
		fieldMap.Add(fieldType.Tag.Get("json"), fmt.Sprintf("%v", field.Interface()))
	}

	if sorted[0] {
		// Сортуємо ключі
		keys := make([]string, 0, len(fieldMap))
		for k := range fieldMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Додаємо відсортовані ключі до url.Values
		for _, k := range keys {
			params.Add(k, fieldMap.Get(k))
		}
	} else {
		// Додаємо несортовані ключі до url.Values
		params = url.Values(fieldMap)
	}

	return params, nil
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
