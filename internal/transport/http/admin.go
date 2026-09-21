package http

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/middleware"
	"github.com/davidmuller5273-boop/safe/internal/service"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

type Handler struct {
	DB   *gorm.DB
	Auth service.Auth
}

func Router(db *gorm.DB, auth service.Auth) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())
	h := Handler{DB: db, Auth: auth}
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok", "service": "admin"}) })
	r.POST("/admin/login", h.login)
	g := r.Group("/admin", middleware.Auth(auth))
	g.GET("/current", h.current)
	g.POST("/logout", func(c *gin.Context) { ok(c, nil) })
	g.POST("/password", h.changePassword)
	g.GET("/admins", middleware.Require(db, "admin:manage"), h.admins)
	g.POST("/admins", middleware.Require(db, "admin:manage"), h.createAdmin)
	g.PUT("/admins/:id", middleware.Require(db, "admin:manage"), h.updateAdmin)
	g.DELETE("/admins/:id", middleware.Require(db, "admin:manage"), h.deleteAdmin)
	g.GET("/roles", middleware.Require(db, "role:manage"), h.roles)
	g.POST("/roles", middleware.Require(db, "role:manage"), h.createRole)
	g.PUT("/roles/:id", middleware.Require(db, "role:manage"), h.updateRole)
	g.DELETE("/roles/:id", middleware.Require(db, "role:manage"), h.deleteRole)
	g.GET("/permissions", middleware.Require(db, "permission:view"), h.permissions)
	g.GET("/system-config", middleware.Require(db, "system:config"), h.systemConfig)
	g.PUT("/system-config/safew-bot", middleware.Require(db, "system:config"), h.updateSafeWBot)
	g.GET("/lottery-types", middleware.Require(db, "lottery:manage"), h.lotteryTypes)
	g.POST("/lottery-types", middleware.Require(db, "lottery:manage"), h.createLotteryType)
	g.PUT("/lottery-types/:id", middleware.Require(db, "lottery:manage"), h.updateLotteryType)
	g.DELETE("/lottery-types/:id", middleware.Require(db, "lottery:manage"), h.deleteLotteryType)
	g.GET("/draw-records", middleware.Require(db, "lottery:manage"), h.drawRecords)
	return r
}
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "data": data})
}
func fail(c *gin.Context, status int, err error) {
	c.JSON(status, gin.H{"code": status, "message": err.Error()})
}
func (h Handler) login(c *gin.Context) {
	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, err)
		return
	}
	token, admin, err := h.Auth.Login(body.Username, body.Password)
	if err != nil {
		fail(c, 401, err)
		return
	}
	ok(c, gin.H{"token": token, "admin": presentAdmin(admin)})
}
func (h Handler) current(c *gin.Context) {
	var admin domain.Admin
	if err := h.DB.Preload("Role.Permissions").First(&admin, c.MustGet("admin_id")).Error; err != nil {
		fail(c, 404, err)
		return
	}
	ok(c, presentAdmin(admin))
}

func (h Handler) changePassword(c *gin.Context) {
	var body struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required,min=6,max=72"`
		ConfirmPassword string `json:"confirm_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		fail(c, http.StatusBadRequest, errors.New("两次输入的新密码不一致"))
		return
	}
	var admin domain.Admin
	if err := h.DB.First(&admin, c.MustGet("admin_id")).Error; err != nil {
		fail(c, http.StatusNotFound, errors.New("管理员不存在"))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(body.CurrentPassword)) != nil {
		fail(c, http.StatusBadRequest, errors.New("原密码不正确"))
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(body.NewPassword)) == nil {
		fail(c, http.StatusBadRequest, errors.New("新密码不能与原密码相同"))
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		fail(c, http.StatusInternalServerError, errors.New("密码加密失败"))
		return
	}
	if err := h.DB.Model(&admin).Update("password_hash", string(hash)).Error; err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	ok(c, nil)
}
func presentAdmin(a domain.Admin) gin.H {
	return gin.H{"id": a.ID, "username": a.Username, "name": a.Name, "enabled": a.Enabled, "role_id": a.RoleID, "role_name": a.Role.Name, "permissions": service.PermissionCodes(a), "created_at": a.CreatedAt}
}
func (h Handler) admins(c *gin.Context) {
	var list []domain.Admin
	if err := h.DB.Preload("Role.Permissions").Order("id desc").Find(&list).Error; err != nil {
		fail(c, 500, err)
		return
	}
	data := make([]gin.H, 0, len(list))
	for _, a := range list {
		data = append(data, presentAdmin(a))
	}
	ok(c, data)
}
func (h Handler) createAdmin(c *gin.Context) {
	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Name     string `json:"name" binding:"required"`
		RoleID   uint   `json:"role_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, err)
		return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	item := domain.Admin{Username: body.Username, PasswordHash: string(hash), Name: body.Name, RoleID: body.RoleID, Enabled: true}
	if err := h.DB.Create(&item).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, item)
}
func (h Handler) updateAdmin(c *gin.Context) {
	var body struct {
		Name     string `json:"name"`
		RoleID   uint   `json:"role_id"`
		Enabled  *bool  `json:"enabled"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, err)
		return
	}
	values := map[string]any{"name": body.Name, "role_id": body.RoleID}
	if body.Enabled != nil {
		values["enabled"] = *body.Enabled
	}
	if body.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		values["password_hash"] = string(hash)
	}
	if err := h.DB.Model(&domain.Admin{}).Where("id = ?", c.Param("id")).Updates(values).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}
func (h Handler) deleteAdmin(c *gin.Context) {
	if err := h.DB.Delete(&domain.Admin{}, c.Param("id")).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}
func (h Handler) roles(c *gin.Context) {
	var list []domain.Role
	if err := h.DB.Preload("Permissions").Find(&list).Error; err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, list)
}
func (h Handler) createRole(c *gin.Context) { h.saveRole(c, domain.Role{}) }
func (h Handler) updateRole(c *gin.Context) {
	var role domain.Role
	if err := h.DB.First(&role, c.Param("id")).Error; err != nil {
		fail(c, 404, err)
		return
	}
	h.saveRole(c, role)
}
func (h Handler) saveRole(c *gin.Context, role domain.Role) {
	var body struct {
		Name          string `json:"name" binding:"required"`
		Description   string `json:"description"`
		PermissionIDs []uint `json:"permission_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, 400, err)
		return
	}
	role.Name = body.Name
	role.Description = body.Description
	if role.ID == 0 {
		if err := h.DB.Create(&role).Error; err != nil {
			fail(c, 400, err)
			return
		}
	} else if err := h.DB.Save(&role).Error; err != nil {
		fail(c, 400, err)
		return
	}
	var permissions []domain.Permission
	h.DB.Find(&permissions, body.PermissionIDs)
	if err := h.DB.Model(&role).Association("Permissions").Replace(permissions); err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, role)
}
func (h Handler) deleteRole(c *gin.Context) {
	if err := h.DB.Delete(&domain.Role{}, c.Param("id")).Error; err != nil {
		fail(c, 400, err)
		return
	}
	ok(c, nil)
}
func (h Handler) permissions(c *gin.Context) {
	var list []domain.Permission
	if err := h.DB.Order("id").Find(&list).Error; err != nil {
		fail(c, 500, err)
		return
	}
	ok(c, list)
}

func (h Handler) systemConfig(c *gin.Context) {
	config, err := systemconfig.ReadSafeW(h.DB)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	ok(c, gin.H{
		"safew_bot_token_configured": config.Token != "",
		"safew_bot_token_masked":     maskToken(config.Token),
		"safew_chat_ids":             config.ChatIDs,
		"safew_bot_ready":            config.Token != "" && len(config.ChatIDs) > 0,
	})
}

func (h Handler) updateSafeWBot(c *gin.Context) {
	var body struct {
		Token   string   `json:"token" binding:"omitempty,max=4096"`
		ChatIDs []string `json:"chat_ids" binding:"required,min=1,dive,required,max=255"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	current, err := systemconfig.ReadSafeW(h.DB)
	if err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	if body.Token == "" && current.Token == "" {
		fail(c, http.StatusBadRequest, errors.New("请填写 SafeW 机器人 Token"))
		return
	}
	if err := systemconfig.SaveSafeW(h.DB, body.Token, body.ChatIDs); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ok(c, nil)
}

func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "********"
	}
	return token[:4] + "********" + token[len(token)-4:]
}

type lotteryTypeBody struct {
	Name            string `json:"name" binding:"required,max=100"`
	Symbol          string `json:"symbol" binding:"required,max=64"`
	SecondsPerIssue uint   `json:"seconds_per_issue" binding:"required,gt=0"`
	IssuesPerDay    uint   `json:"issues_per_day" binding:"required,gt=0"`
	DrawsAllDay     bool   `json:"draws_all_day"`
}

func (h Handler) lotteryTypes(c *gin.Context) {
	var list []domain.LotteryType
	if err := h.DB.Order("id desc").Find(&list).Error; err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	ok(c, list)
}

func (h Handler) createLotteryType(c *gin.Context) {
	var body lotteryTypeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	item := domain.LotteryType{Name: body.Name, Symbol: body.Symbol, SecondsPerIssue: body.SecondsPerIssue, IssuesPerDay: body.IssuesPerDay, DrawsAllDay: body.DrawsAllDay}
	if err := h.DB.Create(&item).Error; err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ok(c, item)
}

func (h Handler) updateLotteryType(c *gin.Context) {
	var body lotteryTypeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	var item domain.LotteryType
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		fail(c, http.StatusNotFound, errors.New("彩种不存在"))
		return
	}
	item.Name = body.Name
	item.Symbol = body.Symbol
	item.SecondsPerIssue = body.SecondsPerIssue
	item.IssuesPerDay = body.IssuesPerDay
	item.DrawsAllDay = body.DrawsAllDay
	if err := h.DB.Save(&item).Error; err != nil {
		fail(c, http.StatusBadRequest, err)
		return
	}
	ok(c, item)
}

func (h Handler) deleteLotteryType(c *gin.Context) {
	result := h.DB.Delete(&domain.LotteryType{}, c.Param("id"))
	if result.Error != nil {
		fail(c, http.StatusBadRequest, result.Error)
		return
	}
	if result.RowsAffected == 0 {
		fail(c, http.StatusNotFound, errors.New("彩种不存在"))
		return
	}
	ok(c, nil)
}

func (h Handler) drawRecords(c *gin.Context) {
	page := positiveInt(c.Query("page"), 1)
	pageSize := positiveInt(c.Query("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	query := h.DB.Model(&domain.DrawRecord{})
	if lotteryTypeID := c.Query("lottery_type_id"); lotteryTypeID != "" {
		query = query.Where("lottery_type_id = ?", lotteryTypeID)
	}
	if issueNumber := c.Query("issue_number"); issueNumber != "" {
		query = query.Where("issue_number LIKE ?", "%"+issueNumber+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	var list []domain.DrawRecord
	if err := query.Preload("LotteryType").Order("issue_number DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		fail(c, http.StatusInternalServerError, err)
		return
	}
	ok(c, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

func positiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}
