package render

import (
	"errors"
	"fmt"
	"net/http"
)

type Redirect struct {
	StatusCode int
	Request    *http.Request
	Location   string
}

// WriteContentType (Redirect) don't write any ContentType.
func (r *Redirect) WriteContentType(w http.ResponseWriter) {}

func (r *Redirect) Render(w http.ResponseWriter) error {
	if (r.StatusCode < http.StatusMultipleChoices || r.StatusCode > http.StatusPermanentRedirect) && r.StatusCode != http.StatusCreated {
		return errors.New(fmt.Sprintf("cannot redirect with status code %d", r.StatusCode))
	}
	http.Redirect(w, r.Request, r.Location, r.StatusCode)
	return nil
}
