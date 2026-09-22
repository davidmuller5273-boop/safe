package botperm

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"gorm.io/gorm"
)

const (
	RoleDeveloper  = "developer"
	RoleAdmin      = "admin" // persisted key; display name = 超级管理员
	RoleGroupAdmin = "group_admin"
	RoleNone       = ""
)

func RoleLabel(role string) string {
	switch role {
	case RoleDeveloper:
		return "开发者"
	case RoleAdmin:
		return "超级管理员"
	case RoleGroupAdmin:
		return "群管理员"
	default:
		return "普通用户"
	}
}

type Store struct {
	DB               *gorm.DB
	DeveloperUserIDs map[string]struct{}
}

func NewStore(db *gorm.DB, developerIDs []string) *Store {
	m := make(map[string]struct{}, len(developerIDs))
	for _, id := range developerIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			m[id] = struct{}{}
		}
	}
	return &Store{DB: db, DeveloperUserIDs: m}
}

func (s *Store) IsDeveloper(userID string) bool {
	_, ok := s.DeveloperUserIDs[strings.TrimSpace(userID)]
	return ok
}

func (s *Store) IsAdmin(userID string) (bool, error) {
	userID = strings.TrimSpace(userID)
	if s.IsDeveloper(userID) {
		return true, nil
	}
	var count int64
	err := s.DB.Model(&domain.BotAdmin{}).Where("user_id = ?", userID).Count(&count).Error
	return count > 0, err
}

func (s *Store) IsGroupAdmin(userID, chatID string) (bool, error) {
	userID = strings.TrimSpace(userID)
	chatID = strings.TrimSpace(chatID)
	if s.IsDeveloper(userID) {
		return true, nil
	}
	ok, err := s.IsAdmin(userID)
	if err != nil || ok {
		return ok, err
	}
	var count int64
	err = s.DB.Model(&domain.BotGroupAdmin{}).Where("user_id = ? AND chat_id = ?", userID, chatID).Count(&count).Error
	return count > 0, err
}

func (s *Store) RoleInChat(userID, chatID string) (string, error) {
	userID = strings.TrimSpace(userID)
	chatID = strings.TrimSpace(chatID)
	if s.IsDeveloper(userID) {
		return RoleDeveloper, nil
	}
	var count int64
	if err := s.DB.Model(&domain.BotAdmin{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return RoleNone, err
	}
	if count > 0 {
		return RoleAdmin, nil
	}
	if err := s.DB.Model(&domain.BotGroupAdmin{}).Where("user_id = ? AND chat_id = ?", userID, chatID).Count(&count).Error; err != nil {
		return RoleNone, err
	}
	if count > 0 {
		return RoleGroupAdmin, nil
	}
	return RoleNone, nil
}

func (s *Store) CanManageAdmins(actorID string) bool {
	return s.IsDeveloper(actorID)
}

func (s *Store) CanManageGroupAdmins(actorID string) (bool, error) {
	return s.IsAdmin(actorID)
}

// CanControlSensitive: developer, 超级管理员, or group admin of that chat.
func (s *Store) CanControlSensitive(actorID, chatID string) (bool, error) {
	actorID = strings.TrimSpace(actorID)
	chatID = strings.TrimSpace(chatID)
	if s.IsDeveloper(actorID) {
		return true, nil
	}
	ok, err := s.IsAdmin(actorID)
	if err != nil || ok {
		return ok, err
	}
	var count int64
	err = s.DB.Model(&domain.BotGroupAdmin{}).Where("user_id = ? AND chat_id = ?", actorID, chatID).Count(&count).Error
	return count > 0, err
}

// CanTogglePush: developer or 超级管理员 only.
func (s *Store) CanTogglePush(actorID string) (bool, error) {
	return s.IsAdmin(actorID)
}

func (s *Store) AddAdmin(userID, note string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user_id 不能为空")
	}
	if s.IsDeveloper(userID) {
		return errors.New("该用户已是开发者，无需再设为超级管理员")
	}
	row := domain.BotAdmin{UserID: userID, Note: note}
	return s.DB.Where("user_id = ?", userID).Assign(domain.BotAdmin{Note: note}).FirstOrCreate(&row).Error
}

func (s *Store) RemoveAdmin(userID string) error {
	return s.DB.Where("user_id = ?", strings.TrimSpace(userID)).Delete(&domain.BotAdmin{}).Error
}

func (s *Store) AddGroupAdmin(userID, chatID, note string) error {
	userID = strings.TrimSpace(userID)
	chatID = strings.TrimSpace(chatID)
	if userID == "" || chatID == "" {
		return errors.New("user_id 与 chat_id 均不能为空")
	}
	row := domain.BotGroupAdmin{UserID: userID, ChatID: chatID, Note: note}
	return s.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).
		Assign(domain.BotGroupAdmin{Note: note}).FirstOrCreate(&row).Error
}

func (s *Store) RemoveGroupAdmin(userID, chatID string) error {
	return s.DB.Where("user_id = ? AND chat_id = ?", strings.TrimSpace(userID), strings.TrimSpace(chatID)).
		Delete(&domain.BotGroupAdmin{}).Error
}

func (s *Store) ListAdmins() ([]domain.BotAdmin, error) {
	var list []domain.BotAdmin
	err := s.DB.Order("id").Find(&list).Error
	return list, err
}

func (s *Store) ListGroupAdmins(chatID string) ([]domain.BotGroupAdmin, error) {
	var list []domain.BotGroupAdmin
	q := s.DB.Order("id")
	if strings.TrimSpace(chatID) != "" {
		q = q.Where("chat_id = ?", strings.TrimSpace(chatID))
	}
	err := q.Find(&list).Error
	return list, err
}

// EnsureGroup registers a group with push disabled by default.
// Uses MySQL upsert (no-op on conflict) instead of GORM clause.OnConflict
// DoNothing, which is unreliable on some MySQL/GORM combinations.
func (s *Store) EnsureGroup(chatID string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil
	}
	return s.DB.Exec(`
INSERT INTO bot_group_settings (chat_id, push_enabled, enable_6_code, enable_7_code, created_at, updated_at)
VALUES (?, 0, 0, 0, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE chat_id = chat_id
`, chatID).Error
}

func (s *Store) SetPushEnabled(chatID string, enabled bool) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return errors.New("chat_id 不能为空")
	}
	if err := s.EnsureGroup(chatID); err != nil {
		return err
	}
	return s.DB.Model(&domain.BotGroupSettings{}).Where("chat_id = ?", chatID).Update("push_enabled", enabled).Error
}

func (s *Store) IsPushEnabled(chatID string) (bool, error) {
	row, err := s.GetGroupSettings(chatID)
	if err != nil {
		return false, err
	}
	return row.PushEnabled, nil
}

func (s *Store) ListPushEnabledChatIDs() ([]string, error) {
	rows, err := s.ListPushTargets()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ChatID)
	}
	return out, nil
}

// ListPushTargets returns groups with PushEnabled=true (includes code-mode fields).
// Loads flag columns via int scan for the same reason as GetGroupSettings.
func (s *Store) ListPushTargets() ([]domain.BotGroupSettings, error) {
	type rawRow struct {
		ID          uint           `gorm:"column:id"`
		ChatID      string         `gorm:"column:chat_id"`
		Title       sql.NullString `gorm:"column:title"`
		Username    sql.NullString `gorm:"column:username"`
		ChatType    sql.NullString `gorm:"column:chat_type"`
		PushEnabled int            `gorm:"column:push_enabled"`
		Enable6Code int            `gorm:"column:enable_6_code"`
		Enable7Code int            `gorm:"column:enable_7_code"`
	}
	var raws []rawRow
	err := s.DB.Raw(`
SELECT id, chat_id, title, username, chat_type,
       push_enabled, enable_6_code, enable_7_code
FROM bot_group_settings WHERE push_enabled = 1 ORDER BY id`).Scan(&raws).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.BotGroupSettings, 0, len(raws))
	for _, r := range raws {
		out = append(out, domain.BotGroupSettings{
			ID:          r.ID,
			ChatID:      r.ChatID,
			Title:       r.Title.String,
			Username:    r.Username.String,
			ChatType:    r.ChatType.String,
			PushEnabled: r.PushEnabled != 0,
			Enable6Code: r.Enable6Code != 0,
			Enable7Code: r.Enable7Code != 0,
		})
	}
	return out, nil
}

// SetCodeMode enables/disables 6-code or 7-code predictions for a group.
// Enabling one mode turns on push and turns the other mode off so a single
// /开启6码 or /开启7码 is enough.
// Uses MySQL INSERT ... ON DUPLICATE KEY UPDATE so a missing row is created
// (does not rely on EnsureGroup / GORM OnConflict alone). Verifies with a
// raw int scan so GORM bool mapping cannot mask a failed write.
func (s *Store) SetCodeMode(chatID string, size int, enabled bool) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return errors.New("chat_id 不能为空")
	}
	if size != 6 && size != 7 {
		return errors.New("size 必须为 6 或 7")
	}
	sqlStr, err := codeModeUpsertSQL(size, enabled)
	if err != nil {
		return err
	}
	if err := s.DB.Exec(sqlStr, chatID).Error; err != nil {
		return err
	}
	e6, e7, push, err := s.scanCodeFlags(chatID)
	if err != nil {
		return fmt.Errorf("SetCodeMode: 写入后校验失败 (chat_id=%s): %w", chatID, err)
	}
	if !codeModeVerifyOK(size, enabled, e6, e7, push) {
		return fmt.Errorf("SetCodeMode: 写入后状态不符 (chat_id=%s size=%d enabled=%v enable_6=%d enable_7=%d push=%d)",
			chatID, size, enabled, e6, e7, push)
	}
	return nil
}

// codeModeUpsertSQL returns MySQL upsert SQL; the sole bind arg is chat_id.
func codeModeUpsertSQL(size int, enabled bool) (string, error) {
	switch {
	case size == 6 && enabled:
		return `
INSERT INTO bot_group_settings (chat_id, push_enabled, enable_6_code, enable_7_code, created_at, updated_at)
VALUES (?, 1, 1, 0, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  push_enabled=VALUES(push_enabled),
  enable_6_code=VALUES(enable_6_code),
  enable_7_code=VALUES(enable_7_code),
  updated_at=VALUES(updated_at)`, nil
	case size == 7 && enabled:
		return `
INSERT INTO bot_group_settings (chat_id, push_enabled, enable_6_code, enable_7_code, created_at, updated_at)
VALUES (?, 1, 0, 1, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  push_enabled=VALUES(push_enabled),
  enable_6_code=VALUES(enable_6_code),
  enable_7_code=VALUES(enable_7_code),
  updated_at=VALUES(updated_at)`, nil
	case size == 6 && !enabled:
		// Disable only flips that flag; still upsert if the row is missing.
		return `
INSERT INTO bot_group_settings (chat_id, push_enabled, enable_6_code, enable_7_code, created_at, updated_at)
VALUES (?, 0, 0, 0, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  enable_6_code=0,
  updated_at=VALUES(updated_at)`, nil
	case size == 7 && !enabled:
		return `
INSERT INTO bot_group_settings (chat_id, push_enabled, enable_6_code, enable_7_code, created_at, updated_at)
VALUES (?, 0, 0, 0, NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  enable_7_code=0,
  updated_at=VALUES(updated_at)`, nil
	default:
		return "", errors.New("size 必须为 6 或 7")
	}
}

func codeModeVerifyOK(size int, enabled bool, e6, e7, push int) bool {
	switch {
	case size == 6 && enabled:
		return e6 != 0 && e7 == 0 && push != 0
	case size == 7 && enabled:
		return e7 != 0 && e6 == 0 && push != 0
	case size == 6 && !enabled:
		return e6 == 0
	case size == 7 && !enabled:
		return e7 == 0
	default:
		return false
	}
}

func (s *Store) scanCodeFlags(chatID string) (e6, e7, push int, err error) {
	err = s.DB.Raw(
		`SELECT enable_6_code, enable_7_code, push_enabled FROM bot_group_settings WHERE chat_id=?`,
		chatID,
	).Row().Scan(&e6, &e7, &push)
	if err != nil {
		return 0, 0, 0, err
	}
	return e6, e7, push, nil
}

// IsCodeEnabled reports whether the given code size is enabled for the chat.
func (s *Store) IsCodeEnabled(chatID string, size int) (bool, error) {
	if size != 6 && size != 7 {
		return false, errors.New("size 必须为 6 或 7")
	}
	row, err := s.GetGroupSettings(chatID)
	if err != nil {
		return false, err
	}
	if size == 6 {
		return Effective6Code(row), nil
	}
	return Effective7Code(row), nil
}

// Effective7Code is true only when Enable7Code is set (no legacy push-only fallback).
func Effective7Code(row domain.BotGroupSettings) bool {
	return row.Enable7Code
}

// Effective6Code is true when Enable6Code is set.
func Effective6Code(row domain.BotGroupSettings) bool {
	return row.Enable6Code
}

// GetGroupSettings returns persisted settings for a chat, or zero value if missing.
// Flag columns are scanned as ints then mapped to bool so tinyint/GORM bool
// quirks cannot report enable_6_code=0 when the DB value is 1.
func (s *Store) GetGroupSettings(chatID string) (domain.BotGroupSettings, error) {
	chatID = strings.TrimSpace(chatID)
	var (
		row                domain.BotGroupSettings
		push, e6, e7       int
		title, user, ctype sql.NullString
		created, updated   sql.NullTime
	)
	err := s.DB.Raw(`
SELECT id, chat_id, title, username, chat_type,
       push_enabled, enable_6_code, enable_7_code, created_at, updated_at
FROM bot_group_settings WHERE chat_id=?`, chatID).Row().Scan(
		&row.ID, &row.ChatID, &title, &user, &ctype,
		&push, &e6, &e7, &created, &updated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.BotGroupSettings{ChatID: chatID}, nil
	}
	if err != nil {
		// Some drivers surface no-rows differently through GORM Raw.
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "sql: no rows in result set" {
			return domain.BotGroupSettings{ChatID: chatID}, nil
		}
		return row, err
	}
	row.Title = title.String
	row.Username = user.String
	row.ChatType = ctype.String
	row.PushEnabled = push != 0
	row.Enable6Code = e6 != 0
	row.Enable7Code = e7 != 0
	if created.Valid {
		row.CreatedAt = created.Time
	}
	if updated.Valid {
		row.UpdatedAt = updated.Time
	}
	return row, nil
}

// FormatGroupCodeState summarizes push / 6 / 7 flags and effective send modes.
func FormatGroupCodeState(row domain.BotGroupSettings) string {
	return fmt.Sprintf(
		"push=%v enable_6=%v enable_7=%v（生效推送: 6码=%v 7码=%v）",
		row.PushEnabled, row.Enable6Code, row.Enable7Code,
		Effective6Code(row), Effective7Code(row),
	)
}

func (s *Store) UpsertGroupMeta(chatID, title, username, chatType string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil
	}
	if err := s.EnsureGroup(chatID); err != nil {
		return err
	}
	updates := map[string]any{}
	if title != "" {
		updates["title"] = title
	}
	if username != "" {
		updates["username"] = username
	}
	if chatType != "" {
		updates["chat_type"] = chatType
	}
	if len(updates) == 0 {
		return nil
	}
	return s.DB.Model(&domain.BotGroupSettings{}).Where("chat_id = ?", chatID).Updates(updates).Error
}

func (s *Store) ListGroups(offset, limit int) ([]domain.BotGroupSettings, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := s.DB.Model(&domain.BotGroupSettings{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []domain.BotGroupSettings
	err := s.DB.Order("id").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

func (s *Store) UpsertMember(chatID, userID, username, firstName string, isAdmin, isCreator bool) error {
	chatID = strings.TrimSpace(chatID)
	userID = strings.TrimSpace(userID)
	if chatID == "" || userID == "" {
		return nil
	}
	row := domain.BotGroupMember{ChatID: chatID, UserID: userID}
	assigns := domain.BotGroupMember{
		Username:  username,
		FirstName: firstName,
		IsAdmin:   isAdmin,
		IsCreator: isCreator,
	}
	return s.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Assign(assigns).FirstOrCreate(&row).Error
}

type MemberInfo struct {
	UserID, Username, FirstName, Status string
}

func (s *Store) RefreshAdmins(chatID string, members []MemberInfo) error {
	chatID = strings.TrimSpace(chatID)
	if err := s.DB.Model(&domain.BotGroupMember{}).Where("chat_id = ?", chatID).
		Updates(map[string]any{"is_admin": false, "is_creator": false}).Error; err != nil {
		return err
	}
	for _, m := range members {
		isCreator := m.Status == "creator"
		isAdmin := isCreator || m.Status == "administrator"
		if err := s.UpsertMember(chatID, m.UserID, m.Username, m.FirstName, isAdmin, isCreator); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListMembers(chatID string, offset, limit int) ([]domain.BotGroupMember, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	chatID = strings.TrimSpace(chatID)
	var total int64
	if err := s.DB.Model(&domain.BotGroupMember{}).Where("chat_id = ?", chatID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []domain.BotGroupMember
	err := s.DB.Where("chat_id = ?", chatID).Order("is_creator DESC, is_admin DESC, id").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

func (s *Store) DeleteMember(chatID, userID string) error {
	chatID = strings.TrimSpace(chatID)
	userID = strings.TrimSpace(userID)
	if chatID == "" || userID == "" {
		return nil
	}
	return s.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&domain.BotGroupMember{}).Error
}
