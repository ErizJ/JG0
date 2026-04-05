package binding

import (
	"encoding/xml"
	"errors"
	"github.com/ErizJ/JG0/validator"
	"net/http"
)

type xmlBinding struct {
	StructValidator validator.StructValidator
}

func (x *xmlBinding) Name() string {
	return "xml"
}

func (x *xmlBinding) Bind(r *http.Request, data any) error {
	body := r.Body
	if r == nil || body == nil {
		return errors.New("invalid xml request")
	}
	decoder := xml.NewDecoder(r.Body)
	if err := decoder.Decode(data); err != nil {
		return err
	}
	if x.StructValidator != nil {
		return validator.StructValidate(x.StructValidator, data)
	}
	return nil
}
