package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestForwardResponsesAndRawBody(t *testing.T) {
	raw := readFixture(t, "private-message.json")
	for _, upstreamStatus := range []int{http.StatusOK, http.StatusCreated} {
		t.Run(http.StatusText(upstreamStatus), func(t *testing.T) {
			var received []byte
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var err error
				received, err = io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read upstream body: %v", err)
				}
				if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-WuzAPI-Gateway") != "true" {
					t.Errorf("unexpected forwarding headers: %v", r.Header)
				}
				w.WriteHeader(upstreamStatus)
			}))
			defer upstream.Close()

			result := callWebhook(t, newTestHandler(upstream.URL, testSecret, true, http.DefaultClient), raw, signature(raw, testSecret))
			if result.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", result.Code)
			}
			if !bytes.Equal(received, raw) {
				t.Fatal("upstream body differs from original raw body")
			}
		})
	}
}

func TestDroppedPayloadsNeverReachUpstream(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	handler := newTestHandler(upstream.URL, "", false, http.DefaultClient)

	for _, fixture := range []string{"status.json", "group.json", "from-me.json"} {
		result := callWebhook(t, handler, readFixture(t, fixture), "")
		if result.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200", fixture, result.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}

func TestHMAC(t *testing.T) {
	raw := readFixture(t, "private-message.json")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer upstream.Close()
	handler := newTestHandler(upstream.URL, testSecret, true, http.DefaultClient)

	tests := []struct {
		name      string
		signature string
		want      int
	}{
		{"valid", signature(raw, testSecret), http.StatusOK},
		{"valid prefixed", "sha256=" + signature(raw, testSecret), http.StatusOK},
		{"invalid", signature(raw, "wrong-secret"), http.StatusUnauthorized},
		{"missing", "", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := callWebhook(t, handler, raw, tt.signature).Code; got != tt.want {
				t.Fatalf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUpstreamFailuresReturnBadGateway(t *testing.T) {
	raw := readFixture(t, "private-message.json")
	for _, upstreamStatus := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run("upstream "+http.StatusText(upstreamStatus), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(upstreamStatus) }))
			defer upstream.Close()
			if got := callWebhook(t, newTestHandler(upstream.URL, "", false, http.DefaultClient), raw, "").Code; got != http.StatusBadGateway {
				t.Fatalf("status = %d, want 502", got)
			}
		})
	}
	t.Run("timeout", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer upstream.Close()
		client := &http.Client{Timeout: 10 * time.Millisecond}
		if got := callWebhook(t, newTestHandler(upstream.URL, "", false, client), raw, "").Code; got != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", got)
		}
	})
}

func TestBodyLimitAndHealth(t *testing.T) {
	handler := NewHandler("https://example.test", "", false, 10, http.DefaultClient, testLogger())
	request := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader([]byte(`{"more":"than ten bytes"}`)))
	result := httptest.NewRecorder()
	handler.Routes().ServeHTTP(result, request)
	if result.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body limit status = %d, want 413", result.Code)
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthResult := httptest.NewRecorder()
	handler.Routes().ServeHTTP(healthResult, healthRequest)
	if healthResult.Code != http.StatusOK || healthResult.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("health = %d %q", healthResult.Code, healthResult.Body.String())
	}
}

func TestLogsExcludePayloadAndIdentifiers(t *testing.T) {
	raw := readFixture(t, "private-message.json")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer upstream.Close()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := NewHandler(upstream.URL, testSecret, true, 1<<20, http.DefaultClient, logger)
	callWebhook(t, handler, raw, signature(raw, testSecret))

	for _, privateValue := range []string{
		"Mensaje de prueba sanitizado",
		"573001234567@s.whatsapp.net",
		"SANITIZED-MESSAGE-ID-001",
		testSecret,
		signature(raw, testSecret),
	} {
		if strings.Contains(logs.String(), privateValue) {
			t.Fatalf("logs contain private value %q", privateValue)
		}
	}
}

func newTestHandler(url, secret string, requireHMAC bool, client *http.Client) *Handler {
	return NewHandler(url, secret, requireHMAC, 1<<20, client, testLogger())
}

func callWebhook(t *testing.T, handler *Handler, raw []byte, hmacSignature string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(raw))
	if hmacSignature != "" {
		request.Header.Set("x-hmac-signature", hmacSignature)
	}
	result := httptest.NewRecorder()
	handler.Routes().ServeHTTP(result, request)
	return result
}

func signature(raw []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
