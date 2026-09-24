package ads

import (
	"errors"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"gorm.io/gorm"
)

const (
	GlobalPrefixKey = "prefix_ad"
	GlobalSuffixKey = "suffix_ad"
)

// BuildOutboundText joins non-empty prefix, body, suffix with a blank line.
// If both ads are empty, returns body only. Always a single string for one sendMessage.
func BuildOutboundText(prefix, body, suffix string) string {
	parts := make([]string, 0, 3)
	if p := strings.TrimSpace(prefix); p != "" {
		parts = append(parts, p)
	}
	if b := strings.TrimRight(body, "\n"); b != "" {
		parts = append(parts, b)
	} else if body != "" {
		parts = append(parts, body)
	}
	if s := strings.TrimSpace(suffix); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n")
}

type Settings struct {
	PrefixAd string
	SuffixAd string
	Scope    string // "global" or "group"
	ChatID   string
}

func LoadForChat(db *gorm.DB, chatID string) (Settings, error) {
	chatID = strings.TrimSpace(chatID)
	if chatID != "" {
		var group domain.BotGroupAd
		err := db.Where("chat_id = ?", chatID).First(&group).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return Settings{}, err
		}
		if err == nil && (strings.TrimSpace(group.PrefixAd) != "" || strings.TrimSpace(group.SuffixAd) != "") {
			// Per side: the group value wins; an empty side falls back to the global ad,
			// so setting only a group prefix keeps the global suffix (and vice versa).
			global, gerr := LoadGlobal(db)
			if gerr != nil {
				return Settings{}, gerr
			}
			return mergeSides(group.PrefixAd, group.SuffixAd, global, chatID), nil
		}
	}
	return LoadGlobal(db)
}

func mergeSides(groupPrefix, groupSuffix string, global Settings, chatID string) Settings {
	s := Settings{PrefixAd: global.PrefixAd, SuffixAd: global.SuffixAd, Scope: "group", ChatID: chatID}
	if strings.TrimSpace(groupPrefix) != "" {
		s.PrefixAd = groupPrefix
	}
	if strings.TrimSpace(groupSuffix) != "" {
		s.SuffixAd = groupSuffix
	}
	return s
}

// SplitPrefixSuffix parses "前广告 | 后广告" or a multi-line form where a line
// containing only "---" or "|" separates the two sides. ok is false when no separator is found.
func SplitPrefixSuffix(text string) (prefix, suffix string, ok bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "---" || t == "|" || t == "｜" {
			return strings.TrimSpace(strings.Join(lines[:i], "\n")), strings.TrimSpace(strings.Join(lines[i+1:], "\n")), true
		}
	}
	for _, sep := range []string{"|", "｜"} {
		if idx := strings.Index(text, sep); idx >= 0 {
			return strings.TrimSpace(text[:idx]), strings.TrimSpace(text[idx+len(sep):]), true
		}
	}
	return "", "", false
}

func LoadGlobal(db *gorm.DB) (Settings, error) {
	var configs []domain.SystemConfig
	if err := db.Where("`key` IN ?", []string{GlobalPrefixKey, GlobalSuffixKey}).Find(&configs).Error; err != nil {
		return Settings{}, err
	}
	s := Settings{Scope: "global"}
	for _, c := range configs {
		switch c.Key {
		case GlobalPrefixKey:
			s.PrefixAd = c.Value
		case GlobalSuffixKey:
			s.SuffixAd = c.Value
		}
	}
	return s, nil
}

func SaveGlobal(db *gorm.DB, prefix, suffix *string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if prefix != nil {
			if err := upsertConfig(tx, GlobalPrefixKey, *prefix); err != nil {
				return err
			}
		}
		if suffix != nil {
			if err := upsertConfig(tx, GlobalSuffixKey, *suffix); err != nil {
				return err
			}
		}
		return nil
	})
}

func SaveGroup(db *gorm.DB, chatID string, prefix, suffix *string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return errors.New("chat_id 不能为空")
	}
	var row domain.BotGroupAd
	err := db.Where("chat_id = ?", chatID).FirstOrCreate(&row, domain.BotGroupAd{ChatID: chatID}).Error
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if prefix != nil {
		updates["prefix_ad"] = *prefix
	}
	if suffix != nil {
		updates["suffix_ad"] = *suffix
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&row).Updates(updates).Error
}

func ClearGroup(db *gorm.DB, chatID string) error {
	return db.Where("chat_id = ?", strings.TrimSpace(chatID)).Delete(&domain.BotGroupAd{}).Error
}

func upsertConfig(tx *gorm.DB, key, value string) error {
	cfg := domain.SystemConfig{Key: key}
	return tx.Where("`key` = ?", key).Assign(domain.SystemConfig{Value: value}).FirstOrCreate(&cfg).Error
}

// Wrap loads ads for chat and builds a single outbound text.
func Wrap(db *gorm.DB, chatID, body string) (string, error) {
	s, err := LoadForChat(db, chatID)
	if err != nil {
		return "", err
	}
	return BuildOutboundText(s.PrefixAd, body, s.SuffixAd), nil
}
