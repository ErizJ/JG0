package render

import (
	"encoding/xml"
	"net/http"
)

type XML struct {
	Data any
}

func (X *XML) WriteContentType(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml;charset=utf-8")
}

func (X *XML) Render(w http.ResponseWriter) error {
	//xmlData, err := xml.Marshal(data)
	//if err != nil {
	//	return err
	//}
	//_, err = c.W.Write(xmlData)
	return xml.NewEncoder(w).Encode(X.Data)
}
