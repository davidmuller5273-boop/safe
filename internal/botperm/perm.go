package botperm

import (
	"errors"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
func (s *Store) EnsureGroup(chatID string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return nil
	}
	row := domain.BotGroupSettings{ChatID: chatID, PushEnabled: false}
	return s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoNothing: true,
	}).Create(&row).Error
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
	chatID = strings.TrimSpace(chatID)
	var row domain.BotGroupSettings
	err := s.DB.Where("chat_id = ?", chatID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
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
func (s *Store) ListPushTargets() ([]domain.BotGroupSettings, error) {
	var rows []domain.BotGroupSettings
	err := s.DB.Where("push_enabled = ?", true).Order("id").Find(&rows).Error
	return rows, err
}

// SetCodeMode enables/disables 6-code or 7-code predictions for a group.
func (s *Store) SetCodeMode(chatID string, size int, enabled bool) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return errors.New("chat_id 不能为空")
	}
	if size != 6 && size != 7 {
		return errors.New("size 必须为 6 或 7")
	}
	if err := s.EnsureGroup(chatID); err != nil {
		return err
	}
	field := "enable_6_code"
	if size == 7 {
		field = "enable_7_code"
	}
	return s.DB.Model(&domain.BotGroupSettings{}).Where("chat_id = ?", chatID).Update(field, enabled).Error
}

// IsCodeEnabled reports whether the given code size is enabled for the chat.
// Size 7 falls back to true when push is on and neither mode has been set
// (backward compatible with groups that only toggled /push).
func (s *Store) IsCodeEnabled(chatID string, size int) (bool, error) {
	chatID = strings.TrimSpace(chatID)
	if size != 6 && size != 7 {
		return false, errors.New("size 必须为 6 或 7")
	}
	var row domain.BotGroupSettings
	err := s.DB.Where("chat_id = ?", chatID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if size == 6 {
		return row.Enable6Code, nil
	}
	return Effective7Code(row), nil
}

// Effective7Code is true when Enable7Code is set, or when push is on and
// neither 6 nor 7 mode has been configured (legacy groups).
func Effective7Code(row domain.BotGroupSettings) bool {
	return row.Enable7Code || (row.PushEnabled && !row.Enable6Code && !row.Enable7Code)
}

// Effective6Code is true when Enable6Code is set.
func Effective6Code(row domain.BotGroupSettings) bool {
	return row.Enable6Code
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

