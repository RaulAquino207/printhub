package httpapi

import (
	"encoding/json"
	"net/http"
)

// errorResponse is the body of every API error. Code is stable and meant for
// programs (the web maps it to a pt-BR message); Message is for humans reading
// logs or curl output.
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Fields lists invalid input fields, for validation errors.
	Fields []fieldError `json:"fields,omitempty"`
}

type fieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status line is already sent, so an encoding error cannot be reported to the client.
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}

func writeValidationError(w http.ResponseWriter, fields []fieldError) {
	writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Error: errorBody{
		Code:    "validation_failed",
		Message: "the request has invalid fields",
		Fields:  fields,
	}})
}
