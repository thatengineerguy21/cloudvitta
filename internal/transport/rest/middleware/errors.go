package middleware

import (
	"encoding/json"
	"net/http"
)

// RFC7807Error represents an RFC 7807 Problem Details error response.
type RFC7807Error struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// WriteJSONError renders an RFC 7807 problem details JSON payload to the ResponseWriter.
func WriteJSONError(w http.ResponseWriter, r *http.Request, status int, errorType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	instance := r.URL.RequestURI()
	if instance == "" {
		instance = r.URL.Path
	}

	resp := RFC7807Error{
		Type:     errorType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}

	_ = json.NewEncoder(w).Encode(resp)
}
