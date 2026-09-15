package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("N8N_WEBHOOK_URL", "https://n8n.example.test/webhook/ws")
	t.Setenv("WUZAPI_HMAC_SECRET", "test-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 || cfg.MaxBodyBytes != 1<<20 || cfg.ForwardTimeout.String() != "8s" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"missing URL", map[string]string{"N8N_WEBHOOK_URL": ""}},
		{"invalid URL scheme", map[string]string{"N8N_WEBHOOK_URL": "ftp://example.test/hook"}},
		{"invalid port", map[string]string{"N8N_WEBHOOK_URL": "https://example.test/hook", "PORT": "70000"}},
		{"missing required HMAC", map[string]string{"N8N_WEBHOOK_URL": "https://example.test/hook", "WUZAPI_HMAC_SECRET": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil")
			}
		})
	}
}
