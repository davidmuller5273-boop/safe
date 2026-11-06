// Package botscope scopes per-group data by bot instance (母机器人 / 克隆机器人).
//
// Every per-chat table carries a bot_id column; unique keys are (bot_id, chat_id, …)
// so two bots that happen to see the same chat_id never share settings, ads,
// admins, members, subscriptions or outbox rows.
package botscope

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// sep joins bot_id and chat_id into one opaque key for code paths (runners,
// outbox fan-out) that only carry a single "chat" string. Chat IDs are numeric
// or @usernames, so '|' never appears in them.
const sep = "|"

// Key encodes (botID, chatID) into one string.
func Key(botID, chatID string) string { return botID + sep + chatID }

// Split decodes a Key. A string without the separator is returned as chatID
// with an empty botID.
func Split(key string) (botID, chatID string) {
	if i := strings.Index(key, sep); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

// TokenBotID returns the numeric bot id prefix of a "123456:ABC…" token
// (Telegram and SafeW tokens both start with the bot's user id).
func TokenBotID(token string) string {
	token = strings.TrimSpace(token)
	if i := strings.Index(token, ":"); i > 0 {
		return token[:i]
	}
	return ""
}

// Table describes one bot-scoped table and its scoped unique key.
type Table struct {
	Name       string
	UniqueName string
	UniqueCols []string // must start with bot_id
}

// Tables are the per-chat tables owned by safew-bot.
var Tables = []Table{
	{"bot_group_settings", "uk_bgs_bot_chat", []string{"bot_id", "chat_id"}},
	{"bot_group_ads", "uk_bga_bot_chat", []string{"bot_id", "chat_id"}},
	{"bot_group_admins", "uk_bgadm_bot_user_chat", []string{"bot_id", "user_id", "chat_id"}},
	{"bot_group_members", "uk_bgm_bot_chat_user", []string{"bot_id", "chat_id", "user_id"}},
	{"bot_admins", "uk_ba_bot_user", []string{"bot_id", "user_id"}},
	{"lottery_broadcast_subscriptions", "uk_lb_subs_bot_chat_selector", []string{"bot_id", "chat_id", "selector"}},
	{"lottery_broadcast_outbox", "uk_lb_outbox_bot_chat_game_issue", []string{"bot_id", "chat_id", "game_code", "issue"}},
}

func quiet(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

func safeIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// EnsureTable is idempotent: adds bot_id (default ”) if missing, adds the
// scoped unique key, then drops every older unique key that lacks bot_id.
// Existing rows keep bot_id=” until BackfillMother assigns the mother's id.
func EnsureTable(db *gorm.DB, t Table) error {
	if !safeIdent(t.Name) || !safeIdent(t.UniqueName) {
		return fmt.Errorf("invalid identifier %s/%s", t.Name, t.UniqueName)
	}
	q := quiet(db)
	var exists int64
	if err := q.Raw("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", t.Name).Scan(&exists).Error; err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	var hasCol int64
	if err := q.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = 'bot_id'", t.Name).Scan(&hasCol).Error; err != nil {
		return err
	}
	if hasCol == 0 {
		if err := q.Exec("ALTER TABLE `" + t.Name + "` ADD COLUMN `bot_id` varchar(64) NOT NULL DEFAULT ''").Error; err != nil &&
			!strings.Contains(err.Error(), "1060") {
			return err
		}
	}
	type idxRow struct {
		IndexName  string `gorm:"column:INDEX_NAME"`
		ColumnName string `gorm:"column:COLUMN_NAME"`
	}
	var rows []idxRow
	if err := q.Raw("SELECT INDEX_NAME, COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND NON_UNIQUE = 0 AND INDEX_NAME <> 'PRIMARY' ORDER BY INDEX_NAME, SEQ_IN_INDEX", t.Name).Scan(&rows).Error; err != nil {
		return err
	}
	hasBot := map[string]bool{}
	names := []string{}
	for _, r := range rows {
		if _, seen := hasBot[r.IndexName]; !seen {
			names = append(names, r.IndexName)
			hasBot[r.IndexName] = false
		}
		if strings.EqualFold(r.ColumnName, "bot_id") {
			hasBot[r.IndexName] = true
		}
	}
	if _, ok := hasBot[t.UniqueName]; !ok {
		cols := make([]string, len(t.UniqueCols))
		for i, c := range t.UniqueCols {
			cols[i] = "`" + c + "`"
		}
		if err := q.Exec("ALTER TABLE `" + t.Name + "` ADD UNIQUE KEY `" + t.UniqueName + "` (" + strings.Join(cols, ",") + ")").Error; err != nil &&
			!strings.Contains(err.Error(), "1061") {
			return err
		}
	}
	for _, name := range names {
		if hasBot[name] || name == t.UniqueName || !safeIdent(name) {
			continue
		}
		if err := q.Exec("ALTER TABLE `" + t.Name + "` DROP INDEX `" + name + "`").Error; err != nil &&
			!strings.Contains(err.Error(), "1091") {
			return err
		}
	}
	return nil
}

// EnsureSchema runs EnsureTable for every table in Tables plus extra ones.
func EnsureSchema(db *gorm.DB, extra ...Table) error {
	for _, t := range append(append([]Table{}, Tables...), extra...) {
		if err := EnsureTable(db, t); err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
	}
	return nil
}

// BackfillMother assigns legacy rows (bot_id=”) to the mother bot. UPDATE IGNORE
// skips the (impossible in practice) rows that would collide with an existing scoped row.
func BackfillMother(db *gorm.DB, motherID string, extra ...Table) error {
	if strings.TrimSpace(motherID) == "" {
		return fmt.Errorf("母机器人 bot_id 为空")
	}
	for _, t := range append(append([]Table{}, Tables...), extra...) {
		if !safeIdent(t.Name) {
			continue
		}
		var exists int64
		if err := quiet(db).Raw("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", t.Name).Scan(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		if err := quiet(db).Exec("UPDATE IGNORE `"+t.Name+"` SET bot_id = ? WHERE bot_id = ''", motherID).Error; err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
	}
	return nil
}
