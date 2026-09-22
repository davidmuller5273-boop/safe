package database

import (
	"errors"
	"fmt"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
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
	if err = ensureBotGroupSettingsSchema(db); err != nil {
		return nil, err
	}
	return db, seed(db)
}

// ensureBotGroupSettingsSchema adds columns AutoMigrate sometimes skips on
// existing MySQL tables (e.g. enable_6_code / enable_7_code), and removes
// mistaken enable6_code / enable7_code columns that GORM's default naming
// (Enable6Code → enable6_code) may have created beside the SQL-migration names.
func ensureBotGroupSettingsSchema(db *gorm.DB) error {
	type col struct {
		field string
		ddl   string
	}
	cols := []col{
		{"Title", "ALTER TABLE `bot_group_settings` ADD COLUMN `title` varchar(255) DEFAULT NULL"},
		{"Username", "ALTER TABLE `bot_group_settings` ADD COLUMN `username` varchar(128) DEFAULT NULL"},
		{"ChatType", "ALTER TABLE `bot_group_settings` ADD COLUMN `chat_type` varchar(32) DEFAULT NULL"},
		{"Enable6Code", "ALTER TABLE `bot_group_settings` ADD COLUMN `enable_6_code` tinyint(1) NOT NULL DEFAULT 0"},
		{"Enable7Code", "ALTER TABLE `bot_group_settings` ADD COLUMN `enable_7_code` tinyint(1) NOT NULL DEFAULT 0"},
	}
	for _, c := range cols {
		if db.Migrator().HasColumn(&domain.BotGroupSettings{}, c.field) {
			continue
		}
		if err := db.Exec(c.ddl).Error; err != nil {
			if isDuplicateColumn(err) {
				continue
			}
			return err
		}
	}
	return mergeMistakenCodeColumns(db)
}

// mergeMistakenCodeColumns copies values from GORM-default enable6_code /
// enable7_code into enable_6_code / enable_7_code, then drops the mistaken columns.
// Missing wrong-named columns are skipped; unknown-column errors never abort startup.
func mergeMistakenCodeColumns(db *gorm.DB) error {
	pairs := []struct{ wrong, right string }{
		{"enable6_code", "enable_6_code"},
		{"enable7_code", "enable_7_code"},
	}
	for _, p := range pairs {
		hasWrong, err := mysqlColumnExists(db, "bot_group_settings", p.wrong)
		if err != nil {
			return err
		}
		if !hasWrong {
			continue
		}
		hasRight, err := mysqlColumnExists(db, "bot_group_settings", p.right)
		if err != nil {
			return err
		}
		if hasRight {
			q := fmt.Sprintf(
				"UPDATE `bot_group_settings` SET `%s` = `%s` WHERE `%s` = 0 AND `%s` <> 0",
				p.right, p.wrong, p.right, p.wrong,
			)
			if err := db.Exec(q).Error; err != nil {
				if isUnknownColumn(err) {
					continue
				}
				return err
			}
			if err := db.Exec(fmt.Sprintf("ALTER TABLE `bot_group_settings` DROP COLUMN `%s`", p.wrong)).Error; err != nil {
				if isUnknownColumn(err) || isCantDropColumn(err) {
					continue
				}
				return err
			}
			continue
		}
		// Only the mistaken name exists: rename it to the canonical column.
		q := fmt.Sprintf(
			"ALTER TABLE `bot_group_settings` CHANGE COLUMN `%s` `%s` tinyint(1) NOT NULL DEFAULT 0",
			p.wrong, p.right,
		)
		if err := db.Exec(q).Error; err != nil {
			if isUnknownColumn(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func mysqlColumnExists(db *gorm.DB, table, column string) (bool, error) {
	if !isSafeSQLIdent(table) || !isSafeSQLIdent(column) {
		return false, fmt.Errorf("invalid SQL identifier: %s.%s", table, column)
	}
	// Probe with SELECT ... LIMIT 0. Silence expected "unknown column" noise.
	silent := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	err := silent.Exec(fmt.Sprintf("SELECT `%s` FROM `%s` LIMIT 0", column, table)).Error
	if err == nil {
		return true, nil
	}
	if isUnknownColumn(err) {
		return false, nil
	}
	return false, err
}

func isSafeSQLIdent(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func isUnknownColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1054") || strings.Contains(msg, "Unknown column")
}

func isCantDropColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1091") || strings.Contains(msg, "Can't DROP")
}

func isDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1060") || strings.Contains(msg, "Duplicate column")
}

// ensureHotNumberPredictionSchema adds code_size column/defaults and a composite
// unique index (lottery_type_id, issue_number, code_size). Without code_size in
// the unique key, size-6 and size-7 FirstOrCreate rows clobber each other.
// Index work is manual because GORM AutoMigrate can emit Error 1061.
func ensureHotNumberPredictionSchema(db *gorm.DB) error {
	if !db.Migrator().HasColumn(&domain.HotNumberPrediction{}, "CodeSize") {
		if err := db.Exec("ALTER TABLE `hot_number_predictions` ADD COLUMN `code_size` int NOT NULL DEFAULT 7").Error; err != nil {
			if !isDuplicateColumn(err) {
				return err
			}
		}
	}
	// Backfill code_size for rows created before the column existed.
	if err := db.Exec("UPDATE hot_number_predictions SET code_size = 7 WHERE code_size = 0 OR code_size IS NULL").Error; err != nil {
		return err
	}
	// Drop legacy unique index (lottery_type_id, issue_number) if present.
	for _, legacy := range []string{"idx_hot_predictions_lottery_issue", "idx_lottery_type_id_issue_number"} {
		if db.Migrator().HasIndex(&domain.HotNumberPrediction{}, legacy) {
			if err := db.Migrator().DropIndex(&domain.HotNumberPrediction{}, legacy); err != nil {
				return err
			}
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
