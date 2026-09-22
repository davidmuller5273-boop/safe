package database

import (
	"errors"
	"fmt"
	"strings"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Open(host string, port int, user, password, database string) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Asia%%2FShanghai&time_zone=%%27%%2B08%%3A00%%27", user, password, host, port, database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err = db.AutoMigrate(
		&domain.Admin{},
		&domain.Role{},
		&domain.Permission{},
		&domain.SystemConfig{},
		&domain.LotteryType{},
		&domain.DrawRecord{},
		&domain.HotNumberPrediction{},
		&domain.BotAdmin{},
		&domain.BotGroupAdmin{},
		&domain.BotGroupAd{},
		&domain.BotGroupSettings{},
		&domain.BotGroupMember{},
	); err != nil {
		return nil, err
	}
	if err = ensureHotNumberPredictionSchema(db); err != nil {
		return nil, err
	}
	return db, seed(db)
}

// ensureHotNumberPredictionSchema adds code_size defaults and a composite unique
// index. Index creation is done manually because GORM AutoMigrate can emit
// Error 1061 (Duplicate key name) for multi-column uniqueIndex tags.
func ensureHotNumberPredictionSchema(db *gorm.DB) error {
	// Backfill code_size for rows created before the column existed.
	if err := db.Exec("UPDATE hot_number_predictions SET code_size = 7 WHERE code_size = 0 OR code_size IS NULL").Error; err != nil {
		return err
	}
	// Drop legacy unique index (lottery_type_id, issue_number) if present.
	if db.Migrator().HasIndex(&domain.HotNumberPrediction{}, "idx_hot_predictions_lottery_issue") {
		if err := db.Migrator().DropIndex(&domain.HotNumberPrediction{}, "idx_hot_predictions_lottery_issue"); err != nil {
			return err
		}
	}
	const name = "idx_hot_predictions_lottery_issue_size"
	if db.Migrator().HasIndex(&domain.HotNumberPrediction{}, name) {
		return nil
	}
	if err := db.Exec("CREATE UNIQUE INDEX `" + name + "` ON `hot_number_predictions` (`lottery_type_id`,`issue_number`,`code_size`)").Error; err != nil {
		// Another process may have created it; ignore duplicate name.
		if isDuplicateKeyName(err) {
			return nil
		}
		return err
	}
	return nil
}

func isDuplicateKeyName(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1061") || strings.Contains(msg, "Duplicate key name")
}
func seed(db *gorm.DB) error {
	permissions := []domain.Permission{{Code: "dashboard:view", Name: "查看首页"}, {Code: "admin:manage", Name: "管理员管理"}, {Code: "role:manage", Name: "角色管理"}, {Code: "permission:view", Name: "查看权限"}, {Code: "system:config", Name: "系统配置"}, {Code: "lottery:manage", Name: "彩票管理"}}
	for i := range permissions {
		if err := db.FirstOrCreate(&permissions[i], domain.Permission{Code: permissions[i].Code}).Error; err != nil {
			return err
		}
	}
	var role domain.Role
	err := db.Preload("Permissions").Where("name = ?", "超级管理员").First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		role = domain.Role{Name: "超级管理员", Description: "拥有全部基础权限", Permissions: permissions}
		if err = db.Create(&role).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	// Append 只补充缺失的角色权限关联，不移除已有权限。
	if err := db.Model(&role).Association("Permissions").Append(permissions); err != nil {
		return err
	}
	var count int64
	if err := db.Model(&domain.Admin{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		return db.Create(&domain.Admin{Username: "admin", PasswordHash: string(hash), Name: "系统管理员", RoleID: role.ID, Enabled: true}).Error
	}
	return nil
}
