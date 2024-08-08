package json_test

import (
	"testing"

	"github.com/fr0ster/turbo-restler/utils/json"
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

	expected := "age=30&name=John"
	actual := params.Encode()
	assert.Equal(t, expected, actual)
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
