package webhook

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecideFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		accepted bool
		reason   string
	}{
		{"private s.whatsapp.net", "private-message.json", true, ""},
		{"private lid", "private-lid-message.json", true, ""},
		{"status", "status.json", false, "status"},
		{"group", "group.json", false, "group"},
		{"from me", "from-me.json", false, "from_me"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := readFixture(t, tt.fixture)
			got := Decide(raw)
			if got.Accepted != tt.accepted || got.Reason != tt.reason {
				t.Fatalf("Decide() = %+v, want accepted=%v reason=%q", got, tt.accepted, tt.reason)
			}
		})
	}
}

func TestDecideDefaultDeny(t *testing.T) {
	validInfo := `"Sender":"573001234567@s.whatsapp.net","IsFromMe":false,"IsGroup":false,"ID":"id","Timestamp":"2026-09-14T12:00:00Z"`
	tests := []struct {
		name   string
		raw    string
		reason string
	}{
		{"g.us suffix despite flag", `{"type":"Message","event":{"Info":{"Chat":"123@g.us",` + validInfo + `}}}`, "group"},
		{"newsletter", `{"type":"Message","event":{"Info":{"Chat":"123@newsletter",` + validInfo + `}}}`, "newsletter"},
		{"broadcast", `{"type":"Message","event":{"Info":{"Chat":"123@broadcast",` + validInfo + `}}}`, "broadcast"},
		{"not message", `{"type":"ReadReceipt","event":{}}`, "not_message"},
		{"invalid JSON", `{`, "invalid_payload"},
		{"missing chat", `{"type":"Message","event":{"Info":{` + validInfo + `}}}`, "invalid_payload"},
		{"missing info", `{"type":"Message","event":{}}`, "invalid_payload"},
		{"unsupported chat", `{"type":"Message","event":{"Info":{"Chat":"123@example.net",` + validInfo + `}}}`, "unsupported_chat"},
		{"non-numeric private chat", `{"type":"Message","event":{"Info":{"Chat":"abc@lid",` + validInfo + `}}}`, "unsupported_chat"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide([]byte(tt.raw))
			if got.Accepted || got.Reason != tt.reason {
				t.Fatalf("Decide() = %+v, want drop reason=%q", got, tt.reason)
			}
		})
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return raw
}
