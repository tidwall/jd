package gjson

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func analyzeValue(v interface{}) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return &json.InvalidUnmarshalError{reflect.TypeOf(v)}
	}
	fmt.Printf("%+v\n", rv)
	return nil
}
