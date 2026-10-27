package aitron

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OutboxDDL returns the idempotent DDL of one kind's outbox table. It holds
// one row per (chat, period block): the unique key is what prevents sending the
// same period twice, also across bot restarts.
func OutboxDDL(table string) string {
	return "CREATE TABLE IF NOT EXISTS `" + table + "` (" +
		"`id` bigint unsigned NOT NULL AUTO_INCREMENT," +
		"`chat_id` varchar(64) NOT NULL," +
		"`block` bigint NOT NULL," +
		"`body` text NOT NULL," +
		"`status` varchar(16) NOT NULL DEFAULT 'pending'," +
		"`attempts` int NOT NULL DEFAULT 0," +
		"`last_error` varchar(500) NOT NULL DEFAULT ''," +
		"`created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
		"`sent_at` datetime(3) NULL DEFAULT NULL," +
		"PRIMARY KEY (`id`)," +
		"UNIQUE KEY `uk_tron_chat_block` (`chat_id`,`block`)," +
		"KEY `idx_tron_status` (`status`,`id`)" +
		") ENGINE=InnoDB"
}

// EnsureSchema creates the kind's outbox table if missing. Safe on every startup.
func EnsureSchema(db *gorm.DB, kind Kind) error {
	quiet := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	return quiet.Exec(OutboxDDL(kind.Table)).Error
}

// OutboxItem is one queued broadcast.
type OutboxItem struct {
	ID        uint64    `gorm:"column:id"`
	ChatID    string    `gorm:"column:chat_id"`
	Block     int64     `gorm:"column:block"`
	Body      string    `gorm:"column:body"`
	Attempts  int       `gorm:"column:attempts"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

// MySQLStore is the production Store for one broadcast kind.
type MySQLStore struct {
	DB    *gorm.DB
	Table string                             // outbox table (Kind.Table)
	List  func() ([]string, error)           // chats with this broadcast enabled
	Set   func(chatID string, on bool) error // per-group flag writer
}

func (s *MySQLStore) EnabledChats() ([]string, error) { return s.List() }

// Enqueue inserts a pending row; false means this (chat, block) already exists.
func (s *MySQLStore) Enqueue(chatID string, block int64, body string) (bool, error) {
	res := s.DB.Exec("INSERT INTO `"+s.Table+"` (chat_id, block, body, status, created_at) VALUES (?, ?, ?, 'pending', NOW(3)) "+
		"ON DUPLICATE KEY UPDATE id = id", chatID, block, body)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (s *MySQLStore) Pending(limit int) ([]OutboxItem, error) {
	var rows []OutboxItem
	err := s.DB.Raw("SELECT id, chat_id, block, body, attempts, created_at FROM `"+s.Table+"` WHERE status = 'pending' ORDER BY block, id LIMIT ?", limit).Scan(&rows).Error
	return rows, err
}

func (s *MySQLStore) MarkSent(id uint64) error {
	return s.DB.Exec("UPDATE `"+s.Table+"` SET status='sent', attempts=attempts+1, last_error='', sent_at=NOW(3) WHERE id=?", id).Error
}

// MarkAttemptFailed counts a failed attempt; after maxAttempts the row becomes 'failed'.
func (s *MySQLStore) MarkAttemptFailed(id uint64, errText string, maxAttempts int) error {
	return s.DB.Exec("UPDATE `"+s.Table+"` SET attempts=attempts+1, last_error=?, status=CASE WHEN attempts >= ? THEN 'failed' ELSE 'pending' END WHERE id=?",
		trunc(errText, 500), maxAttempts, id).Error
}

// MarkFailed gives up on a row immediately (stale, expired, dead group).
func (s *MySQLStore) MarkFailed(id uint64, errText string) error {
	return s.DB.Exec("UPDATE `"+s.Table+"` SET status='failed', last_error=? WHERE id=? AND status='pending'", trunc(errText, 500), id).Error
}

// DisableChat switches the group's broadcast flag off and voids its pending rows.
func (s *MySQLStore) DisableChat(chatID string) error {
	if err := s.Set(chatID, false); err != nil {
		return err
	}
	return s.DB.Exec("UPDATE `"+s.Table+"` SET status='failed', last_error='群不可达，已关闭该播报' WHERE chat_id = ? AND status = 'pending'", chatID).Error
}

// Prune deletes finished rows older than keep.
func (s *MySQLStore) Prune(keep time.Duration) error {
	return s.DB.Exec("DELETE FROM `"+s.Table+"` WHERE status <> 'pending' AND created_at < ?", time.Now().Add(-keep)).Error
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
