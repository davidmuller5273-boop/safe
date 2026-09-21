package systemconfig

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"gorm.io/gorm"
)

const (
	SafeWBotTokenKey = "safew_bot_token"
	SafeWChatIDKey   = "safew_chat_id"
)

type SafeW struct {
	Token   string
	ChatIDs []string
}

func LoadSafeW(db *gorm.DB) (SafeW, error) {
	result, err := ReadSafeW(db)
	if err != nil {
		return SafeW{}, err
	}
	// Token alone is enough for command polling / private chats.
	// Group chat IDs are optional; needed only when broadcasting lottery pushes.
	if result.Token == "" {
		return result, errors.New("请先在系统配置中设置 SafeW 机器人 Token")
	}
	return result, nil
}

func ReadSafeW(db *gorm.DB) (SafeW, error) {
	var configs []domain.SystemConfig
	if err := db.Where("`key` IN ?", []string{SafeWBotTokenKey, SafeWChatIDKey}).Find(&configs).Error; err != nil {
		return SafeW{}, err
	}
	result := SafeW{}
	for _, config := range configs {
		switch config.Key {
		case SafeWBotTokenKey:
			result.Token = config.Value
		case SafeWChatIDKey:
			result.ChatIDs = parseChatIDs(config.Value)
		}
	}
	return result, nil
}

func SaveSafeW(db *gorm.DB, token string, chatIDs []string) error {
	chatIDs = NormalizeChatIDs(chatIDs)
	chatIDsJSON, err := json.Marshal(chatIDs)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if token != "" {
			config := domain.SystemConfig{Key: SafeWBotTokenKey}
			if err := tx.Where("`key` = ?", SafeWBotTokenKey).Assign(domain.SystemConfig{Value: token}).FirstOrCreate(&config).Error; err != nil {
				return err
			}
		}
		config := domain.SystemConfig{Key: SafeWChatIDKey}
		return tx.Where("`key` = ?", SafeWChatIDKey).Assign(domain.SystemConfig{Value: string(chatIDsJSON)}).FirstOrCreate(&config).Error
	})
}

// NormalizeChatIDs removes blank and duplicate group IDs while preserving order.
func NormalizeChatIDs(chatIDs []string) []string {
	result := make([]string, 0, len(chatIDs))
	seen := make(map[string]struct{}, len(chatIDs))
	for _, chatID := range chatIDs {
		chatID = strings.TrimSpace(chatID)
		if chatID == "" {
			continue
		}
		if _, exists := seen[chatID]; exists {
			continue
		}
		seen[chatID] = struct{}{}
		result = append(result, chatID)
	}
	return result
}

func parseChatIDs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	var chatIDs []string
	if json.Unmarshal([]byte(value), &chatIDs) == nil {
		return NormalizeChatIDs(chatIDs)
	}
	// Compatible with the original configuration, which stored one plain ID.
	return NormalizeChatIDs([]string{value})
}
