package webhook

import (
	"encoding/json"
	"strings"
)

type Decision struct {
	Accepted  bool
	Reason    string
	Event     string
	MessageID string
}

type envelope struct {
	Type  *string `json:"type"`
	Event *event  `json:"event"`
}

type event struct {
	Info *messageInfo `json:"Info"`
}

type messageInfo struct {
	Chat      *string `json:"Chat"`
	Sender    *string `json:"Sender"`
	IsFromMe  *bool   `json:"IsFromMe"`
	IsGroup   *bool   `json:"IsGroup"`
	ID        *string `json:"ID"`
	Timestamp *string `json:"Timestamp"`
}

func Decide(raw []byte) Decision {
	var payload envelope
	if err := json.Unmarshal(raw, &payload); err != nil {
		return drop("invalid_payload")
	}
	if payload.Type == nil {
		return drop("invalid_payload")
	}
	if *payload.Type != "Message" {
		return drop("not_message")
	}
	if payload.Event == nil || payload.Event.Info == nil {
		return drop("invalid_payload")
	}

	info := payload.Event.Info
	if info.Chat == nil || *info.Chat == "" || info.Sender == nil || *info.Sender == "" ||
		info.IsFromMe == nil || info.IsGroup == nil || info.ID == nil || *info.ID == "" || info.Timestamp == nil || *info.Timestamp == "" {
		return drop("invalid_payload")
	}
	if *info.IsFromMe {
		return drop("from_me")
	}
	if *info.IsGroup {
		return drop("group")
	}

	chat := *info.Chat
	if chat == "status@broadcast" {
		return drop("status")
	}
	if strings.HasSuffix(chat, "@g.us") {
		return drop("group")
	}
	if strings.HasSuffix(chat, "@newsletter") {
		return drop("newsletter")
	}
	if strings.HasSuffix(chat, "@broadcast") {
		return drop("broadcast")
	}
	if !validPrivateChat(chat) {
		return drop("unsupported_chat")
	}

	return Decision{Accepted: true, Event: *payload.Type, MessageID: *info.ID}
}

func validPrivateChat(chat string) bool {
	var local string
	switch {
	case strings.HasSuffix(chat, "@s.whatsapp.net"):
		local = strings.TrimSuffix(chat, "@s.whatsapp.net")
	case strings.HasSuffix(chat, "@lid"):
		local = strings.TrimSuffix(chat, "@lid")
	default:
		return false
	}
	if local == "" {
		return false
	}
	for _, char := range local {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func drop(reason string) Decision {
	return Decision{Reason: reason}
}
