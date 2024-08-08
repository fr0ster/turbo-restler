package json

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/bitly/go-simplejson"
)

type ParameterMap map[string]string

func NewJSON(data []byte) (j *simplejson.Json, err error) {
	j, err = simplejson.NewJson(data)
	if err != nil {
		return nil, err
	}
	return j, nil
}

// StructToUrlValues конвертує структуру або вказівник на структуру в url.Values
func StructToUrlValues(data interface{}) (params url.Values, err error) {
	// Перевірка, чи є вхідні дані структурою або вказівником на структуру
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input data is not a struct or pointer to a struct")
	}

	params = url.Values{}
	t := v.Type()

	// Додати непусті поля структури до url.Values
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		if !value.IsZero() {
			// Використати тег JSON для імені поля
			tag := field.Tag.Get("json")
			if tag == "" || tag == "-" {
				tag = field.Name
			} else {
				tag = strings.Split(tag, ",")[0]
			}
			params.Set(tag, fmt.Sprintf("%v", value))
		}
	}

	return params, nil
}

// StructToParameterMap конвертує структуру або вказівник на структуру в ParameterMap
func StructToParameterMap(data interface{}) (params ParameterMap, err error) {
	// Перевірка, чи є вхідні дані структурою або вказівником на структуру
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input data is not a struct or pointer to a struct")
	}

	params = make(ParameterMap)
	t := v.Type()

	// Додати непусті поля структури до url.Values
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		if !value.IsZero() {
			// Використати тег JSON для імені поля
			tag := field.Tag.Get("json")
			if tag == "" || tag == "-" {
				tag = field.Name
			} else {
				tag = strings.Split(tag, ",")[0]
			}
			params[tag] = fmt.Sprintf("%v", value)
		}
	}

	return params, nil
}
