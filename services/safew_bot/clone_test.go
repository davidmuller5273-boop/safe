package safewbot

import (
	"context"
	"strings"
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/botclone"
	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/botscope"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
)

const devID = "14210512"

func cloneTestWorkers(t *testing.T) (mother, child worker) {
	t.Helper()
	m := &botInstance{ID: "111", IsMother: true, client: safew.NewClient("http://127.0.0.1:1")}
	c := &botInstance{ID: "222", client: safew.NewClient("http://127.0.0.1:1"), token: "222:x"}
	reg := newRegistry(context.Background(), m)
	reg.add(c)
	base := worker{basePerms: botperm.NewStore(nil, []string{devID}), reg: reg, clones: &botclone.Store{}}
	return base.forInstance(m), base.forInstance(c)
}

func hasAction(kb *safew.InlineKeyboardMarkup, action string) bool {
	for _, row := range kb.InlineKeyboard {
		for _, b := range row {
			if strings.HasSuffix(b.CallbackData, ":"+action) {
				return true
			}
		}
	}
	return false
}

func TestChildHidesDeveloperFeatures(t *testing.T) {
	mother, child := cloneTestWorkers(t)
	if !mother.perms.IsDeveloper(devID) || mother.perms.BotID != "111" {
		t.Fatalf("mother perms: %+v", mother.perms)
	}
	if child.perms.IsDeveloper(devID) {
		t.Fatal("IsDeveloper must be false on a clone")
	}
	if child.perms.BotID != "222" || child.isMother() || child.canManageClones(devID) {
		t.Fatal("child must not manage clones")
	}
	if !mother.canManageClones(devID) || mother.canManageClones("999") {
		t.Fatal("only mother developers manage clones")
	}
	// Developer-level buttons are refused on the child even for a mother developer.
	for _, a := range []string{"ex", "kll", "klo:333", "kld:333"} {
		if ok, _ := authorizeCallback(child.perms, devID, "-100", a); ok {
			t.Fatalf("child allowed developer action %s", a)
		}
		if ok, _ := authorizeCallback(mother.perms, devID, "-100", a); !ok {
			t.Fatalf("mother refused developer action %s", a)
		}
	}
	if ok, _ := authorizeCallback(child.perms, "999", "-100", "kl"); !ok {
		t.Fatal("anyone may start a clone")
	}
	mk := mother.privateHomeMarkup(tierSuper, devID, botperm.RoleDeveloper)
	if !hasAction(mk, "kll") || !hasAction(mk, "kl") || !hasAction(mk, "ex") {
		t.Fatal("mother developer menu misses clone list / export")
	}
	// On a clone the best role is 超级管理员 (RoleInChat never returns developer).
	ck := child.privateHomeMarkup(tierSuper, devID, botperm.RoleAdmin)
	if hasAction(ck, "kll") || hasAction(ck, "ex") || !hasAction(ck, "kl") {
		t.Fatal("child menu must show 克隆 but no developer buttons")
	}
	help := helpForRole(botperm.RoleAdmin)
	for _, dev := range []string{"/devgroups", "/devmembers", "/导出所有群成员", "/克隆列表"} {
		if strings.Contains(help, dev) {
			t.Fatalf("help leaks %s", dev)
		}
	}
	if !strings.Contains(help, "/克隆") {
		t.Fatal("help misses /克隆")
	}
}

func TestChildUsesOwnToken(t *testing.T) {
	_, child := cloneTestWorkers(t)
	cfg, err := child.loadBotConfig()
	if err != nil || cfg.Token != "222:x" {
		t.Fatalf("child token: %v", err)
	}
	if child.sentChatKey("222", "-100") != botscope.Key("222", "-100") || child.sentChatKey("111", "-100") != "-100" {
		t.Fatal("sent keys: mother unchanged, clone scoped")
	}
}

func TestRegistryNoDuplicatePoller(t *testing.T) {
	_, child := cloneTestWorkers(t)
	if child.reg.add(&botInstance{ID: "222"}) || child.reg.add(&botInstance{ID: "111"}) {
		t.Fatal("same bot_id must not get a second poller")
	}
	if child.reg.remove("111") {
		t.Fatal("mother cannot be removed")
	}
	if !child.reg.remove("222") || child.reg.running("222") {
		t.Fatal("remove clone")
	}
}
