package binding

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ErizJ/JG0/validator"
	"net/http"
	"reflect"
)

type jsonBinding struct {
	DisallowUnknownFields bool
	IsValidate            bool
	StructValidator       validator.StructValidator
}

func (j *jsonBinding) Name() string {
	return "json"
}

func (j *jsonBinding) Bind(r *http.Request, data any) error {
	body := r.Body
	if r == nil || body == nil {
		return errors.New("invalid json request")
	}
	decoder := json.NewDecoder(body)
	if j.DisallowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	if j.IsValidate {
		err := j.validateParam(data, decoder)
		if err != nil {
			return err
		}
	} else {
		err := decoder.Decode(data)
		if err != nil {
			return err
		}
	}
	//github validator begin
	if j.StructValidator != nil {
		return validator.StructValidate(j.StructValidator, data)
	}
	return nil
}

func (j *jsonBinding) validateParam(data any, decoder *json.Decoder) error {
	valueOf := reflect.ValueOf(data)
	if valueOf.Kind() != reflect.Pointer {
		return errors.New("data argument must have a pointer type")
	}
	elem := valueOf.Elem().Interface()
	value := reflect.ValueOf(elem)
	switch value.Kind() {
	case reflect.Struct:
		return checkParam(value, data, decoder)
	case reflect.Array, reflect.Slice:
		ele := value.Type().Elem()
		if ele.Kind() == reflect.Ptr {
			ele = ele.Elem()
		}
		if ele.Kind() == reflect.Struct {
			return checkSliceParam(ele, data, decoder)
		}
	default:
		return decoder.Decode(data)
	}
	return nil
}

func checkSliceParam(elem reflect.Type, data any, decoder *json.Decoder) error {
	mapVal := make([]map[string]interface{}, 0)
	//解析为map
	err := decoder.Decode(&mapVal)
	if err != nil {
		return err
	}
	for i := 0; i < elem.NumField(); i++ {
		field := elem.Field(i)
		name := field.Name
		jsonName := field.Tag.Get("json")
		requiredTag := field.Tag.Get("required")
		if jsonName == "" {
			jsonName = name
		}
		for i, v := range mapVal {
			if v[jsonName] == nil && requiredTag != "" {
				return errors.New(fmt.Sprintf("row[%d] field [%s] is not exist", i+1, jsonName))
			}
		}

	}
	b, _ := json.Marshal(mapVal)
	return json.Unmarshal(b, data)
}

func checkParam(value reflect.Value, data any, decoder *json.Decoder) error {
	mapVal := make(map[string]interface{})
	//解析为map
	err := decoder.Decode(&mapVal)
	if err != nil {
		return err
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		name := field.Name
		jsonName := field.Tag.Get("json")
		requiredTag := field.Tag.Get("required")
		if jsonName == "" {
			jsonName = name
		}
		if mapVal[jsonName] == nil && requiredTag != "" {
			return errors.New(fmt.Sprintf("field [%s] is not exist", jsonName))
		}
	}
	b, _ := json.Marshal(mapVal)
	return json.Unmarshal(b, data)
}
