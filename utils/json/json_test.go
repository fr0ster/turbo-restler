package json_test

import (
	encoding_json "encoding/json"
	"net/url"
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

func TestStructToSortedQueryMap(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	params, err := json.StructToSortedUrlValues(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated sorted query map
	assert.Equal(t, "John", params.Get("name"))
	assert.Equal(t, "age=30&name=John", params.Encode())
}

func TestStructToQueryMap(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	params, err := json.StructToUrlValues(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated query map
	assert.Equal(t, "John", params.Get("name"))
	assert.Equal(t, "age=30&name=John", params.Encode())
}

func TestStructToSortedJSON(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	jsonData, err := json.StructToSortedJSON(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated sorted JSON
	assert.Equal(t, `{"age":30,"name":"John"}`, string(jsonData))
}

func TestStructToJSON(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	jsonData, err := json.StructToJSON(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated JSON
	assert.Equal(t, `{"age":30,"name":"John"}`, string(jsonData))
}

func TestStructToSortedSimpleJSON(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	jsonData, err := json.StructToSortedSimpleJSON(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated sorted simple JSON
	assert.Equal(t, `{"age":30,"name":"John"}`, jsonData)
}

func TestIsEmptyValue(t *testing.T) {
	type Person struct {
		Name   string      `json:"name"`
		Age    int         `json:"age"`
		Params interface{} `json:"params"`
	}

	person := Person{Name: "John", Age: 30, Params: nil}
	params, err := json.StructToSortedUrlValues(person)
	assert.Nil(t, err)
	// Add test cases here to cover different scenarios for isEmptyValue function
	assert.Equal(t, "John", params.Get("name"))
	assert.Equal(t, "30", params.Get("age"))
	assert.Equal(t, "", params.Get("params"))
	assert.Equal(t, "age=30&name=John", params.Encode())
}

func TestRequest(t *testing.T) {
	type Request struct {
		ID     string      `json:"id"`
		Method string      `json:"method"`
		Params interface{} `json:"params"`
	}

	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	_, err := json.StructToSortedUrlValues(person)
	assert.Nil(t, err)

	request := Request{ID: "1", Method: "test", Params: nil}
	params, err := json.StructToSortedUrlValues(request)
	assert.Nil(t, err)
	jsonParams, err := json.StructToSortedJSON(request)
	assert.Nil(t, err)
	simpleJson, err := json.StructToSortedSimpleJSON(request)
	assert.Nil(t, err)
	standardJson, err := encoding_json.Marshal(request)
	assert.Nil(t, err)

	// Add assertions here to test the generated request
	assert.Equal(t, "1", params.Get("id"))
	assert.Equal(t, "test", params.Get("method"))
	assert.Equal(t, "", params.Get("params"))
	assert.Equal(t, "id=1&method=test", params.Encode())
	assert.Equal(t, "{\"id\":\"1\",\"method\":\"test\"}", string(jsonParams))
	assert.Equal(t, "id=1&method=test", params.Encode())
	assert.Equal(t, "{\"id\":\"1\",\"method\":\"test\"}", simpleJson)
	assert.Equal(t, "{\"id\":\"1\",\"method\":\"test\",\"params\":null}", string(standardJson))

	request = Request{ID: "1", Method: "test", Params: person}
	params, err = json.StructToSortedUrlValues(request)
	assert.Nil(t, err)
	jsonParams, err = json.StructToSortedJSON(request)
	assert.Nil(t, err)
	simpleJson, err = json.StructToSortedSimpleJSON(request)
	assert.Nil(t, err)
	standardJson, err = encoding_json.Marshal(request)
	assert.Nil(t, err)

	// Add assertions here to test the generated request
	assert.Equal(t, "1", params.Get("id"))
	assert.Equal(t, "test", params.Get("method"))
	assert.Equal(t, "{John 30}", params.Get("params"))
	assert.Equal(t, "{\"id\":\"1\",\"method\":\"test\",\"params\":{\"name\":\"John\",\"age\":30}}", string(jsonParams))
	assert.Equal(t, "id=1&method=test&params=%7BJohn+30%7D", params.Encode())
	assert.Equal(t, "{\"id\":\"1\",\"method\":\"test\",\"params\":{\"name\":\"John\",\"age\":30}}", simpleJson)
	assert.Equal(t, "{\"id\":\"1\",\"method\":\"test\",\"params\":{\"name\":\"John\",\"age\":30}}", string(standardJson))
}

func TestMarshal(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	jsonData, err := json.Marshal(person)
	assert.Nil(t, err)
	urlValues, err := json.StructToSortedUrlValues(person)
	assert.Nil(t, err)
	jsonUrlValues, err := json.UrlValuesToSortedJSON(urlValues)
	assert.Nil(t, err)
	byteUrlValues, err := json.UrlValuesToQueryString(urlValues)
	assert.Nil(t, err)
	jsonPerson, err := json.StructToSortedJSON(person)
	assert.Nil(t, err)
	strPerson, err := json.StructToQueryString(person)
	assert.Nil(t, err)
	bytePerson, err := encoding_json.Marshal(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated JSON
	assert.Equal(t, `{"name":"John","age":30}`, string(jsonData))
	assert.Equal(t, `{"age":30,"name":"John"}`, string(jsonPerson))
	assert.Equal(t, "age=30&name=John", strPerson)
	assert.Equal(t, `{"name":"John","age":30}`, string(bytePerson))
	assert.Equal(t, `{"age":"30","name":"John"}`, string(jsonUrlValues))
	assert.Equal(t, "age=30&name=John", byteUrlValues)
}

func StructToSortedQueryByteArr(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	person := Person{Name: "John", Age: 30}
	byteArr, err := json.StructToSortedQueryByteArr(person)
	assert.Nil(t, err)

	// Add assertions here to test the generated sorted query byte array
	assert.Equal(t, "age=30&name=John", string(byteArr))
}

func TestUrlValuesRequest(t *testing.T) {
	type Request struct {
		ID     string     `json:"id"`
		Method string     `json:"method"`
		Params url.Values `json:"params"`
	}

	request := Request{ID: "1", Method: "test", Params: nil}
	params, err := json.StructToSortedUrlValues(request)
	assert.Nil(t, err)
	byteRequest, err := encoding_json.Marshal(request)
	assert.Nil(t, err)
	strRequest, err := json.StructToSortedQueryByteArr(request)
	assert.Nil(t, err)

	// Add assertions here to test the generated request
	assert.Equal(t, "1", params.Get("id"))
	assert.Equal(t, "test", params.Get("method"))
	assert.Equal(t, "", params.Get("params"))
	assert.Equal(t, "id=1&method=test", params.Encode())
	assert.Equal(t, `{"id":"1","method":"test","params":null}`, string(byteRequest))
	assert.Equal(t, `{"id":"1","method":"test"}`, string(strRequest))
}
