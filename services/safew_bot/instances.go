package safewbot

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/botclone"
	"github.com/davidmuller5273-boop/safe/internal/botscope"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
	"github.com/davidmuller5273-boop/safe/internal/systemconfig"

	"gorm.io/gorm"
)

// botInstance is one bot served by this safew-bot process: the mother (the
// configured SafeW bot) or a clone (克隆机器人, usually on api.telegram.org).
// Every instance has its own client, token, getUpdates offset and group data
// (scoped by ID); lottery results / predictions / tron snapshots are shared.
type botInstance struct {
	ID       string
	Username string
	IsMother bool
	APIBase  string
	ParentID string

	client *safew.Client
	token  string // clones only; the mother re-reads systemconfig each time
	cancel context.CancelFunc
}

// registry tracks running instances. The mother process owns every poller,
// including clones of clones.
type registry struct {
	mu      sync.RWMutex
	root    context.Context
	mother  *botInstance
	bots    map[string]*botInstance
	pending map[string]time.Time // botID|userID → waiting for a pasted token
}

func newRegistry(root context.Context, mother *botInstance) *registry {
	r := &registry{root: root, mother: mother, bots: map[string]*botInstance{}, pending: map[string]time.Time{}}
	if mother != nil {
		r.bots[mother.ID] = mother
	}
	return r
}

func (r *registry) get(id string) *botInstance {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bots[id]
}

func (r *registry) motherID() string {
	if r == nil || r.mother == nil {
		return ""
	}
	return r.mother.ID
}

// add registers inst; false when that bot_id is already running.
func (r *registry) add(inst *botInstance) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.bots[inst.ID]; ok {
		return false
	}
	r.bots[inst.ID] = inst
	return true
}

// remove stops and unregisters a clone (never the mother).
func (r *registry) remove(id string) bool {
	r.mu.Lock()
	inst, ok := r.bots[id]
	if ok && !inst.IsMother {
		delete(r.bots, id)
	}
	r.mu.Unlock()
	if !ok || inst.IsMother {
		return false
	}
	if inst.cancel != nil {
		inst.cancel()
	}
	return true
}

func (r *registry) running(id string) bool { return r.get(id) != nil }

const clonePendingTTL = 10 * time.Minute

func (r *registry) setPending(botID, userID string, on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := botscope.Key(botID, userID)
	if on {
		r.pending[k] = time.Now()
	} else {
		delete(r.pending, k)
	}
}

func (r *registry) isPending(botID, userID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.pending[botscope.Key(botID, userID)]
	return ok && time.Since(t) < clonePendingTTL
}

// ---- worker helpers ----

// forInstance returns a worker bound to inst: its client, token and bot_id-scoped stores.
func (w worker) forInstance(inst *botInstance) worker {
	c := w
	c.inst = inst
	c.client = inst.client
	if w.basePerms != nil {
		c.perms = w.basePerms.ForBot(inst.ID, inst.IsMother)
	}
	if w.lb != nil {
		c.lbs = w.lb.store.ForBot(inst.ID)
	}
	return c
}

// botID is the current instance's bot_id ("" only in unit tests).
func (w worker) botID() string {
	if w.inst == nil {
		return ""
	}
	return w.inst.ID
}

// isMother is true for the mother bot (and in tests without an instance).
func (w worker) isMother() bool { return w.inst == nil || w.inst.IsMother }

// loadBotConfig returns this instance's token (clones never read the mother's config).
func (w worker) loadBotConfig() (systemconfig.SafeW, error) {
	if w.inst != nil && !w.inst.IsMother {
		return systemconfig.SafeW{Token: w.inst.token}, nil
	}
	return systemconfig.LoadSafeW(w.db)
}

// route returns the worker + token for a bot_id (fan-out senders).
func (w worker) route(botID string) (worker, string, bool) {
	inst := w.reg.get(botID)
	if inst == nil {
		return w, "", false
	}
	iw := w.forInstance(inst)
	cfg, err := iw.loadBotConfig()
	if err != nil || strings.TrimSpace(cfg.Token) == "" {
		return w, "", false
	}
	return iw, cfg.Token, true
}

// sentKey keeps the mother's Redis "already sent" keys unchanged.
func (w worker) sentChatKey(botID, chatID string) string {
	if botID == w.reg.motherID() {
		return chatID
	}
	return botscope.Key(botID, chatID)
}

// startClone launches a clone's getUpdates loop inside this process.
func (w worker) startClone(c botclone.Clone, token string) bool {
	if w.reg == nil {
		return false
	}
	base := strings.TrimSpace(c.APIBase)
	if base == "" {
		base = botclone.DefaultAPIBase
	}
	ctx, cancel := context.WithCancel(w.reg.root)
	inst := &botInstance{ID: c.BotID, Username: c.Username, APIBase: base, ParentID: c.ParentBotID,
		client: safew.NewClient(base), token: token, cancel: cancel}
	if !w.reg.add(inst) {
		cancel()
		return false
	}
	iw := w.forInstance(inst)
	go func() {
		dctx, dcancel := context.WithTimeout(ctx, 20*time.Second)
		if err := inst.client.DeleteWebhook(dctx, token); err != nil && ctx.Err() == nil {
			log.Printf("克隆机器人 @%s deleteWebhook 失败（忽略）: %v", c.Username, err)
		}
		dcancel()
		log.Printf("克隆机器人已启动: bot_id=%s @%s", c.BotID, c.Username)
		_ = iw.runCommands(ctx)
		log.Printf("克隆机器人已停止: bot_id=%s @%s", c.BotID, c.Username)
	}()
	return true
}

// startActiveClones boots every active clone on startup.
func (w worker) startActiveClones() {
	if w.clones == nil {
		return
	}
	rows, err := w.clones.ListActive()
	if err != nil {
		log.Printf("读取克隆机器人失败: %v", err)
		return
	}
	for _, c := range rows {
		if c.BotID == w.reg.motherID() {
			continue
		}
		token, err := w.clones.Token(c)
		if err != nil {
			log.Printf("克隆机器人 bot_id=%s 解密 Token 失败，已跳过: %v", c.BotID, err)
			_ = w.clones.SetStatus(c.BotID, botclone.StatusDisabled, "Token 解密失败（CLONE_CIPHER_KEY 或母机器人 Token 已变更）")
			continue
		}
		w.startClone(c, token)
	}
	if len(rows) > 0 {
		log.Printf("已加载克隆机器人 %d 个", len(rows))
	}
}

// resolveMother waits for the mother token, then identifies the mother bot via
// getMe (falls back to the token's numeric prefix when getMe is unreachable).
func resolveMother(ctx context.Context, db *gorm.DB, client *safew.Client) (*botInstance, string, error) {
	for {
		cfg, err := systemconfig.LoadSafeW(db)
		if err == nil && strings.TrimSpace(cfg.Token) != "" {
			id, username := "", ""
			for attempt := 0; attempt < 3 && id == ""; attempt++ {
				gctx, cancel := context.WithTimeout(ctx, 15*time.Second)
				me, gerr := client.GetMe(gctx, cfg.Token)
				cancel()
				if gerr == nil && me.IDString() != "" {
					id, username = me.IDString(), me.Username
					break
				}
				if ctx.Err() != nil {
					return nil, "", ctx.Err()
				}
				log.Printf("母机器人 getMe 失败(%d/3): %v", attempt+1, gerr)
				if !wait(ctx, 2*time.Second) {
					return nil, "", ctx.Err()
				}
			}
			if id == "" {
				id = botscope.TokenBotID(cfg.Token)
			}
			if id == "" {
				var saved domain.SystemConfig
				if db.Where("`key` = ?", "mother_bot_id").First(&saved).Error == nil {
					id = strings.TrimSpace(saved.Value)
				}
			}
			if id == "" {
				id = "mother"
			}
			return &botInstance{ID: id, Username: username, IsMother: true, APIBase: client.BaseURL(), client: client}, cfg.Token, nil
		}
		log.Printf("等待 SafeW 机器人 Token 配置: %v", err)
		if !wait(ctx, 5*time.Second) {
			return nil, "", ctx.Err()
		}
	}
}

// cloneSecret is CLONE_CIPHER_KEY, or the mother token when unset.
func cloneSecret(motherToken string) string {
	if v := strings.TrimSpace(os.Getenv("CLONE_CIPHER_KEY")); v != "" {
		return v
	}
	return motherToken
}
