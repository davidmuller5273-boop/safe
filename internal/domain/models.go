package domain

import "time"

type Admin struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name" gorm:"size:64"`
	RoleID       uint      `json:"role_id"`
	Role         Role      `json:"role"`
	Enabled      bool      `json:"enabled" gorm:"default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type Role struct {
	ID          uint         `json:"id" gorm:"primaryKey"`
	Name        string       `json:"name" gorm:"uniqueIndex;size:64"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions" gorm:"many2many:role_permissions;"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}
type Permission struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Code        string `json:"code" gorm:"uniqueIndex;size:100"`
	Name        string `json:"name" gorm:"size:100"`
	Description string `json:"description"`
}

// SystemConfig stores server-side system settings. Sensitive values must not
// be exposed directly by API presenters.
type SystemConfig struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"uniqueIndex;size:100;not null"`
	Value     string    `json:"-" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LotteryType describes a lottery's draw frequency and daily schedule.
type LotteryType struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Name            string    `json:"name" gorm:"uniqueIndex;size:100;not null"`
	Symbol          string    `json:"symbol" gorm:"size:64;not null"`
	SecondsPerIssue uint      `json:"seconds_per_issue" gorm:"not null"`
	IssuesPerDay    uint      `json:"issues_per_day" gorm:"not null"`
	DrawsAllDay     bool      `json:"draws_all_day" gorm:"not null;default:false"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DrawRecord stores one draw and the scheduling information for the next draw.
// Time values are Unix timestamps in seconds.
type DrawRecord struct {
	ID                uint        `json:"id" gorm:"primaryKey"`
	LotteryTypeID     uint        `json:"lottery_type_id" gorm:"not null;uniqueIndex:idx_draw_records_lottery_issue;index"`
	LotteryType       LotteryType `json:"lottery_type" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	IssueNumber       string      `json:"issue_number" gorm:"size:64;not null;uniqueIndex:idx_draw_records_lottery_issue"`
	DrawResult        string      `json:"draw_result" gorm:"type:text;not null"`
	DrawTimestamp     int64       `json:"draw_timestamp" gorm:"not null;index"`
	NextDrawTimestamp int64       `json:"next_draw_timestamp" gorm:"not null;index"`
	NextIssueNumber   string      `json:"next_issue_number" gorm:"size:64;not null"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

// HotNumberPrediction stores the ordered seven-number state used to predict
// an issue and its evaluation after the draw completes.
type HotNumberPrediction struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	LotteryTypeID   uint       `json:"lottery_type_id" gorm:"not null;uniqueIndex:idx_hot_predictions_lottery_issue;index"`
	IssueNumber     string     `json:"issue_number" gorm:"size:64;not null;uniqueIndex:idx_hot_predictions_lottery_issue"`
	HotNumbers      string     `json:"hot_numbers" gorm:"type:text;not null"`
	Prediction      string     `json:"prediction" gorm:"size:16;not null"`
	ActualHotNumber string     `json:"actual_hot_number" gorm:"size:2"`
	Correct         *bool      `json:"correct"`
	EvaluatedAt     *time.Time `json:"evaluated_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}


// BotAdmin grants global bot-admin privileges (not developer).
// Only developers can add/remove admins. Persisted in MySQL.
type BotAdmin struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id" gorm:"uniqueIndex;size:64;not null"`
	Note      string    `json:"note" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BotGroupAdmin grants group-scoped bot permissions for a specific chat/group.
type BotGroupAdmin struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id" gorm:"uniqueIndex:idx_bot_group_admin;size:64;not null"`
	ChatID    string    `json:"chat_id" gorm:"uniqueIndex:idx_bot_group_admin;size:64;not null"`
	Note      string    `json:"note" gorm:"size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BotGroupAd stores optional per-group prefix/suffix ads.
// Empty fields fall back to global ads in system_configs.
type BotGroupAd struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ChatID    string    `json:"chat_id" gorm:"uniqueIndex;size:64;not null"`
	PrefixAd  string    `json:"prefix_ad" gorm:"type:text"`
	SuffixAd  string    `json:"suffix_ad" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BotGroupSettings tracks per-group bot behaviour.
// PushEnabled defaults to false when the bot first sees a group.
type BotGroupSettings struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	ChatID      string    `json:"chat_id" gorm:"uniqueIndex;size:64;not null"`
	PushEnabled bool      `json:"push_enabled" gorm:"not null;default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

