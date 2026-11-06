package safewbot

import (
	"context"
	"errors"
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/botclone"
	"github.com/davidmuller5273-boop/safe/internal/botscope"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
)

func tgMotherWorker(t *testing.T) (worker, worker) {
	mother, _ := cloneTestWorkers(t)
	tg := &botInstance{ID: "333", IsMother: true, client: safew.NewClient("http://127.0.0.1:1"), token: "333:tg"}
	if !mother.reg.setTGMother(tg) {
		t.Fatal("setTGMother")
	}
	return mother, mother.forInstance(tg)
}

func TestTGMotherIsMother(t *testing.T) {
	_, tg := tgMotherWorker(t)
	if !tg.isMother() || !tg.perms.IsDeveloper(devID) || !tg.canManageClones(devID) || !tg.canManageTGMother(devID) {
		t.Fatal("telegram mother must have full permissions")
	}
	cfg, _ := tg.loadBotConfig()
	if cfg.Token != "333:tg" {
		t.Fatal("tg mother uses its own token")
	}
	if tg.reg.remove("333") || tg.reg.remove("111") {
		t.Fatal("mothers cannot be removed")
	}
	if !tg.reg.isMotherID("111") || !tg.reg.isMotherID("333") || tg.reg.isMotherID("222") {
		t.Fatal("isMotherID")
	}
}

func TestSentKeysTGMotherScoped(t *testing.T) {
	_, tg := tgMotherWorker(t)
	if tg.sentChatKey("111", "-100") != "-100" || tg.sentChatKey("333", "-100") != botscope.Key("333", "-100") {
		t.Fatal("SafeW mother raw, telegram mother scoped")
	}
}

func TestCloneOfEitherMotherRejected(t *testing.T) {
	_, tg := tgMotherWorker(t)
	for _, id := range []string{"111", "333"} {
		getMe := func(ctx context.Context, base, tok string) (botclone.BotIdentity, error) {
			return botclone.BotIdentity{ID: id, Username: "x"}, nil
		}
		_, err := botclone.Register(context.Background(), nil, getMe, botclone.Request{
			Token: id + ":AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", MotherBotID: tg.reg.motherID(), IsMotherID: tg.reg.isMotherID})
		if !errors.Is(err, botclone.ErrIsMother) {
			t.Fatalf("clone of mother %s: %v", id, err)
		}
	}
}

func TestInvalidTGTokenNotSaved(t *testing.T) {
	mother, _ := cloneTestWorkers(t)
	bad := func(ctx context.Context, base, tok string) (string, string, error) {
		return "", "", errors.New("401")
	}
	// db is nil: any save attempt would panic, so success here proves nothing was saved.
	if _, err := mother.applyTGMotherToken(context.Background(), "444:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", bad); err == nil {
		t.Fatal("invalid token accepted")
	}
	if _, err := mother.applyTGMotherToken(context.Background(), "garbage", bad); err == nil {
		t.Fatal("garbage accepted")
	}
	if mother.reg.tgMotherInst() != nil {
		t.Fatal("no telegram mother should run")
	}
}

func TestTGMotherButtons(t *testing.T) {
	mother, child := cloneTestWorkers(t)
	if !hasAction(mother.privateHomeMarkup(tierSuper, devID, "developer"), "tgs") {
		t.Fatal("developer sees 设置TG母机器人")
	}
	if hasAction(child.privateHomeMarkup(tierSuper, devID, "developer"), "tgs") || hasAction(mother.privateHomeMarkup(tierSuper, "999", "super"), "tgs") {
		t.Fatal("only mother developers")
	}
	if ok, _ := authorizeCallback(child.perms, devID, "-100", "tgs"); ok {
		t.Fatal("clone must refuse tgs")
	}
	_, tg := tgMotherWorker(t)
	kb := tg.privateHomeMarkup(tierSuper, devID, "developer")
	if !hasAction(kb, "tgo") || hasAction(kb, "tgs") {
		t.Fatal("running TG mother shows 关闭")
	}
}
