package lotterybroadcast

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Table names. Tables are created by EnsureSchema (CREATE TABLE IF NOT EXISTS),
// never by GORM AutoMigrate, so restarts cannot hit duplicate-index errors.
const (
	TableResults       = "lottery_broadcast_results"
	TableSubscriptions = "lottery_broadcast_subscriptions"
	TableSourceStatus  = "lottery_broadcast_source_status"
	TableOutbox        = "lottery_broadcast_outbox"
)

// System config keys for the admin switches.
const (
	KeyQueryEnabled     = "lottery_query_enabled"
	KeyBroadcastEnabled = "lottery_broadcast_enabled"
	KeyWithAds          = "lottery_broadcast_with_ads"
	KeyGames            = "lottery_broadcast_games"
)

// SchemaStatements are idempotent DDL statements (indexes are declared inline).
var SchemaStatements = []string{
	"CREATE TABLE IF NOT EXISTS `" + TableResults + "` (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT," +
		"`source` varchar(32) NOT NULL DEFAULT ''," +
		"`game_code` varchar(32) NOT NULL," +
		"`game_name` varchar(64) NOT NULL DEFAULT ''," +
		"`issue` varchar(64) NOT NULL," +
		"`draw_time` varchar(64) NOT NULL DEFAULT ''," +
		"`primary_numbers` varchar(255) NOT NULL DEFAULT ''," +
		"`secondary_numbers` varchar(64) NOT NULL DEFAULT ''," +
		"`zodiac` varchar(64) NOT NULL DEFAULT ''," +
		"`wave` varchar(128) NOT NULL DEFAULT ''," +
		"`detail_url` varchar(512) NOT NULL DEFAULT ''," +
		"`fetched_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
		"`created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
		"PRIMARY KEY (`id`)," +
		"UNIQUE KEY `uk_lb_results_game_issue` (`game_code`,`issue`)," +
		"KEY `idx_lb_results_game_id` (`game_code`,`id`)" +
		") ENGINE=InnoDB",
	"CREATE TABLE IF NOT EXISTS `" + TableSubscriptions + "` (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT," +
		"`chat_id` varchar(64) NOT NULL," +
		"`selector` varchar(32) NOT NULL," +
		"`created_by` varchar(64) NOT NULL DEFAULT ''," +
		"`is_enabled` tinyint(1) NOT NULL DEFAULT 1," +
		"`created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
		"PRIMARY KEY (`id`)," +
		"UNIQUE KEY `uk_lb_subs_chat_selector` (`chat_id`,`selector`)," +
		"KEY `idx_lb_subs_selector` (`selector`,`is_enabled`)" +
		") ENGINE=InnoDB",
	"CREATE TABLE IF NOT EXISTS `" + TableSourceStatus + "` (" +
		"`source` varchar(32) NOT NULL," +
		"`last_checked_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
		"`last_success_at` datetime(3) NULL DEFAULT NULL," +
		"`last_error` varchar(1000) NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`source`)" +
		") ENGINE=InnoDB",
	"CREATE TABLE IF NOT EXISTS `" + TableOutbox + "` (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT," +
		"`chat_id` varchar(64) NOT NULL," +
		"`game_code` varchar(32) NOT NULL," +
		"`issue` varchar(64) NOT NULL," +
		"`body` text NOT NULL," +
		"`status` varchar(16) NOT NULL DEFAULT 'pending'," +
		"`attempts` int NOT NULL DEFAULT 0," +
		"`last_error` varchar(500) NOT NULL DEFAULT ''," +
		"`message_id` bigint NOT NULL DEFAULT 0," +
		"`pinned` tinyint(1) NOT NULL DEFAULT 0," +
		"`created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
		"`sent_at` datetime(3) NULL DEFAULT NULL," +
		"PRIMARY KEY (`id`)," +
		"UNIQUE KEY `uk_lb_outbox_chat_game_issue` (`chat_id`,`game_code`,`issue`)," +
		"KEY `idx_lb_outbox_status` (`status`,`id`)" +
		") ENGINE=InnoDB",
}

// EnsureSchema creates the lottery broadcast tables if missing. Safe on every startup.
func EnsureSchema(db *gorm.DB) error {
	quiet := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	for _, stmt := range SchemaStatements {
		if err := quiet.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// ResultRow is a stored draw (explicit column tags; see lessons on GORM naming).
type ResultRow struct {
	ID               uint64    `gorm:"column:id"`
	Source           string    `gorm:"column:source"`
	GameCode         string    `gorm:"column:game_code"`
	GameName         string    `gorm:"column:game_name"`
	Issue            string    `gorm:"column:issue"`
	DrawTime         string    `gorm:"column:draw_time"`
	PrimaryNumbers   string    `gorm:"column:primary_numbers"`
	SecondaryNumbers string    `gorm:"column:secondary_numbers"`
	Zodiac           string    `gorm:"column:zodiac"`
	Wave             string    `gorm:"column:wave"`
	DetailURL        string    `gorm:"column:detail_url"`
	FetchedAt        time.Time `gorm:"column:fetched_at"`
}

// TableName implements gorm's tabler.
func (ResultRow) TableName() string { return TableResults }

// ToResult converts a stored row back to a Result.
func (r ResultRow) ToResult() Result {
	return Result{Source: r.Source, GameCode: r.GameCode, GameName: r.GameName, Issue: r.Issue,
		DrawTime: r.DrawTime, Primary: strings.Fields(r.PrimaryNumbers), Secondary: strings.Fields(r.SecondaryNumbers),
		DetailURL: r.DetailURL, Zodiac: strings.Fields(r.Zodiac), Wave: strings.Fields(r.Wave)}
}

// Subscription is one group subscription row.
type Subscription struct {
	ID         uint64    `gorm:"column:id" json:"id"`
	ChatID     string    `gorm:"column:chat_id" json:"chat_id"`
	Selector   string    `gorm:"column:selector" json:"selector"`
	CreatedBy  string    `gorm:"column:created_by" json:"created_by"`
	IsEnabled  int       `gorm:"column:is_enabled" json:"is_enabled"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	GroupTitle string    `gorm:"-" json:"group_title"`
}

// SourceStatus is one data-source health row.
type SourceStatus struct {
	Source        string       `gorm:"column:source" json:"source"`
	LastCheckedAt time.Time    `gorm:"column:last_checked_at" json:"last_checked_at"`
	LastSuccessAt sql.NullTime `gorm:"column:last_success_at" json:"-"`
	LastError     string       `gorm:"column:last_error" json:"last_error"`
}

// OutboxItem is one queued broadcast.
type OutboxItem struct {
	ID        uint64       `gorm:"column:id" json:"id"`
	ChatID    string       `gorm:"column:chat_id" json:"chat_id"`
	GameCode  string       `gorm:"column:game_code" json:"game_code"`
	Issue     string       `gorm:"column:issue" json:"issue"`
	Body      string       `gorm:"column:body" json:"-"`
	Status    string       `gorm:"column:status" json:"status"`
	Attempts  int          `gorm:"column:attempts" json:"attempts"`
	LastError string       `gorm:"column:last_error" json:"last_error"`
	MessageID int64        `gorm:"column:message_id" json:"message_id"`
	Pinned    int          `gorm:"column:pinned" json:"pinned"`
	CreatedAt time.Time    `gorm:"column:created_at" json:"created_at"`
	SentAt    sql.NullTime `gorm:"column:sent_at" json:"-"`
}

// Store is the MySQL implementation of Repo plus admin/bot queries.
type Store struct{ DB *gorm.DB }

// NewStore wraps a gorm DB.
func NewStore(db *gorm.DB) *Store { return &Store{DB: db} }

var _ Repo = (*Store)(nil)

// ---- results ----

func (s *Store) recentRows(code string, limit int) ([]ResultRow, error) {
	var rows []ResultRow
	err := s.DB.Raw("SELECT * FROM `"+TableResults+"` WHERE game_code = ? ORDER BY id DESC LIMIT ?", code, limit).Scan(&rows).Error
	return rows, err
}

func sortRowsByIssueDesc(rows []ResultRow) {
	sort.SliceStable(rows, func(i, j int) bool { return CompareIssues(rows[i].Issue, rows[j].Issue) > 0 })
}

// LatestIssue returns the highest stored issue for a game ("" when none).
func (s *Store) LatestIssue(code string) (string, error) {
	row, ok, err := s.LatestResult(code)
	if err != nil || !ok {
		return "", err
	}
	return row.Issue, nil
}

// LatestResult returns the stored row with the highest issue.
func (s *Store) LatestResult(code string) (ResultRow, bool, error) {
	rows, err := s.recentRows(code, 50)
	if err != nil || len(rows) == 0 {
		return ResultRow{}, false, err
	}
	sortRowsByIssueDesc(rows)
	return rows[0], true, nil
}

// LatestResults returns the latest row per code (GameOrder order, missing skipped).
func (s *Store) LatestResults(codes []string) ([]ResultRow, error) {
	if len(codes) == 0 {
		codes = GameOrder
	}
	var out []ResultRow
	for _, code := range codes {
		row, ok, err := s.LatestResult(code)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, row)
		}
	}
	return out, nil
}

// History returns up to limit stored rows, highest issue first.
func (s *Store) History(code string, limit int) ([]ResultRow, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, err := s.recentRows(code, 300)
	if err != nil {
		return nil, err
	}
	sortRowsByIssueDesc(rows)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

// SaveResult upserts (game_code, issue); inserted reports a brand-new row.
func (s *Store) SaveResult(r Result) (bool, error) {
	res := s.DB.Exec("INSERT INTO `"+TableResults+"` (source, game_code, game_name, issue, draw_time, primary_numbers, secondary_numbers, zodiac, wave, detail_url, fetched_at, created_at) "+
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(3), NOW(3)) "+
		"ON DUPLICATE KEY UPDATE game_name=VALUES(game_name), draw_time=VALUES(draw_time), primary_numbers=VALUES(primary_numbers), "+
		"secondary_numbers=VALUES(secondary_numbers), zodiac=VALUES(zodiac), wave=VALUES(wave), detail_url=VALUES(detail_url), fetched_at=VALUES(fetched_at)",
		trunc(r.Source, 32), r.GameCode, trunc(r.GameName, 64), trunc(r.Issue, 64), trunc(r.DrawTime, 64),
		trunc(strings.Join(r.Primary, " "), 255), trunc(strings.Join(r.Secondary, " "), 64),
		trunc(strings.Join(r.Zodiac, " "), 64), trunc(strings.Join(r.Wave, " "), 128), trunc(r.DetailURL, 512))
	if res.Error != nil {
		return false, res.Error
	}
	// MySQL: 1 = inserted, 2 = updated, 0 = unchanged.
	return res.RowsAffected == 1, nil
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// ---- subscriptions ----

// AddSubscription mirrors add_lottery_subscription (upsert, re-enable).
func (s *Store) AddSubscription(chatID, selector, createdBy string) error {
	if !IsSelector(selector) {
		return fmt.Errorf("未知彩种：%s", selector)
	}
	return s.DB.Exec("INSERT INTO `"+TableSubscriptions+"` (chat_id, selector, created_by, is_enabled, created_at) VALUES (?, ?, ?, 1, NOW(3)) "+
		"ON DUPLICATE KEY UPDATE created_by=VALUES(created_by), is_enabled=1", chatID, selector, createdBy).Error
}

// RemoveSubscription mirrors remove_lottery_subscription ("" selector = all of the chat).
func (s *Store) RemoveSubscription(chatID, selector string) (int64, error) {
	var res *gorm.DB
	if selector == "" {
		res = s.DB.Exec("DELETE FROM `"+TableSubscriptions+"` WHERE chat_id = ?", chatID)
	} else {
		res = s.DB.Exec("DELETE FROM `"+TableSubscriptions+"` WHERE chat_id = ? AND selector = ?", chatID, selector)
	}
	return res.RowsAffected, res.Error
}

// Subscriptions lists subscriptions (chatID "" = all, newest first) with group titles.
func (s *Store) Subscriptions(chatID string, limit int) ([]Subscription, error) {
	if limit <= 0 {
		limit = 300
	}
	var rows []Subscription
	// Titles are looked up separately (no cross-table JOIN) so differing table
	// collations on existing databases can never break this query.
	q := "SELECT id, chat_id, selector, created_by, is_enabled, created_at FROM `" + TableSubscriptions + "` "
	var err error
	if chatID != "" {
		err = s.DB.Raw(q+"WHERE chat_id = ? ORDER BY selector", chatID).Scan(&rows).Error
	} else {
		err = s.DB.Raw(q+"ORDER BY created_at DESC, id DESC LIMIT ?", limit).Scan(&rows).Error
	}
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ChatID)
	}
	var groups []domain.BotGroupSettings
	if err := s.DB.Select("chat_id", "title").Where("chat_id IN ?", ids).Find(&groups).Error; err == nil {
		titles := map[string]string{}
		for _, g := range groups {
			titles[g.ChatID] = g.Title
		}
		for i := range rows {
			rows[i].GroupTitle = titles[rows[i].ChatID]
		}
	}
	return rows, nil
}

// SubscribedCodesForChat expands a chat's selectors into game codes (GameOrder order).
func (s *Store) SubscribedCodesForChat(chatID string) ([]string, error) {
	var selectors []string
	if err := s.DB.Raw("SELECT selector FROM `"+TableSubscriptions+"` WHERE chat_id = ? AND is_enabled = 1", chatID).Scan(&selectors).Error; err != nil {
		return nil, err
	}
	return ExpandSelectors(selectors), nil
}

// SubscribedCodes returns every game that has at least one enabled subscriber.
func (s *Store) SubscribedCodes() ([]string, error) {
	var selectors []string
	if err := s.DB.Raw("SELECT DISTINCT selector FROM `" + TableSubscriptions + "` WHERE is_enabled = 1").Scan(&selectors).Error; err != nil {
		return nil, err
	}
	return ExpandSelectors(selectors), nil
}

// ExpandSelectors turns selectors into a de-duplicated, ordered code list.
func ExpandSelectors(selectors []string) []string {
	set := map[string]bool{}
	for _, sel := range selectors {
		for _, code := range CodesForSelector(sel) {
			set[code] = true
		}
	}
	var out []string
	for _, code := range GameOrder {
		if set[code] {
			out = append(out, code)
		}
	}
	return out
}

// SubscriberChatIDs mirrors lottery_subscriber_chat_ids.
func (s *Store) SubscriberChatIDs(code string) ([]string, error) {
	game, ok := Games[code]
	if !ok {
		return nil, nil
	}
	marksix := ""
	if IsMarkSix(code) {
		marksix = "marksix"
	}
	var ids []string
	err := s.DB.Raw("SELECT DISTINCT chat_id FROM `"+TableSubscriptions+"` WHERE is_enabled = 1 AND selector IN ('all', ?, ?, ?)",
		game.Source, code, marksix).Scan(&ids).Error
	return ids, err
}

// SetSubscribedCodes replaces a chat's subscriptions with exactly these codes
// (all 11 → single "all" row), like the Python toggle keyboard does.
func (s *Store) SetSubscribedCodes(chatID string, codes []string, createdBy string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM `"+TableSubscriptions+"` WHERE chat_id = ?", chatID).Error; err != nil {
			return err
		}
		codes = ExpandSelectors(codes)
		if len(codes) == len(GameOrder) {
			codes = []string{"all"}
		}
		for _, code := range codes {
			if err := tx.Exec("INSERT INTO `"+TableSubscriptions+"` (chat_id, selector, created_by, is_enabled, created_at) VALUES (?, ?, ?, 1, NOW(3)) "+
				"ON DUPLICATE KEY UPDATE created_by=VALUES(created_by), is_enabled=1", chatID, code, createdBy).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ---- source status ----

// SetSourceStatus mirrors set_lottery_source_status.
func (s *Store) SetSourceStatus(source string, ok bool, errText string) error {
	okInt := 0
	if ok {
		okInt = 1
	}
	return s.DB.Exec("INSERT INTO `"+TableSourceStatus+"` (source, last_checked_at, last_success_at, last_error) "+
		"VALUES (?, NOW(3), CASE WHEN ? = 1 THEN NOW(3) ELSE NULL END, ?) "+
		"ON DUPLICATE KEY UPDATE last_checked_at = NOW(3), "+
		"last_success_at = CASE WHEN ? = 1 THEN NOW(3) ELSE last_success_at END, last_error = VALUES(last_error)",
		source, okInt, trunc(errText, 1000), okInt).Error
}

// SourceStatuses lists data-source health.
func (s *Store) SourceStatuses() ([]SourceStatus, error) {
	var rows []SourceStatus
	err := s.DB.Raw("SELECT source, last_checked_at, last_success_at, last_error FROM `" + TableSourceStatus + "` ORDER BY source").Scan(&rows).Error
	return rows, err
}

// ---- outbox ----

// Enqueue inserts one pending broadcast; duplicates (same chat+game+issue) are ignored.
func (s *Store) Enqueue(chatID, code, issue, body string) (bool, error) {
	res := s.DB.Exec("INSERT INTO `"+TableOutbox+"` (chat_id, game_code, issue, body, status, created_at) VALUES (?, ?, ?, ?, 'pending', NOW(3)) "+
		"ON DUPLICATE KEY UPDATE id = id", chatID, code, issue, trunc(body, 4000))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

// PendingOutbox mirrors pending_outbox.
func (s *Store) PendingOutbox(limit int) ([]OutboxItem, error) {
	if limit <= 0 {
		limit = 15
	}
	var rows []OutboxItem
	err := s.DB.Raw("SELECT * FROM `"+TableOutbox+"` WHERE status = 'pending' ORDER BY id LIMIT ?", limit).Scan(&rows).Error
	return rows, err
}

// FinishOutbox records a send attempt. On failure the item stays pending until
// maxAttempts is reached, then becomes 'failed'.
func (s *Store) FinishOutbox(id uint64, success bool, messageID int64, pinned bool, errText string, maxAttempts int) error {
	if success {
		p := 0
		if pinned {
			p = 1
		}
		return s.DB.Exec("UPDATE `"+TableOutbox+"` SET status='sent', attempts=attempts+1, last_error=?, message_id=?, pinned=?, sent_at=NOW(3) WHERE id=?",
			trunc(errText, 500), messageID, p, id).Error
	}
	return s.DB.Exec("UPDATE `"+TableOutbox+"` SET attempts=attempts+1, last_error=?, status=CASE WHEN attempts >= ? THEN 'failed' ELSE 'pending' END WHERE id=?",
		trunc(errText, 500), maxAttempts, id).Error
}

// RecentOutbox lists recent broadcasts for the admin page.
func (s *Store) RecentOutbox(limit int) ([]OutboxItem, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []OutboxItem
	err := s.DB.Raw("SELECT id, chat_id, game_code, issue, status, attempts, last_error, message_id, pinned, created_at, sent_at FROM `"+TableOutbox+"` ORDER BY id DESC LIMIT ?", limit).Scan(&rows).Error
	return rows, err
}

// ---- switches ----

func (s *Store) readConfig(keys []string) (map[string]string, error) {
	var rows []domain.SystemConfig
	if err := s.DB.Where("`key` IN ?", keys).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

func (s *Store) writeConfig(key, value string) error {
	cfg := domain.SystemConfig{Key: key}
	return s.DB.Where("`key` = ?", key).Assign(domain.SystemConfig{Value: value}).FirstOrCreate(&cfg).Error
}

func flag(m map[string]string, key string) bool {
	v, ok := m[key]
	if !ok {
		return true // default on
	}
	return strings.TrimSpace(v) != "0"
}

// LoadSwitches reads the admin switches (all default on).
func (s *Store) LoadSwitches() (Switches, error) {
	m, err := s.readConfig([]string{KeyQueryEnabled, KeyBroadcastEnabled, KeyWithAds, KeyGames})
	if err != nil {
		return DefaultSwitches(), err
	}
	sw := Switches{QueryEnabled: flag(m, KeyQueryEnabled), BroadcastEnabled: flag(m, KeyBroadcastEnabled),
		WithAds: flag(m, KeyWithAds), Games: map[string]bool{}}
	if raw := strings.TrimSpace(m[KeyGames]); raw != "" {
		var games map[string]bool
		if json.Unmarshal([]byte(raw), &games) == nil {
			for code, on := range games {
				if _, ok := Games[code]; ok {
					sw.Games[code] = on
				}
			}
		}
	}
	return sw, nil
}

// SaveSwitches persists the admin switches.
func (s *Store) SaveSwitches(sw Switches) error {
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	games := map[string]bool{}
	for _, code := range GameOrder {
		games[code] = sw.GameEnabled(code)
	}
	raw, err := json.Marshal(games)
	if err != nil {
		return err
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		st := &Store{DB: tx}
		for k, v := range map[string]string{KeyQueryEnabled: b(sw.QueryEnabled), KeyBroadcastEnabled: b(sw.BroadcastEnabled),
			KeyWithAds: b(sw.WithAds), KeyGames: string(raw)} {
			if err := st.writeConfig(k, v); err != nil {
				return err
			}
		}
		return nil
	})
}

// ErrUnknownGame is returned for invalid codes.
var ErrUnknownGame = errors.New("未知彩种")
