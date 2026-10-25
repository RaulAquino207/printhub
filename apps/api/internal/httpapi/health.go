package httpapi

import "net/http"

type healthResponse struct {
	Status string `json:"status"`
}

// handleHealth reports that the process is up and able to serve requests.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
}
