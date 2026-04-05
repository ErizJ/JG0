package validator

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"reflect"
	"strings"
	"sync"
)

type StructValidator interface {
	// ValidateStruct 结构体验证,并返回错误
	ValidateStruct(any) error
	// Engine 返回使用的验证器
	Engine() any
}

var Validator StructValidator = &defaultValidator{}

type defaultValidator struct {
	single   sync.Once
	validate *validator.Validate
}

func (d *defaultValidator) ValidateStruct(data any) error {

	if data == nil {
		return nil
	}
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		count := v.Len()
		errs := make(SliceValidationError, 0)
		for i := 0; i < count; i++ {
			if err := d.validateStruct(v.Index(i).Interface()); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			return nil
		}
		return errs
	case reflect.Ptr:
		return d.ValidateStruct(v.Elem().Interface())
	case reflect.Struct:
		return d.validateStruct(data)
	}
	return nil
}

func (d *defaultValidator) validateStruct(obj any) error {
	d.Engine()
	return d.validate.Struct(obj)
}

func (d *defaultValidator) Engine() any {
	d.single.Do(func() {
		d.validate = validator.New()
	})
	return d.validate
}

func (d *defaultValidator) lazyInit() {
	d.single.Do(func() {
		d.validate = validator.New()
	})
}

// StructValidate 通用结构体校验
func StructValidate(s StructValidator, data any) error {
	return s.ValidateStruct(data)
}

type SliceValidationError []error

func (err SliceValidationError) Error() string {
	n := len(err)
	switch n {
	case 0:
		return ""
	default:
		var b strings.Builder
		if err[0] != nil {
			_, err := fmt.Fprintf(&b, "[%d]: %s", 0, err[0].Error())
			if err != nil {
				return ""
			}
		}
		if n > 1 {
			for i := 1; i < n; i++ {
				if err[i] != nil {
					b.WriteString("\n")
					_, err := fmt.Fprintf(&b, "[%d]: %s", i, err[i].Error())
					if err != nil {
						return ""
					}
				}
			}
		}
		return b.String()
	}
}
