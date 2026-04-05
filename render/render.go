package render

import "net/http"

type Render interface {
	WriteContentType(w http.ResponseWriter)
	Render(w http.ResponseWriter) error
}
