package render

import (
	"github.com/ErizJ/JG0/internal/bytesconv"
	"html/template"
	"net/http"
)

type HTMLRender struct {
	Template *template.Template
}

type HTML struct {
	Name       string
	Data       any
	Template   *template.Template
	IsTemplate bool
}

func (H *HTML) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html;charset=utf-8")
}

func (H *HTML) Render(w http.ResponseWriter) error {
	if !H.IsTemplate {
		_, err := w.Write(bytesconv.StringToBytes(H.Data.(string)))
		return err
	}
	return H.Template.ExecuteTemplate(w, H.Name, H.Data)
}
