package json

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
	params, err := structToJSON(data)
	if err != nil {
		return nil, err
	}
	return []byte(params), nil
}
