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

func (s *Store) CanControlSensitive(actorID, chatID string) (bool, error) {
	return s.IsGroupAdmin(actorID, chatID)
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
	var rows []domain.BotGroupSettings
	if err := s.DB.Where("push_enabled = ?", true).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ChatID)
	}
	return out, nil
}
