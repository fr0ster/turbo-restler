package json

import (
	"fmt"
	"net/url"

	"github.com/bitly/go-simplejson"
)

func ConvertSimpleJSONToString(js *simplejson.Json) (string, error) {
	// Парсинг JSON строки
	values := url.Values{}
	for key, value := range js.MustMap() {
		values.Set(key, fmt.Sprintf("%v", value))
	}

	return values.Encode(), nil
}
