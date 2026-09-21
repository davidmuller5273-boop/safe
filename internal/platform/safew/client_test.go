package safew

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Path, "/bot123:secret/sendMessage"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		var body struct {
			ChatID string `json:"chat_id"`
			Text   string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.ChatID != "-100123" || body.Text != "开奖消息" {
			t.Errorf("body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	if err := NewClient(server.URL).SendMessage(context.Background(), "123:secret", "-100123", "开奖消息"); err != nil {
		t.Fatal(err)
	}
}

func TestSendMessageReturnsAPIDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"chat not found"}`))
	}))
	defer server.Close()

	err := NewClient(server.URL).SendMessage(context.Background(), "token", "missing", "test")
	if err == nil {
		t.Fatal("expected API error")
	}
}
