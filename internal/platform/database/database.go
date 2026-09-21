package database

import (
	"errors"
	"fmt"
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
	return db, seed(db)
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
