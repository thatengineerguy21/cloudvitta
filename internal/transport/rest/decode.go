package rest

import (
	"encoding/json"
	"net/http"
)

// MaxRequestBodyBytes specifies the 1MB maximum payload ceiling for JSON request bodies.
const MaxRequestBodyBytes = 1 << 20

// DecodeJSONBody limits request body to 1MB and strictly decodes JSON into dst without allowing unknown fields.
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
