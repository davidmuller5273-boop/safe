package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/service"
	"gorm.io/gorm"
	"net/http"
	"strings"
)

func Auth(auth service.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "请先登录"})
			return
		}
		id, err := auth.Parse(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
			return
		}
		c.Set("admin_id", id)
		c.Next()
	}
}

func Require(db *gorm.DB, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var admin domain.Admin
		if err := db.Preload("Role.Permissions").First(&admin, c.MustGet("admin_id")).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "管理员不存在"})
			return
		}
		for _, permission := range admin.Role.Permissions {
			if permission.Code == code {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "没有操作权限"})
	}
}
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
