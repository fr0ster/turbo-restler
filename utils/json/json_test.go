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

	// Add assertions here to test the created JSON object
	assert.Equal(t, "John", j.Get("name").MustString())
}

func TestStructToQueryString(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	queryString, err := json.StructToQueryString(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated query string
	assert.Equal(t, "age=30&name=John", queryString)
}

func TestStructToQueryMap(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	params, err := json.StructToQueryMap(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated query map
	assert.Equal(t, "John", params.Get("name"))
	queryString := params.Encode()
	assert.Equal(t, "age=30&name=John", queryString)
}

func TestStructToSortedQueryMap(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	params, err := json.StructToSortedQueryMap(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated sorted query map
	assert.Equal(t, "John", params.Get("name"))
	queryString := params.Encode()
	assert.Equal(t, "age=30&name=John", queryString)
}
