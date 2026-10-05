package safewbot

import (
	"context"
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/platform/safew"
)

func keyboardActions(m *safew.InlineKeyboardMarkup) map[string]bool {
	out := map[string]bool{}
	for _, row := range m.InlineKeyboard {
		for _, b := range row {
			_, action, ok := parseCallbackData(b.CallbackData)
			if ok {
				out[action] = true
			}
		}
	}
	return out
}

func allButtons(m *safew.InlineKeyboardMarkup) []safew.InlineKeyboardButton {
	var out []safew.InlineKeyboardButton
	for _, row := range m.InlineKeyboard {
		out = append(out, row...)
	}
	return out
}

func TestPanelButtonsByRole(t *testing.T) {
	st := panelState{IsGroup: true}
	user := keyboardActions(panelKeyboard(tierForRole(botperm.RoleNone), st))
	for a := range user {
		if actionLevel(a) != levelAnyone {
			t.Errorf("ordinary user sees control button %q", a)
		}
	}
	if !user["q:lt"] || !user["q:ps"] {
		t.Errorf("ordinary user must see query buttons: %v", user)
	}

	group := keyboardActions(panelKeyboard(tierForRole(botperm.RoleGroupAdmin), st))
	for a := range group {
		if actionLevel(a) == levelSuper {
			t.Errorf("group admin sees super-only button %q", a)
		}
	}
	if !group["lb:1"] || !group["ls"] || !group["ad"] {
		t.Errorf("group admin must see lottery/ad buttons: %v", group)
	}

	for _, role := range []string{botperm.RoleAdmin, botperm.RoleDeveloper} {
		super := keyboardActions(panelKeyboard(tierForRole(role), st))
		for _, want := range []string{"q:lt", "p:1", "c6:1", "c7:1", "c:0", "r6:1", "r7:1", "r:0", "lb:1", "ls", "ad"} {
			if !super[want] {
				t.Errorf("%s missing %q: %v", role, want, super)
			}
		}
	}

	private := keyboardActions(panelKeyboard(tierSuper, panelState{IsGroup: false}))
	if len(private) != 1 || !private["q:lt"] {
		t.Errorf("private chat should only get 查看开奖: %v", private)
	}
}

func TestPanelShowsCurrentState(t *testing.T) {
	m := panelKeyboard(tierSuper, panelState{IsGroup: true, Push: true, C6: true, R7: true, SubscribedCnt: 11})
	var texts []string
	actions := keyboardActions(m)
	for _, b := range allButtons(m) {
		texts = append(texts, b.Text)
	}
	joined := strings.Join(texts, "|")
	for _, want := range []string{"✅ 推送已开", "✅ 冠军6码", "✅ 亚军7码", "✅ 开奖播报 11/11"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %s", want, joined)
		}
	}
	// enabled toggles flip to "off" actions
	if !actions["p:0"] || !actions["c6:0"] || !actions["c7:1"] || !actions["r7:0"] || !actions["lb:0"] {
		t.Errorf("actions = %v", actions)
	}
}

func TestCallbackDataFitsLimit(t *testing.T) {
	sub := map[string]bool{"new_macau_lhc": true}
	boards := []*safew.InlineKeyboardMarkup{
		panelKeyboard(tierSuper, panelState{IsGroup: true}),
		panelKeyboard(tierGroup, panelState{IsGroup: true, SubscribedCnt: 3}),
		panelKeyboard(tierUser, panelState{IsGroup: true}),
		lotterySubKeyboard(tierSuper, sub),
		historyKeyboard("new_macau_lhc", 5, 10),
	}
	for _, m := range boards {
		for _, b := range allButtons(m) {
			if len(b.CallbackData) == 0 || len(b.CallbackData) > 64 {
				t.Errorf("callback_data %q is %d bytes", b.CallbackData, len(b.CallbackData))
			}
			if _, action, ok := parseCallbackData(b.CallbackData); !ok || actionLevel(action) == levelInvalid {
				t.Errorf("button %q has unknown action", b.CallbackData)
			}
		}
	}
}

type fakePerms struct {
	super       map[string]bool
	groupAdmins map[string]bool // "user|chat"
	err         error
}

func (f fakePerms) CanTogglePush(userID string) (bool, error) { return f.super[userID], f.err }
func (f fakePerms) CanControlSensitive(userID, chatID string) (bool, error) {
	return f.super[userID] || f.groupAdmins[userID+"|"+chatID], f.err
}

func TestAuthorizeCallbackRechecksPermission(t *testing.T) {
	perms := fakePerms{super: map[string]bool{"dev": true}, groupAdmins: map[string]bool{"ga|-100": true}}
	cases := []struct {
		user, chat, action string
		want               bool
	}{
		{"nobody", "-100", "q:lt", true},
		{"nobody", "-100", "q:ps", true},
		{"nobody", "-100", "lh:ssq:2", true},
		{"nobody", "-100", "p:1", false},
		{"nobody", "-100", "c6:1", false},
		{"nobody", "-100", "lb:1", false},
		{"nobody", "-100", "ad", false},
		{"ga", "-100", "p:1", false},  // group admin cannot toggle push
		{"ga", "-100", "r7:1", false}, // nor champion/runner-up modes
		{"ga", "-100", "lb:1", true},
		{"ga", "-100", "lt:hklhc", true},
		{"ga", "-200", "lb:1", false}, // only in their own group
		{"dev", "-100", "p:0", true},
		{"dev", "-100", "c:0", true},
		{"dev", "-100", "lt:unknown", false}, // invalid action always denied
		{"dev", "-100", "rm -rf", false},
	}
	for _, c := range cases {
		got, err := authorizeCallback(perms, c.user, c.chat, c.action)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Errorf("authorize(%s,%s,%s) = %v want %v", c.user, c.chat, c.action, got, c.want)
		}
	}
	// a tier forged in callback_data never matters: authorization only looks at the clicker
	if _, action, _ := parseCallbackData("s:p:1"); func() bool { ok, _ := authorizeCallback(perms, "nobody", "-100", action); return ok }() {
		t.Error("forged super tier granted access")
	}
	perms.err = errors.New("db down")
	if _, err := authorizeCallback(perms, "x", "-100", "p:1"); err == nil {
		t.Error("expected error to propagate")
	}
}

func TestParseCallbackData(t *testing.T) {
	if tier, action, ok := parseCallbackData("g:lt:ssq"); !ok || tier != "g" || action != "lt:ssq" {
		t.Fatalf("got %s %s %v", tier, action, ok)
	}
	for _, bad := range []string{"", "x:p:1", ":p:1", "s:", "lotteryhist:ssq:1"} {
		if _, _, ok := parseCallbackData(bad); ok {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestHelpHidesDeveloperCommands(t *testing.T) {
	forbidden := []string{"/devgroups", "/devmembers", "/导出所有群成员", "/exportmembers", "/addsuperadmin", "/removesuperadmin",
		"/addadmin", "/listsuperadmins", "/listadmins", "setprefix global", "/setprefix", "/setsuffix", "global"}
	for _, role := range []string{botperm.RoleNone, botperm.RoleGroupAdmin, botperm.RoleAdmin, botperm.RoleDeveloper} {
		h := helpForRole(role)
		for _, f := range forbidden {
			if strings.Contains(h, f) {
				t.Errorf("help for %q contains %q", role, f)
			}
		}
		if strings.ContainsAny(h, "<>") {
			t.Errorf("help for %q contains <> which breaks HTML parse mode", role)
		}
	}
	user := helpForRole(botperm.RoleNone)
	for _, f := range []string{"/push on", "/push off", "/开启6码", "/开启亚军6码", "/订阅开奖", "/广告前", "/say", "/addgroupadmin"} {
		if strings.Contains(user, f) {
			t.Errorf("ordinary user help contains control command %q", f)
		}
	}
	for _, want := range []string{"/开奖", "/开奖历史", "/pushstatus", "/菜单"} {
		if !strings.Contains(user, want) {
			t.Errorf("ordinary user help missing %q", want)
		}
	}
	group := helpForRole(botperm.RoleGroupAdmin)
	if !strings.Contains(group, "/订阅开奖") || !strings.Contains(group, "/广告前") || strings.Contains(group, "/push on") {
		t.Errorf("group admin help wrong:\n%s", group)
	}
	super := helpForRole(botperm.RoleAdmin)
	for _, want := range []string{"/push on", "/开启6码", "/开启亚军7码", "/订阅开奖", "/addgroupadmin"} {
		if !strings.Contains(super, want) {
			t.Errorf("super admin help missing %q", want)
		}
	}
}

func TestExportOrdersAdminsFirst(t *testing.T) {
	rows := []domain.BotGroupMember{
		{UserID: "1", FirstName: "普通甲"},
		{UserID: "2", FirstName: "管理乙", IsAdmin: true},
		{UserID: "3", FirstName: "普通丙"},
		{UserID: "4", FirstName: "群主丁", IsAdmin: true, IsCreator: true},
		{UserID: "5", FirstName: "管理戊", IsAdmin: true},
	}
	got := orderMembersForExport(rows)
	var ids []string
	for _, m := range got {
		ids = append(ids, m.UserID)
	}
	if strings.Join(ids, ",") != "4,2,5,1,3" {
		t.Fatalf("order = %v", ids)
	}
	data := buildMembersCSV([]exportGroup{{ChatID: "-100", Title: "测试群", LiveCount: 120, Members: rows}, {ChatID: "-200", LiveCount: -1}})
	if !strings.HasPrefix(string(data), "\ufeff") {
		t.Fatal("CSV must start with UTF-8 BOM")
	}
	records, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if records[0][0] != "群名称" || records[1][4] != "【群主】" || records[2][4] != "【管理员】" || records[3][4] != "【管理员】" || records[4][4] != "成员" {
		t.Fatalf("records = %v", records)
	}
	if records[1][2] != "120" || records[1][3] != "5" || records[1][5] != "4" {
		t.Fatalf("row = %v", records[1])
	}
	last := records[len(records)-1]
	if last[1] != "-200" || last[2] != "未知" || last[0] != "(无标题)" {
		t.Fatalf("empty group row = %v", last)
	}
	pages := buildMembersText([]exportGroup{{ChatID: "-100", Title: "测试群", LiveCount: 120, Members: rows}})
	if len(pages) != 1 || strings.Index(pages[0], "【群主】4") > strings.Index(pages[0], "1 普通甲") {
		t.Fatalf("text = %v", pages)
	}
}

func TestIsPrivateGroupRow(t *testing.T) {
	if !isPrivateGroupRow(domain.BotGroupSettings{ChatID: "123", ChatType: "private"}) ||
		!isPrivateGroupRow(domain.BotGroupSettings{ChatID: "123"}) ||
		isPrivateGroupRow(domain.BotGroupSettings{ChatID: "-100", ChatType: "supergroup"}) ||
		isPrivateGroupRow(domain.BotGroupSettings{ChatID: "-100"}) {
		t.Fatal("private detection wrong")
	}
}

type fakeBroadcastClient struct {
	sent    []string
	pinned  []int
	pinErr  error
	sendErr error
}

func (f *fakeBroadcastClient) SendMessageEx(_ context.Context, _, chatID, text string, _ safew.SendOptions) (safew.Message, error) {
	if f.sendErr != nil {
		return safew.Message{}, f.sendErr
	}
	f.sent = append(f.sent, chatID+":"+text)
	return safew.Message{MessageID: 900 + len(f.sent)}, nil
}

func (f *fakeBroadcastClient) PinChatMessage(_ context.Context, _, _ string, messageID int, disable bool) error {
	if disable {
		return errors.New("broadcast pins must notify normally")
	}
	if f.pinErr != nil {
		return f.pinErr
	}
	f.pinned = append(f.pinned, messageID)
	return nil
}

func TestBroadcastIsPinnedAndPinFailureDoesNotBlock(t *testing.T) {
	c := &fakeBroadcastClient{}
	id, pinned, err := sendAndPin(context.Background(), c, "t", "-100", "彩票开奖播报")
	if err != nil || !pinned || id != 901 || len(c.pinned) != 1 || c.pinned[0] != 901 {
		t.Fatalf("id=%d pinned=%v err=%v pins=%v", id, pinned, err, c.pinned)
	}
	c.pinErr = errors.New("not enough rights to pin a message")
	id, pinned, err = sendAndPin(context.Background(), c, "t", "-100", "第二条")
	if err != nil || pinned || id != 902 || len(c.sent) != 2 {
		t.Fatalf("pin failure must not fail the broadcast: id=%d pinned=%v err=%v", id, pinned, err)
	}
	c.sendErr = errors.New("chat not found")
	if _, _, err := sendAndPin(context.Background(), c, "t", "-1", "x"); err == nil {
		t.Fatal("send failure must be reported for retry")
	}
}

func TestChunkText(t *testing.T) {
	blocks := []string{strings.Repeat("a", 10), strings.Repeat("b", 10), strings.Repeat("c", 10)}
	got := chunkText(blocks, "\n\n", 25)
	if len(got) != 2 || got[0] != blocks[0]+"\n\n"+blocks[1] || got[1] != blocks[2] {
		t.Fatalf("chunks = %q", got)
	}
}

func TestLotteryQueryThrottle(t *testing.T) {
	rt := &lotteryRuntime{lastQuery: map[string]time.Time{}}
	now := time.Now()
	if got := rt.dueForQuery([]string{"ssq", "dlt"}, now); len(got) != 2 {
		t.Fatalf("first query should refresh both: %v", got)
	}
	if got := rt.dueForQuery([]string{"ssq", "kl8"}, now.Add(5*time.Second)); len(got) != 1 || got[0] != "kl8" {
		t.Fatalf("throttled: %v", got)
	}
	if got := rt.dueForQuery([]string{"ssq"}, now.Add(lotteryQueryThrottle)); len(got) != 1 {
		t.Fatalf("after throttle window: %v", got)
	}
}

func TestHistoryKeyboard(t *testing.T) {
	if historyKeyboard("ssq", 0, 1) != nil {
		t.Fatal("single page needs no buttons")
	}
	acts := keyboardActions(historyKeyboard("ssq", 1, 3))
	if !acts["lh:ssq:0"] || !acts["lh:ssq:2"] {
		t.Fatalf("acts = %v", acts)
	}
}
