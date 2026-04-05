package render

import (
	"encoding/json"
	"net/http"
)

type JSON struct {
	Data any
}

func (J *JSON) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json;charset=utf-8")
}

func (J *JSON) Render(w http.ResponseWriter) error {
	jsonData, err := json.Marshal(J.Data)
	if err != nil {
		return err
	}
	_, err = w.Write(jsonData)
	return err
}
