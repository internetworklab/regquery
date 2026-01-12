package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

type ErrResponse struct {
	Err string `json:"err"`
}

type Formattable interface {
	String() string
}

func encResp(req *http.Request, w http.ResponseWriter, resp interface{}) error {
	accept := req.Header.Get("Accept")
	if accept == "" {
		accept = req.URL.Query().Get("accept")
	}
	if accept == "" {
		accept = "application/json"
	}

	if strings.HasPrefix(accept, "application/json") {
		return json.NewEncoder(w).Encode(resp)
	}
	if strings.HasPrefix(accept, "text/plain") {
		if s, ok := resp.(string); ok {
			fmt.Fprintf(w, "%s", s)
			return nil
		}
		if f, ok := resp.(Formattable); ok {
			fmt.Fprintf(w, "%s", f.String())
			return nil
		}
		return json.NewEncoder(w).Encode(resp)
	}
	if strings.HasPrefix(accept, "application/yaml") {
		return yaml.NewEncoder(w).Encode(resp)
	}
	return json.NewEncoder(w).Encode(resp)
}
