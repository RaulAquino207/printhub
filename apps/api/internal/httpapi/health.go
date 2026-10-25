package httpapi

import (
	"encoding/json"
	"net/http"
)

type healthResponse struct {
	Status string `json:"status"`
}

// handleHealth reports that the process is up and able to serve requests.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// The status line is already sent, so an encoding error cannot be reported to the client.
	_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}
