package service

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"time"
)

type Auth struct {
	DB     *gorm.DB
	Secret []byte
	Expire time.Duration
}

func (s Auth) Login(username, password string) (string, domain.Admin, error) {
	var admin domain.Admin
	if err := s.DB.Preload("Role.Permissions").Where("username = ?", username).First(&admin).Error; err != nil {
		return "", admin, errors.New("用户名或密码错误")
	}
	if !admin.Enabled || bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)) != nil {
		return "", admin, errors.New("用户名或密码错误")
	}
	claims := jwt.MapClaims{"sub": admin.ID, "exp": time.Now().Add(s.Expire).Unix(), "iat": time.Now().Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.Secret)
	return token, admin, err
}
func (s Auth) Parse(tokenString string) (uint, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("无效签名算法")
		}
		return s.Secret, nil
	})
	if err != nil || !token.Valid {
		return 0, errors.New("无效或过期的登录凭证")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("无效登录凭证")
	}
	id, ok := claims["sub"].(float64)
	if !ok {
		return 0, errors.New("无效登录凭证")
	}
	return uint(id), nil
}
func PermissionCodes(admin domain.Admin) []string {
	codes := make([]string, 0, len(admin.Role.Permissions))
	for _, p := range admin.Role.Permissions {
		codes = append(codes, p.Code)
	}
	return codes
}
