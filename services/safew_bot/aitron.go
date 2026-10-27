package safewbot

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/davidmuller5273-boop/safe/internal/ads"
	"github.com/davidmuller5273-boop/safe/internal/aitron"
	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"

	"gorm.io/gorm"
)

const (
	aiTronEnableCmd  = "/开启ai波场单双一分" // handleCommand lower-cases the command
	aiTronDisableCmd = "/关闭ai波场单双一分"
	aiTronName       = "AI波场单双一分"
)

// aiTronCommand maps a command (also the spaced form 「/开启 AI波场单双一分」) to
// enabled/disabled; ok=false when it is not an AI波场单双一分 command.
func aiTronCommand(cmd, args string) (enabled, ok bool) {
	switch cmd {
	case aiTronEnableCmd:
		return true, true
	case aiTronDisableCmd:
		return false, true
	case "/开启", "/关闭":
		if strings.EqualFold(strings.Join(strings.Fields(args), ""), "AI波场单双一分") {
			return cmd == "/开启", true
		}
	}
	return false, false
}

// cmdSetAITron implements /开启AI波场单双一分 and /关闭AI波场单双一分. Same permission
// as /开启6码 (developer or 超级管理员); independent of /push and the 6/7码 flags.
func (w worker) cmdSetAITron(ctx context.Context, botConfig systemconfig.SafeW, userID, chatID string, enabled bool) error {
	ok, err := w.perms.CanTogglePush(userID)
	if err != nil {
		return err
	}
	if !ok {
		return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要开发者或超级管理员）")
	}
	text, _, err := w.applyAITron(chatID, enabled)
	if err != nil {
		return err
	}
	return w.replyPlain(ctx, botConfig.Token, chatID, text)
}

// applyAITron is the shared toggle (permission checked by the caller).
func (w worker) applyAITron(chatID string, enabled bool) (text, toast string, err error) {
	if err := w.perms.SetAITronEnabled(chatID, enabled); err != nil {
		return "", "", err
	}
	if enabled {
		return fmt.Sprintf("已开启本群 %s 播报：每期（约1分钟）开奖后自动发送 AI 预测。", aiTronName),
			"✅ 已开启" + aiTronName, nil
	}
	return fmt.Sprintf("已关闭本群 %s 播报。", aiTronName), "已关闭" + aiTronName, nil
}

// aiTronSender delivers one broadcast: the group's ads are wrapped around the
// (already HTML-escaped) body and the message is sent silently, not pinned.
type aiTronSender struct {
	db      *gorm.DB
	client  *safew.Client
	withAds bool
}

func (s aiTronSender) Send(ctx context.Context, chatID, body string) error {
	botConfig, err := systemconfig.LoadSafeW(s.db)
	if err != nil {
		return fmt.Errorf("等待 SafeW 配置: %w", err)
	}
	text := body
	if s.withAds {
		text, err = ads.Wrap(s.db, chatID, body)
		if err != nil {
			return err
		}
	}
	_, err = s.client.SendMessageEx(ctx, botConfig.Token, chatID, text, safew.SendOptions{DisableNotification: true})
	return err
}

// newAITronRunner builds the AI波场单双一分 runner (nil when disabled with AI_TRON_ENABLED=0).
//
// Environment: AI_TRON_API_URL (default http://127.0.0.1:8080, the
// tg-directory-bot web service), AI_TRON_WITH_ADS (default 1; 0 = no group ads).
func newAITronRunner(db *gorm.DB, client *safew.Client, perms *botperm.Store) *aitron.Runner {
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("AI_TRON_ENABLED"))); v == "0" || v == "false" || v == "off" {
		log.Printf("AI波场单双一分: 已通过 AI_TRON_ENABLED 关闭")
		return nil
	}
	if err := aitron.EnsureSchema(db); err != nil {
		log.Printf("AI波场单双一分表结构检查失败（该功能暂不可用，其他功能不受影响）: %v", err)
		return nil
	}
	withAds := true
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("AI_TRON_WITH_ADS"))); v == "0" || v == "false" || v == "off" {
		withAds = false
	}
	src := aitron.NewClient(os.Getenv("AI_TRON_API_URL"))
	log.Printf("AI波场单双一分: 数据源 %s (ads=%v)", src.BaseURL, withAds)
	return &aitron.Runner{
		Source: src,
		Store:  &aitron.MySQLStore{DB: db, Flags: perms},
		Sender: aiTronSender{db: db, client: client, withAds: withAds},
	}
}

// runAITron is the worker goroutine; it never fails the process.
func (w worker) runAITron(ctx context.Context) error {
	if w.aiTron == nil {
		return nil
	}
	return w.aiTron.Run(ctx)
}
