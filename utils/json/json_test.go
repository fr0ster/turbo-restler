package json_test

import (
	"testing"

	"github.com/bitly/go-simplejson"
	"github.com/fr0ster/turbo-restler/utils/json"
	"github.com/stretchr/testify/assert"
)

func TestConvertSimpleJSONToString(t *testing.T) {
	func() {
		js := simplejson.New()
		js.Set("name", "John")
		js.Set("age", 30)
		expected := `age=30&name=John`
		result, err := json.ConvertSimpleJSONToString(js)
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	}()
	func() {
		js := simplejson.New()
		js.Set("key", "value")
		expected := `key=value`
		result, err := json.ConvertSimpleJSONToString(js)
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	}()
	func() {
		js := simplejson.New()
		js.Set("array", []int{1, 2, 3})
		expected := `array=%5B1+2+3%5D`
		result, err := json.ConvertSimpleJSONToString(js)
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	}()
	func() {
		js := simplejson.New()
		js.Set("nested", simplejson.New())
		expected := `nested=%26%7Bmap%5B%5D%7D`
		result, err := json.ConvertSimpleJSONToString(js)
		assert.Nil(t, err)
		assert.Equal(t, expected, result)
	}()
}
