package ads

import (
	"errors"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	Scope    string // "group" (chat has its own ad) or "none"
	ChatID   string
}

// LoadForChat returns the chat's OWN ads only. There is no global fallback:
// a chat without its own prefix/suffix gets empty Settings (Scope "none"), and
// Wrap then sends just the body.
func LoadForChat(db *gorm.DB, chatID string) (Settings, error) {
	chatID = strings.TrimSpace(chatID)
	none := Settings{Scope: "none", ChatID: chatID}
	if chatID == "" {
		return none, nil
	}
	var group domain.BotGroupAd
	// Quiet: a missing group row is the normal "no ads" case.
	err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
		Where("chat_id = ?", chatID).First(&group).Error
	if err == gorm.ErrRecordNotFound {
		return none, nil
	}
	if err != nil {
		return Settings{}, err
	}
	return groupSettings(group, chatID), nil
}

// groupSettings turns a chat's own ad row into Settings; both sides empty means
// "no ads" (Scope "none"). Nothing else is ever merged in.
func groupSettings(group domain.BotGroupAd, chatID string) Settings {
	if strings.TrimSpace(group.PrefixAd) == "" && strings.TrimSpace(group.SuffixAd) == "" {
		return Settings{Scope: "none", ChatID: chatID}
	}
	return Settings{PrefixAd: group.PrefixAd, SuffixAd: group.SuffixAd, Scope: "group", ChatID: chatID}
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

// Wrap loads the chat's own ads (never a global ad) and builds a single outbound text.
func Wrap(db *gorm.DB, chatID, body string) (string, error) {
	s, err := LoadForChat(db, chatID)
	if err != nil {
		return "", err
	}
	return BuildOutboundText(s.PrefixAd, body, s.SuffixAd), nil
}
