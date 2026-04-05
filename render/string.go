package render

import (
	"fmt"
	"github.com/ErizJ/JG0/internal/bytesconv"
	"net/http"
)

type String struct {
	Format string
	Values []any
}

func (s *String) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain;charset=utf-8")
}

func (s *String) Render(w http.ResponseWriter) error {
	var err error
	if len(s.Values) > 0 {
		_, err = fmt.Fprintf(w, s.Format, s.Values...)
	} else {
		_, err = w.Write(bytesconv.StringToBytes(s.Format))
	}
	return err
}
