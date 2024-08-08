package json

import (
	encoding_json "encoding/json"
	"reflect"

	"github.com/bitly/go-simplejson"
)

func NewJSON(data []byte) (j *simplejson.Json, err error) {
	j, err = simplejson.NewJson(data)
	if err != nil {
		return nil, err
	}
	return j, nil
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
