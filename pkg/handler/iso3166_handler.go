package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	pkgiso3166 "example.com/registryquery/pkg/iso3166"
)

type ISO3166Handler struct {
	ISOAlpha2Map map[string][]pkgiso3166.ISOCountryCodeRecord
}

const (
	paramKeyAlpha2 = "alpha2"
)

type DataResponse struct {
	Data interface{} `json:"data"`
}

func (h *ISO3166Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if alpha2 := r.URL.Query().Get(paramKeyAlpha2); alpha2 != "" {
		if records, ok := h.ISOAlpha2Map[strings.ToUpper(alpha2)]; ok {
			json.NewEncoder(w).Encode(DataResponse{Data: records})
			return
		}
		if records, ok := h.ISOAlpha2Map[strings.ToLower(alpha2)]; ok {
			json.NewEncoder(w).Encode(DataResponse{Data: records})
			return
		}
		errMsg, _ := json.Marshal(ErrResponse{Err: "No records found for alpha2: " + alpha2})
		http.Error(w, string(errMsg), http.StatusNotFound)
	} else {
		errMsg, _ := json.Marshal(ErrResponse{Err: "Missing " + paramKeyAlpha2 + " parameter"})
		http.Error(w, string(errMsg), http.StatusBadRequest)
		return
	}
}
