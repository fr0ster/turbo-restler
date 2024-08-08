package json_test

import (
	"net/url"
	"testing"

	encoding_json "encoding/json"

	"github.com/fr0ster/turbo-restler/utils/json"
	web_api "github.com/fr0ster/turbo-restler/web_api"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewJSON(t *testing.T) {
	data := []byte(`{"name": "John", "age": 30}`)
	j, err := json.NewJSON(data)
	assert.Nil(t, err)
	name := j.Get("name").MustString()
	assert.Equal(t, "John", name)

	age := j.Get("age").MustInt()
	assert.Equal(t, 30, age)
}

func TestStructToUrlValues(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	p := Person{
		Name: "John",
		Age:  30,
	}

	params, err := json.StructToUrlValues(p)
	assert.Nil(t, err)
	assert.Equal(t, "John", params.Get("name"))
	ptr := &p
	params, err = json.StructToUrlValues(ptr)
	assert.Nil(t, err)
	assert.Equal(t, "John", params.Get("name"))

	expected := "age=30&name=John"
	actual := params.Encode()
	assert.Equal(t, expected, actual)

	jsonPerson, err := encoding_json.Marshal(json.ConvertUrlValuesToMap(params))
	assert.Nil(t, err)
	assert.Equal(t, `{"age":"30","name":"John"}`, string(jsonPerson))

	jsonPerson, err = encoding_json.Marshal(p)
	assert.Nil(t, err)
	assert.Equal(t, `{"name":"John","age":30}`, string(jsonPerson))
}

func TestStructToParameterMap(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	p := Person{
		Name: "John",
		Age:  30,
	}

	params, err := json.StructToParameterMap(p)
	assert.Nil(t, err)

	expected := json.ParameterMap{
		"name": "John",
		"age":  "30",
	}
	assert.Equal(t, expected, params)

	ptr := &p
	params, err = json.StructToParameterMap(ptr)
	assert.Nil(t, err)

	assert.Equal(t, expected, params)
}

func TestConvertUrlValuesToMap(t *testing.T) {
	params := url.Values{}
	params.Set("name", "John")
	params.Set("age", "30")

	request := web_api.Request{
		ID:     uuid.New().String(),
		Method: "test.test",
		Params: json.ConvertUrlValuesToMap(params),
	}
	bytes, err := encoding_json.Marshal(request)
	assert.Nil(t, err)
	assert.Equal(t, `{"id":"`+request.ID+`","method":"`+request.Method+`","params":{"age":"30","name":"John"}}`, string(bytes))
	bytes, err = encoding_json.Marshal(json.ConvertUrlValuesToMap(params))
	assert.Nil(t, err)
	assert.Equal(t, `{"age":"30","name":"John"}`, string(bytes))
}

func TestIsEmptyValue(t *testing.T) {
	type Person struct {
		Name   string      `json:"name"`
		Age    int         `json:"age"`
		Params interface{} `json:"params"`
	}

	person := Person{Name: "John", Age: 30, Params: nil}
	params, err := json.StructToUrlValues(person)
	assert.Nil(t, err)
	// Add test cases here to cover different scenarios for isEmptyValue function
	assert.Equal(t, "John", params.Get("name"))
	assert.Equal(t, "30", params.Get("age"))
	assert.Equal(t, "", params.Get("params"))
	assert.Equal(t, "age=30&name=John", params.Encode())
}

func TestStructToUrlValuesWithTags(t *testing.T) {
	mapper := make(map[string]interface{})
	mapper["name"] = "John"
	mapper["age"] = "30.2"
	mapper["timestamp"] = 1111111111
	request := web_api.Request{
		ID:     "121212",
		Method: "method.method",
		Params: mapper,
	}
	requestMap, err := encoding_json.Marshal(request)
	assert.Nil(t, err)
	assert.Equal(t, `{"id":"121212","method":"method.method","params":{"age":"30.2","name":"John","timestamp":1111111111}}`, string(requestMap))
}
