package safew

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendMessageExReturnsMessageIDAndMarkup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		markup, ok := body["reply_markup"].(map[string]any)
		if !ok {
			t.Fatalf("reply_markup missing: %v", body)
		}
		rows := markup["inline_keyboard"].([]any)
		first := rows[0].([]any)[0].(map[string]any)
		if first["text"] != "查看开奖" || first["callback_data"] != "u:q:lt" {
			t.Errorf("button = %v", first)
		}
		if body["parse_mode"] != "HTML" {
			t.Errorf("parse_mode = %v", body["parse_mode"])
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":77,"chat":{"id":-100123,"type":"supergroup"}}}`))
	}))
	defer server.Close()
	markup := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{{Text: "查看开奖", CallbackData: "u:q:lt"}}}}
	msg, err := NewClient(server.URL).SendMessageEx(context.Background(), "t", "-100123", "hi", SendOptions{ReplyMarkup: markup})
	if err != nil {
		t.Fatal(err)
	}
	if msg.MessageID != 77 {
		t.Fatalf("message_id = %d", msg.MessageID)
	}
}

func TestPinChatMessageFallsBackToLowercasePath(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.HasSuffix(r.URL.Path, "/pinChatMessage") {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"ok":false,"error_code":404,"description":"Not Found"}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["message_id"].(float64) != 77 || body["chat_id"] != "-100123" {
			t.Errorf("body = %v", body)
		}
		if _, ok := body["disable_notification"]; ok {
			t.Errorf("normal pin must not set disable_notification")
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()
	if err := NewClient(server.URL).PinChatMessage(context.Background(), "t", "-100123", 77, false); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[1] != "/bott/pinchatmessage" {
		t.Fatalf("paths = %v", paths)
	}
}

func TestPinChatMessageNotAdminReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: not enough rights to pin a message"}`))
	}))
	defer server.Close()
	if err := NewClient(server.URL).PinChatMessage(context.Background(), "t", "-1", 1, false); err == nil {
		t.Fatal("expected error")
	}
}

func TestAnswerCallbackQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/answerCallbackQuery") {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["callback_query_id"] != "cb1" || body["text"] != "权限不足" {
			t.Errorf("body = %v", body)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	defer server.Close()
	if err := NewClient(server.URL).AnswerCallbackQuery(context.Background(), "t", "cb1", "权限不足", false); err != nil {
		t.Fatal(err)
	}
}

func TestSendDocumentMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/sendDocument") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("chat_id") != "42" || r.FormValue("caption") != "导出" {
			t.Errorf("fields = %v", r.MultipartForm.Value)
		}
		f, hdr, err := r.FormFile("document")
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(f)
		if hdr.Filename != "a.csv" || string(data) != "\ufeffx,y" {
			t.Errorf("file = %s %q", hdr.Filename, data)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":5}}`))
	}))
	defer server.Close()
	msg, err := NewClient(server.URL).SendDocument(context.Background(), "t", "42", "a.csv", []byte("\ufeffx,y"), "导出")
	if err != nil || msg.MessageID != 5 {
		t.Fatalf("msg=%v err=%v", msg, err)
	}
}

func TestGetUpdatesDecodesCallbackQueryAndAllowsIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(raw), "callback_query") {
			t.Errorf("allowed_updates must include callback_query: %s", raw)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"update_id":9,"callback_query":{"id":"cb9","from":{"id":1001,"first_name":"A"},"data":"s:p:1","message":{"message_id":3,"chat":{"id":-100,"type":"group"}}}}]}`))
	}))
	defer server.Close()
	updates, err := NewClient(server.URL).GetUpdates(context.Background(), "t", 0, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 1 || updates[0].CallbackQuery == nil {
		t.Fatalf("updates = %+v", updates)
	}
	cq := updates[0].CallbackQuery
	if cq.ID != "cb9" || cq.Data != "s:p:1" || cq.From.IDString() != "1001" || cq.Message.MessageID != 3 || cq.Message.Chat.IDString() != "-100" {
		t.Fatalf("callback = %+v", cq)
	}
}

func TestIsNotModified(t *testing.T) {
	if !IsNotModified(&APIError{HTTPStatus: 400, Description: "Bad Request: message is not modified"}) {
		t.Fatal("expected not modified")
	}
	if IsNotModified(nil) {
		t.Fatal("nil is not 'not modified'")
	}
}
