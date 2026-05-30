package api

import (
	"encoding/json"
	"net/http"
)

// writeOK encodes a successful enveloped response.
func writeOK[T any](w http.ResponseWriter, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(Envelope[T]{OK: true, Data: &data})
}

// writeErr encodes an enveloped error with the given status. The message is a safe,
// user-facing string; internal detail is logged separately, never leaked.
func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope[struct{}]{OK: false, Error: msg})
}
