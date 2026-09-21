package botperm

import (
	"errors"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/domain"
	"gorm.io/gorm"
)

const (
	RoleDeveloper  = "developer"
	RoleAdmin      = "admin"
	RoleGroupAdmin = "group_admin"
	RoleNone       = ""
)

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

// RoleInChat returns the highest role the user has for the given chat.
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

// CanManageRoles: only developers can add/remove admins; developer OR admin can grant/revoke group_admin.
func (s *Store) CanManageAdmins(actorID string) bool {
	return s.IsDeveloper(actorID)
}

func (s *Store) CanManageGroupAdmins(actorID string) (bool, error) {
	return s.IsAdmin(actorID)
}

// CanControlSensitive allows ads/broadcast/say for developer, admin, or group_admin (scoped).
func (s *Store) CanControlSensitive(actorID, chatID string) (bool, error) {
	return s.IsGroupAdmin(actorID, chatID)
}

func (s *Store) AddAdmin(userID, note string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user_id 不能为空")
	}
	if s.IsDeveloper(userID) {
		return errors.New("该用户已是 developer，无需再设为 admin")
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
