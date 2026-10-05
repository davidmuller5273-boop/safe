package safew

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: &http.Client{Timeout: 45 * time.Second}}
}

type User struct {
	ID        json.Number `json:"id"`
	IsBot     bool        `json:"is_bot"`
	FirstName string      `json:"first_name"`
	LastName  string      `json:"last_name"`
	Username  string      `json:"username"`
}

func (u User) IDString() string {
	return strings.TrimSpace(u.ID.String())
}

type Chat struct {
	ID        json.Number `json:"id"`
	Type      string      `json:"type"`
	Title     string      `json:"title"`
	Username  string      `json:"username"`
	FirstName string      `json:"first_name"`
}

func (c Chat) IDString() string {
	return strings.TrimSpace(c.ID.String())
}

type Message struct {
	MessageID int    `json:"message_id"`
	From      *User  `json:"from"`
	Chat      Chat   `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
}

type Update struct {
	UpdateID      int                `json:"update_id"`
	Message       *Message           `json:"message"`
	MyChatMember  *ChatMemberUpdated `json:"my_chat_member"`
	ChatMember    *ChatMemberUpdated `json:"chat_member"`
	CallbackQuery *CallbackQuery     `json:"callback_query"`
}

// CallbackQuery is an inline-keyboard button click.
type CallbackQuery struct {
	ID              string   `json:"id"`
	From            User     `json:"from"`
	Message         *Message `json:"message"`
	InlineMessageID string   `json:"inline_message_id"`
	Data            string   `json:"data"`
}

// InlineKeyboardButton is one inline button (callback_data must be ≤ 64 bytes).
type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

// InlineKeyboardMarkup is reply_markup.inline_keyboard.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// SendOptions are optional sendMessage fields.
type SendOptions struct {
	ReplyMarkup         *InlineKeyboardMarkup
	DisableNotification bool
	// PlainText disables parse_mode (default is HTML like SendMessage).
	PlainText bool
}

// APIError is a SafeW API error response.
type APIError struct {
	HTTPStatus  int
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("SafeW API 错误（HTTP %d, code %d）: %s", e.HTTPStatus, e.Code, e.Description)
}

// IsNotModified reports the harmless "message is not modified" edit error.
func IsNotModified(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not modified")
}

type ChatMemberUpdated struct {
	Chat          Chat       `json:"chat"`
	From          User       `json:"from"`
	Date          int64      `json:"date"`
	OldChatMember ChatMember `json:"old_chat_member"`
	NewChatMember ChatMember `json:"new_chat_member"`
}

type ChatMember struct {
	User   User   `json:"user"`
	Status string `json:"status"`
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

// ChatIDValue returns a JSON-friendly chat_id: numeric when the string is all
// digits (optional leading -), otherwise the original string. SafeW examples
// use JSON numbers for chat_id.
func ChatIDValue(chatID string) any {
	s := strings.TrimSpace(chatID)
	if s == "" {
		return s
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	return s
}

func (c *Client) SendMessage(ctx context.Context, token, chatID, text string) error {
	_, err := c.SendMessageEx(ctx, token, chatID, text, SendOptions{})
	return err
}

// SendMessageEx sends a message (HTML parse mode unless PlainText) and returns the sent Message
// so callers can pin it or attach/refresh inline keyboards.
// reply_markup is always attached as a JSON object when set. Numeric chat_id strings are
// sent as JSON numbers. If HTML+markup fails, retries once as PlainText with the same markup.
func (c *Client) SendMessageEx(ctx context.Context, token, chatID, text string, opts SendOptions) (Message, error) {
	msg, err := c.sendMessageOnce(ctx, token, chatID, text, opts)
	if err == nil {
		return msg, nil
	}
	if opts.ReplyMarkup != nil && !opts.PlainText {
		log.Printf("sendMessage HTML+markup 失败 chat=%s: %v；改用 PlainText 重试", chatID, err)
		retry := opts
		retry.PlainText = true
		msg2, err2 := c.sendMessageOnce(ctx, token, chatID, text, retry)
		if err2 != nil {
			log.Printf("sendMessage PlainText+markup 仍失败 chat=%s: %v", chatID, err2)
			return Message{}, err2
		}
		return msg2, nil
	}
	log.Printf("sendMessage 失败 chat=%s: %v", chatID, err)
	return Message{}, err
}

func (c *Client) sendMessageOnce(ctx context.Context, token, chatID, text string, opts SendOptions) (Message, error) {
	payload := map[string]any{"chat_id": ChatIDValue(chatID), "text": text}
	if !opts.PlainText {
		payload["parse_mode"] = "HTML"
	}
	if opts.ReplyMarkup != nil {
		payload["reply_markup"] = opts.ReplyMarkup
	}
	if opts.DisableNotification {
		payload["disable_notification"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	var msg Message
	if err := c.do(ctx, token, "sendMessage", body, &msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// PinChatMessage pins a message (bot must be a group admin with pin rights).
func (c *Client) PinChatMessage(ctx context.Context, token, chatID string, messageID int, disableNotification bool) error {
	payload := map[string]any{"chat_id": ChatIDValue(chatID), "message_id": messageID}
	if disableNotification {
		payload["disable_notification"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.doAlt(ctx, token, "pinChatMessage", "pinchatmessage", body, nil)
}

// AnswerCallbackQuery acknowledges a button click with an optional toast (≤200 chars).
func (c *Client) AnswerCallbackQuery(ctx context.Context, token, callbackQueryID, text string, showAlert bool) error {
	payload := map[string]any{"callback_query_id": callbackQueryID}
	if text != "" {
		r := []rune(text)
		if len(r) > 200 {
			text = string(r[:200])
		}
		payload["text"] = text
	}
	if showAlert {
		payload["show_alert"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.doAlt(ctx, token, "answerCallbackQuery", "answercallbackquery", body, nil)
}

// EditMessageReplyMarkup replaces the inline keyboard of a bot message.
func (c *Client) EditMessageReplyMarkup(ctx context.Context, token, chatID string, messageID int, markup *InlineKeyboardMarkup) error {
	payload := map[string]any{"chat_id": ChatIDValue(chatID), "message_id": messageID}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.do(ctx, token, "editMessageReplyMarkup", body, nil)
}

// EditMessageText replaces text (HTML) and keyboard of a bot message.
func (c *Client) EditMessageText(ctx context.Context, token, chatID string, messageID int, text string, markup *InlineKeyboardMarkup) error {
	payload := map[string]any{"chat_id": ChatIDValue(chatID), "message_id": messageID, "text": text, "parse_mode": "HTML"}
	if markup != nil {
		payload["reply_markup"] = markup
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.do(ctx, token, "editMessageText", body, nil)
}

// SendDocument uploads a file via multipart/form-data.
func (c *Client) SendDocument(ctx context.Context, token, chatID, filename string, data []byte, caption string) (Message, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", chatID)
	if caption != "" {
		_ = w.WriteField("caption", caption)
	}
	part, err := w.CreateFormFile("document", filename)
	if err != nil {
		return Message{}, err
	}
	if _, err := part.Write(data); err != nil {
		return Message{}, err
	}
	if err := w.Close(); err != nil {
		return Message{}, err
	}
	var msg Message
	if err := c.doRaw(ctx, token, "sendDocument", w.FormDataContentType(), buf.Bytes(), &msg); err != nil {
		return Message{}, err
	}
	return msg, nil
}

// doAlt calls method, retrying once with alt when the API answers 404
// (the SafeW docs list some paths in lower case, e.g. /pinchatmessage).
func (c *Client) doAlt(ctx context.Context, token, method, alt string, body []byte, result any) error {
	err := c.do(ctx, token, method, body, result)
	var apiErr *APIError
	if err != nil && alt != "" && alt != method && errors.As(err, &apiErr) && apiErr.HTTPStatus == http.StatusNotFound {
		return c.do(ctx, token, alt, body, result)
	}
	return err
}

func (c *Client) GetUpdates(ctx context.Context, token string, offset int64, limit, timeout int) ([]Update, error) {
	if limit <= 0 {
		limit = 100
	}
	if timeout < 0 {
		timeout = 0
	}
	payload, err := json.Marshal(map[string]any{
		"offset":          offset,
		"limit":           limit,
		"timeout":         timeout,
		"allowed_updates": []string{"message", "my_chat_member", "chat_member", "callback_query"},
	})
	if err != nil {
		return nil, err
	}
	// Long poll: allow client timeout longer than API timeout.
	var updates []Update
	if err := c.do(ctx, token, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (c *Client) GetMe(ctx context.Context, token string) (User, error) {
	var user User
	if err := c.do(ctx, token, "getMe", []byte("{}"), &user); err != nil {
		return User{}, err
	}
	return user, nil
}

func (c *Client) do(ctx context.Context, token, method string, body []byte, result any) error {
	return c.doRaw(ctx, token, method, "application/json", body, result)
}

func (c *Client) doRaw(ctx context.Context, token, method, contentType string, body []byte, result any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", contentType)
	response, err := c.httpClient.Do(request)
	if err != nil {
		var urlError *url.Error
		if errors.As(err, &urlError) {
			return fmt.Errorf("SafeW API 请求失败: %w", urlError.Err)
		}
		return errors.New("SafeW API 请求失败")
	}
	defer response.Body.Close()

	raw := json.RawMessage{}
	envelope := struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}{}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("解析 SafeW 响应失败（HTTP %d）: %w", response.StatusCode, err)
	}
	_ = raw
	if response.StatusCode < 200 || response.StatusCode >= 300 || !envelope.OK {
		if envelope.Description == "" {
			envelope.Description = http.StatusText(response.StatusCode)
		}
		return &APIError{HTTPStatus: response.StatusCode, Code: envelope.ErrorCode, Description: envelope.Description}
	}
	if result == nil || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("解析 SafeW result 失败: %w", err)
	}
	return nil
}

func (c *Client) GetChatMemberCount(ctx context.Context, token, chatID string) (int, error) {
	body, err := json.Marshal(map[string]any{"chat_id": chatID})
	if err != nil {
		return 0, err
	}
	var count int
	if err := c.do(ctx, token, "getChatMemberCount", body, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (c *Client) GetChat(ctx context.Context, token, chatID string) (Chat, error) {
	body, err := json.Marshal(map[string]any{"chat_id": chatID})
	if err != nil {
		return Chat{}, err
	}
	var chat Chat
	if err := c.do(ctx, token, "getChat", body, &chat); err != nil {
		return Chat{}, err
	}
	return chat, nil
}

func (c *Client) GetChatAdministrators(ctx context.Context, token, chatID string) ([]ChatMember, error) {
	body, err := json.Marshal(map[string]any{"chat_id": chatID})
	if err != nil {
		return nil, err
	}
	var list []ChatMember
	if err := c.do(ctx, token, "getChatAdministrators", body, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// FormatChatID normalizes numeric or string chat identifiers.
func FormatChatID(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	case json.Number:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
