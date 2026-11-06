// Package botclone stores cloned bots (克隆机器人) owned by the mother safew-bot
// process. Tokens are encrypted at rest (AES-GCM) and never logged.
package botclone

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	Table = "bot_clones"

	StatusPending  = "pending"
	StatusActive   = "active"
	StatusDisabled = "disabled"

	// DefaultAPIBase is the official Telegram Bot API (new clones default to it).
	DefaultAPIBase = "https://api.telegram.org"
)

// DDL is idempotent.
const DDL = "CREATE TABLE IF NOT EXISTS `" + Table + "` (" +
	"`id` bigint unsigned NOT NULL AUTO_INCREMENT," +
	"`parent_bot_id` varchar(64) NOT NULL DEFAULT ''," +
	"`bot_id` varchar(64) NOT NULL," +
	"`username` varchar(128) NOT NULL DEFAULT ''," +
	"`display_name` varchar(255) NOT NULL DEFAULT ''," +
	"`api_base` varchar(255) NOT NULL DEFAULT ''," +
	"`encrypted_token` text NOT NULL," +
	"`status` varchar(16) NOT NULL DEFAULT 'pending'," +
	"`requested_by` varchar(64) NOT NULL DEFAULT ''," +
	"`requested_by_username` varchar(128) NOT NULL DEFAULT ''," +
	"`last_error` varchar(500) NOT NULL DEFAULT ''," +
	"`created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
	"`updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)," +
	"PRIMARY KEY (`id`)," +
	"UNIQUE KEY `uk_bot_clones_bot_id` (`bot_id`)," +
	"KEY `idx_bot_clones_status` (`status`)" +
	") ENGINE=InnoDB"

// EnsureSchema creates bot_clones if missing.
func EnsureSchema(db *gorm.DB) error {
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Exec(DDL).Error
}

// Clone is one bot_clones row.
type Clone struct {
	ID                  uint64    `gorm:"column:id"`
	ParentBotID         string    `gorm:"column:parent_bot_id"`
	BotID               string    `gorm:"column:bot_id"`
	Username            string    `gorm:"column:username"`
	DisplayName         string    `gorm:"column:display_name"`
	APIBase             string    `gorm:"column:api_base"`
	EncryptedToken      string    `gorm:"column:encrypted_token"`
	Status              string    `gorm:"column:status"`
	RequestedBy         string    `gorm:"column:requested_by"`
	RequestedByUsername string    `gorm:"column:requested_by_username"`
	LastError           string    `gorm:"column:last_error"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at"`
}

// TableName implements gorm's tabler.
func (Clone) TableName() string { return Table }

// ---- token encryption ----

// Cipher encrypts tokens with AES-256-GCM.
type Cipher struct{ aead cipher.AEAD }

// NewCipher derives a 256-bit key from secret (CLONE_CIPHER_KEY, or the mother
// token when that is unset).
func NewCipher(secret string) (*Cipher, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("克隆加密密钥为空")
	}
	key := sha256.Sum256([]byte("safe-bot-clone:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt returns base64(nonce||ciphertext).
func (c *Cipher) Encrypt(plain string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := c.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt.
func (c *Cipher) Decrypt(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	n := c.aead.NonceSize()
	if len(raw) < n {
		return "", errors.New("密文过短")
	}
	plain, err := c.aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", errors.New("解密失败（密钥已变更？）")
	}
	return string(plain), nil
}

// ---- token helpers ----

var tokenRe = regexp.MustCompile(`^[0-9]{3,20}:[A-Za-z0-9_-]{20,}$`)

// LooksLikeToken reports whether s has the BotFather token shape.
func LooksLikeToken(s string) bool { return tokenRe.MatchString(strings.TrimSpace(s)) }

// ---- store ----

// Store persists clones.
type Store struct {
	DB     *gorm.DB
	Cipher *Cipher
}

func (s *Store) quiet() *gorm.DB {
	return s.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

// Get returns the clone with botID (found=false when missing).
func (s *Store) Get(botID string) (Clone, bool, error) {
	var c Clone
	err := s.quiet().Where("bot_id = ?", botID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Clone{}, false, nil
	}
	return c, err == nil, err
}

// List returns all clones (newest first).
func (s *Store) List() ([]Clone, error) {
	var rows []Clone
	err := s.DB.Order("id DESC").Find(&rows).Error
	return rows, err
}

// ListActive returns clones to start on boot.
func (s *Store) ListActive() ([]Clone, error) {
	var rows []Clone
	err := s.DB.Where("status = ?", StatusActive).Order("id").Find(&rows).Error
	return rows, err
}

// CountActiveBy counts a requester's active clones.
func (s *Store) CountActiveBy(userID string) (int64, error) {
	var n int64
	err := s.DB.Model(&Clone{}).Where("requested_by = ? AND status = ?", userID, StatusActive).Count(&n).Error
	return n, err
}

// SetStatus updates status / last_error.
func (s *Store) SetStatus(botID, status, lastError string) error {
	if len([]rune(lastError)) > 500 {
		lastError = string([]rune(lastError)[:500])
	}
	return s.DB.Model(&Clone{}).Where("bot_id = ?", botID).
		Updates(map[string]any{"status": status, "last_error": lastError, "updated_at": time.Now()}).Error
}

// Delete removes the clone row (its group data is kept under its bot_id).
func (s *Store) Delete(botID string) error {
	return s.DB.Where("bot_id = ?", botID).Delete(&Clone{}).Error
}

// Token decrypts a clone's token.
func (s *Store) Token(c Clone) (string, error) { return s.Cipher.Decrypt(c.EncryptedToken) }

// Save inserts or re-activates (same bot_id) a clone with an encrypted token.
func (s *Store) Save(c Clone, token string) (Clone, error) {
	enc, err := s.Cipher.Encrypt(token)
	if err != nil {
		return Clone{}, err
	}
	c.EncryptedToken = enc
	now := time.Now()
	c.UpdatedAt = now
	existing, found, err := s.Get(c.BotID)
	if err != nil {
		return Clone{}, err
	}
	if found {
		c.ID = existing.ID
		c.CreatedAt = existing.CreatedAt
		return c, s.DB.Save(&c).Error
	}
	c.CreatedAt = now
	return c, s.DB.Create(&c).Error
}

// ---- registration ----

// BotIdentity is the getMe answer.
type BotIdentity struct {
	ID        string
	Username  string
	FirstName string
}

// GetMeFunc validates a token against apiBase.
type GetMeFunc func(ctx context.Context, apiBase, token string) (BotIdentity, error)

// Saver is the persistence Register needs (Store implements it).
type Saver interface {
	Get(botID string) (Clone, bool, error)
	Save(c Clone, token string) (Clone, error)
}

// Request is a user's clone submission.
type Request struct {
	ParentBotID     string
	MotherBotID     string
	IsMotherID      func(botID string) bool // any mother (SafeW / Telegram)
	APIBase         string
	Token           string
	RequestedBy     string
	RequestedByName string
	IsReserved      func(botID string) bool // mother / already running instance
}

// Errors returned by Register (user-facing, never contain the token).
var (
	ErrBadToken      = errors.New("Token 无效：请到 @BotFather 复制完整的 Bot Token 再发送")
	ErrIsMother      = errors.New("不能克隆母机器人自己")
	ErrAlreadyActive = errors.New("这个机器人已经在运行中了，无需重复克隆")
)

// Register validates req.Token with getMe and stores it encrypted as active.
// Nothing is written when validation fails.
func Register(ctx context.Context, store Saver, getMe GetMeFunc, req Request) (Clone, error) {
	token := strings.TrimSpace(req.Token)
	if !LooksLikeToken(token) {
		return Clone{}, ErrBadToken
	}
	base := strings.TrimRight(strings.TrimSpace(req.APIBase), "/")
	if base == "" {
		base = DefaultAPIBase
	}
	me, err := getMe(ctx, base, token)
	if err != nil || strings.TrimSpace(me.ID) == "" {
		return Clone{}, ErrBadToken
	}
	if (req.MotherBotID != "" && me.ID == req.MotherBotID) || (req.IsMotherID != nil && req.IsMotherID(me.ID)) {
		return Clone{}, ErrIsMother
	}
	if req.IsReserved != nil && req.IsReserved(me.ID) {
		return Clone{}, ErrAlreadyActive
	}
	if existing, found, err := store.Get(me.ID); err != nil {
		return Clone{}, err
	} else if found && existing.Status == StatusActive {
		return Clone{}, ErrAlreadyActive
	}
	c := Clone{
		ParentBotID:         req.ParentBotID,
		BotID:               me.ID,
		Username:            me.Username,
		DisplayName:         me.FirstName,
		APIBase:             base,
		Status:              StatusActive,
		RequestedBy:         req.RequestedBy,
		RequestedByUsername: req.RequestedByName,
	}
	saved, err := store.Save(c, token)
	if err != nil {
		return Clone{}, fmt.Errorf("保存克隆失败: %w", err)
	}
	return saved, nil
}
