package safew

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	UpdateID     int               `json:"update_id"`
	Message      *Message          `json:"message"`
	MyChatMember *ChatMemberUpdated `json:"my_chat_member"`
	ChatMember   *ChatMemberUpdated `json:"chat_member"`
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

func (c *Client) SendMessage(ctx context.Context, token, chatID, text string) error {
	body, err := json.Marshal(map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML"})
	if err != nil {
		return err
	}
	return c.do(ctx, token, "sendMessage", body, nil)
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
		"allowed_updates": []string{"message", "my_chat_member", "chat_member"},
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
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
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
		return fmt.Errorf("SafeW API 错误（HTTP %d, code %d）: %s", response.StatusCode, envelope.ErrorCode, envelope.Description)
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
