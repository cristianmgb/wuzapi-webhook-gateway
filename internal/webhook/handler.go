package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	n8nURL       string
	hmacSecret   []byte
	requireHMAC  bool
	maxBodyBytes int64
	client       *http.Client
	logger       *slog.Logger
}

type response struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

func NewHandler(n8nURL, hmacSecret string, requireHMAC bool, maxBodyBytes int64, client *http.Client, logger *slog.Logger) *Handler {
	return &Handler{
		n8nURL:       n8nURL,
		hmacSecret:   []byte(hmacSecret),
		requireHMAC:  requireHMAC,
		maxBodyBytes: maxBodyBytes,
		client:       client,
		logger:       logger,
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /webhook", h.webhook)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, response{Accepted: false, Reason: "invalid_payload"})
			return
		}
		h.logger.Error("request body read failed", "action", "reject")
		writeJSON(w, http.StatusBadRequest, response{Accepted: false, Reason: "invalid_payload"})
		return
	}

	if len(h.hmacSecret) > 0 && !validSignature(raw, r.Header.Get("x-hmac-signature"), h.hmacSecret) {
		h.logger.Warn("webhook authentication failed", "action", "reject", "reason", "invalid_hmac")
		writeJSON(w, http.StatusUnauthorized, response{Accepted: false, Reason: "invalid_payload"})
		return
	}
	if h.requireHMAC && len(h.hmacSecret) == 0 {
		// Startup validation prevents this state. Keep fail-closed if Handler is constructed directly.
		writeJSON(w, http.StatusUnauthorized, response{Accepted: false, Reason: "invalid_payload"})
		return
	}

	decision := Decide(raw)
	if !decision.Accepted {
		h.logger.Info("webhook dropped", "action", "drop", "reason", decision.Reason)
		writeJSON(w, http.StatusOK, response{Accepted: false, Reason: decision.Reason})
		return
	}

	started := time.Now()
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, h.n8nURL, bytes.NewReader(raw))
	if err != nil {
		h.forwardError(w, started, 0, err)
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-WuzAPI-Gateway", "true")

	h.logger.Info("webhook forwarding", "action", "forward", "event", decision.Event, "message_id", messageIDHash(decision.MessageID))
	upstream, err := h.client.Do(request)
	if err != nil {
		h.forwardError(w, started, 0, err)
		return
	}
	defer upstream.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(upstream.Body, 64<<10))

	if upstream.StatusCode < 200 || upstream.StatusCode > 299 {
		h.forwardError(w, started, upstream.StatusCode, nil)
		return
	}
	h.logger.Info("webhook forwarded", "action", "forward_success", "status", upstream.StatusCode, "duration_ms", time.Since(started).Milliseconds(), "message_id", messageIDHash(decision.MessageID))
	writeJSON(w, http.StatusOK, response{Accepted: true})
}

func (h *Handler) forwardError(w http.ResponseWriter, started time.Time, upstreamStatus int, err error) {
	attributes := []any{"action", "forward_error", "status", http.StatusBadGateway, "duration_ms", time.Since(started).Milliseconds()}
	if upstreamStatus != 0 {
		attributes = append(attributes, "upstream_status", upstreamStatus)
	}
	if err != nil {
		attributes = append(attributes, "error", "upstream request failed")
	}
	h.logger.Error("webhook forward failed", attributes...)
	writeJSON(w, http.StatusBadGateway, response{Accepted: false, Reason: "upstream_error"})
}

func validSignature(raw []byte, signature string, secret []byte) bool {
	signature = strings.TrimPrefix(strings.TrimSpace(signature), "sha256=")
	provided, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(raw)
	return hmac.Equal(provided, mac.Sum(nil))
}

func messageIDHash(messageID string) string {
	digest := sha256.Sum256([]byte(messageID))
	return hex.EncodeToString(digest[:6])
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
