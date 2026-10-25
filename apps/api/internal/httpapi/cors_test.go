package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	const allowedOrigin = "http://localhost:3000"
	router := NewRouter(Dependencies{
		Logger:      slog.New(slog.DiscardHandler),
		CORSOrigins: []string{allowedOrigin},
	})

	tests := []struct {
		name            string
		method          string
		origin          string
		preflightMethod string
		wantStatus      int
		wantAllowOrigin string
		wantAllowMethod bool
	}{
		{
			name:       "same-origin or non-browser request",
			method:     http.MethodGet,
			wantStatus: http.StatusOK,
		},
		{
			name:            "allowed origin",
			method:          http.MethodGet,
			origin:          allowedOrigin,
			wantStatus:      http.StatusOK,
			wantAllowOrigin: allowedOrigin,
		},
		{
			name:       "unknown origin",
			method:     http.MethodGet,
			origin:     "http://evil.example",
			wantStatus: http.StatusOK,
		},
		{
			name:            "preflight from allowed origin",
			method:          http.MethodOptions,
			origin:          allowedOrigin,
			preflightMethod: http.MethodPost,
			wantStatus:      http.StatusNoContent,
			wantAllowOrigin: allowedOrigin,
			wantAllowMethod: true,
		},
		{
			name:            "preflight from unknown origin",
			method:          http.MethodOptions,
			origin:          "http://evil.example",
			preflightMethod: http.MethodPost,
			wantStatus:      http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/healthz", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.preflightMethod != "" {
				req.Header.Set("Access-Control-Request-Method", tt.preflightMethod)
			}
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tt.wantAllowOrigin)
			}
			wantCredentials := ""
			if tt.wantAllowOrigin != "" {
				wantCredentials = "true"
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != wantCredentials {
				t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, wantCredentials)
			}
			if got := rec.Header().Get("Access-Control-Allow-Methods") != ""; got != tt.wantAllowMethod {
				t.Errorf("Access-Control-Allow-Methods present = %v, want %v", got, tt.wantAllowMethod)
			}
			if got := rec.Header().Values("Vary"); len(got) == 0 || got[0] != "Origin" {
				t.Errorf("Vary = %v, want it to start with Origin", got)
			}
		})
	}
}
