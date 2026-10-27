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

	autoTronEnableCmd  = "/开启自动最优波场单双一分"
	autoTronDisableCmd = "/关闭自动最优波场单双一分"
	autoTronName       = "自动最优波场单双一分"
)

// tronCommand maps a command (also the spaced form 「/开启 <name>」) to
// enabled/disabled; ok=false when it is not a command for that broadcast.
func tronCommand(cmd, args, enableCmd, disableCmd, name string) (enabled, ok bool) {
	switch cmd {
	case enableCmd:
		return true, true
	case disableCmd:
		return false, true
	case "/开启", "/关闭":
		if strings.EqualFold(strings.Join(strings.Fields(args), ""), name) {
			return cmd == "/开启", true
		}
	}
	return false, false
}

// aiTronCommand: /开启AI波场单双一分 and /关闭AI波场单双一分.
func aiTronCommand(cmd, args string) (enabled, ok bool) {
	return tronCommand(cmd, args, aiTronEnableCmd, aiTronDisableCmd, aiTronName)
}

// autoTronCommand: /开启自动最优波场单双一分 and /关闭自动最优波场单双一分.
func autoTronCommand(cmd, args string) (enabled, ok bool) {
	return tronCommand(cmd, args, autoTronEnableCmd, autoTronDisableCmd, autoTronName)
}

// cmdSetAITron implements /开启AI波场单双一分 and /关闭AI波场单双一分. Same permission
// as /开启6码 (developer or 超级管理员); independent of /push and the 6/7码 flags.
func (w worker) cmdSetAITron(ctx context.Context, botConfig systemconfig.SafeW, userID, chatID string, enabled bool) error {
	return w.cmdSetTron(ctx, botConfig, userID, chatID, enabled, false)
}

// cmdSetAutoTron implements /开启自动最优波场单双一分 and /关闭自动最优波场单双一分
// (same permission and behaviour as the AI one, own flag).
func (w worker) cmdSetAutoTron(ctx context.Context, botConfig systemconfig.SafeW, userID, chatID string, enabled bool) error {
	return w.cmdSetTron(ctx, botConfig, userID, chatID, enabled, true)
}

func (w worker) cmdSetTron(ctx context.Context, botConfig systemconfig.SafeW, userID, chatID string, enabled, auto bool) error {
	ok, err := w.perms.CanTogglePush(userID)
	if err != nil {
		return err
	}
	if !ok {
		return w.replyPlain(ctx, botConfig.Token, chatID, "权限不足（需要开发者或超级管理员）")
	}
	text, _, err := w.applyTron(chatID, enabled, auto)
	if err != nil {
		return err
	}
	return w.replyPlain(ctx, botConfig.Token, chatID, text)
}

// applyAITron is the shared toggle (permission checked by the caller).
func (w worker) applyAITron(chatID string, enabled bool) (text, toast string, err error) {
	return w.applyTron(chatID, enabled, false)
}

func (w worker) applyTron(chatID string, enabled, auto bool) (text, toast string, err error) {
	name, what := aiTronName, "AI 预测"
	set := w.perms.SetAITronEnabled
	if auto {
		name, what = autoTronName, "自动最优预测"
		set = w.perms.SetAutoTronEnabled
	}
	if err := set(chatID, enabled); err != nil {
		return "", "", err
	}
	if enabled {
		return fmt.Sprintf("已开启本群 %s 播报：每期（约1分钟）开奖后自动发送%s。", name, what),
			"✅ 已开启" + name, nil
	}
	return fmt.Sprintf("已关闭本群 %s 播报。", name), "已关闭" + name, nil
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

// tronEnvOff reports whether an env switch (e.g. AI_TRON_ENABLED) is 0/false/off.
func tronEnvOff(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return v == "0" || v == "false" || v == "off"
}

// newAITronRunner builds the AI波场单双一分 runner (nil when disabled with AI_TRON_ENABLED=0).
//
// Environment: AI_TRON_API_URL (default http://127.0.0.1:8080, the
// tg-directory-bot web service), AI_TRON_WITH_ADS (default 1; 0 = no group ads).
func newAITronRunner(db *gorm.DB, client *safew.Client, perms *botperm.Store) *aitron.Runner {
	return newTronRunner(db, client, aitron.AI, "AI_TRON_ENABLED", "AI_TRON_API_URL", "AI_TRON_WITH_ADS",
		perms.ListAITronChatIDs, perms.SetAITronEnabled)
}

// newAutoTronRunner builds the 自动最优波场单双一分 runner (nil when disabled with
// AUTO_TRON_ENABLED=0). It reads GET {base}/api/tron-draw/auto-broadcast; base and
// ads follow AUTO_TRON_API_URL / AUTO_TRON_WITH_ADS, defaulting to the AI_TRON_* values.
func newAutoTronRunner(db *gorm.DB, client *safew.Client, perms *botperm.Store) *aitron.Runner {
	return newTronRunner(db, client, aitron.Auto, "AUTO_TRON_ENABLED", "AUTO_TRON_API_URL", "AUTO_TRON_WITH_ADS",
		perms.ListAutoTronChatIDs, perms.SetAutoTronEnabled)
}

func newTronRunner(db *gorm.DB, client *safew.Client, kind aitron.Kind, enabledEnv, urlEnv, adsEnv string,
	list func() ([]string, error), set func(string, bool) error) *aitron.Runner {
	if tronEnvOff(enabledEnv) {
		log.Printf("%s: 已通过 %s 关闭", kind.Name, enabledEnv)
		return nil
	}
	if err := aitron.EnsureSchema(db, kind); err != nil {
		log.Printf("%s表结构检查失败（该功能暂不可用，其他功能不受影响）: %v", kind.Name, err)
		return nil
	}
	withAds := true
	if tronEnvOff(adsEnv) || (os.Getenv(adsEnv) == "" && kind.Key == "auto" && tronEnvOff("AI_TRON_WITH_ADS")) {
		withAds = false
	}
	base := os.Getenv(urlEnv)
	if base == "" && kind.Key == "auto" {
		base = os.Getenv("AI_TRON_API_URL")
	}
	src := aitron.NewKindClient(base, kind)
	log.Printf("%s: 数据源 %s%s (ads=%v)", kind.Name, src.BaseURL, kind.Path, withAds)
	return &aitron.Runner{
		Kind:   kind,
		Source: src,
		Store:  &aitron.MySQLStore{DB: db, Table: kind.Table, List: list, Set: set},
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

// runAutoTron is the 自动最优 worker goroutine; it never fails the process.
func (w worker) runAutoTron(ctx context.Context) error {
	if w.autoTron == nil {
		return nil
	}
	return w.autoTron.Run(ctx)
}
