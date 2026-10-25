package main

import (
	"slices"
	"testing"
)

func TestLoadConfigCORSOrigins(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{name: "default", value: "", want: []string{"http://localhost:3000"}},
		{name: "list with spaces and blanks", value: " http://a.test , ,http://b.test", want: []string{"http://a.test", "http://b.test"}},
		{name: "wildcard rejected", value: "http://a.test,*", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv restores the previous value when the test ends.
			t.Setenv("PRINTHUB_CORS_ALLOWED_ORIGINS", tt.value)

			cfg, err := loadConfig()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(cfg.corsOrigins, tt.want) {
				t.Errorf("corsOrigins = %v, want %v", cfg.corsOrigins, tt.want)
			}
		})
	}
}

func TestLoadConfigMaxUpload(t *testing.T) {
	t.Setenv("PRINTHUB_MAX_UPLOAD_MB", "")
	cfg, err := loadConfig()
	if err != nil || cfg.maxUploadBytes != 50<<20 {
		t.Errorf("default = %d, %v; want 50 MiB", cfg.maxUploadBytes, err)
	}

	for _, bad := range []string{"0", "-1", "lots"} {
		t.Setenv("PRINTHUB_MAX_UPLOAD_MB", bad)
		if _, err := loadConfig(); err == nil {
			t.Errorf("PRINTHUB_MAX_UPLOAD_MB=%q: want an error", bad)
		}
	}
}
