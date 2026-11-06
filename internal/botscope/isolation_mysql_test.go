package botscope_test

// Bot isolation against a real MySQL/MariaDB. Runs only when
// SAFE_TEST_MYSQL_DSN is set (e.g. "root:pw@tcp(127.0.0.1:3306)/safe_test?parseTime=true");
// the database is wiped, so never point it at a real deployment.

import (
	"os"
	"testing"

	"github.com/davidmuller5273-boop/safe/internal/ads"
	"github.com/davidmuller5273-boop/safe/internal/botclone"
	"github.com/davidmuller5273-boop/safe/internal/botperm"
	"github.com/davidmuller5273-boop/safe/internal/botscope"
	"github.com/davidmuller5273-boop/safe/internal/domain"
	"github.com/davidmuller5273-boop/safe/internal/lotterybroadcast"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTestDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("SAFE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("SAFE_TEST_MYSQL_DSN 未设置，跳过 MySQL 隔离测试")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tbl := range []string{"bot_group_settings", "bot_group_ads", "bot_group_admins", "bot_group_members", "bot_admins",
		lotterybroadcast.TableSubscriptions, lotterybroadcast.TableOutbox, botclone.Table} {
		db.Exec("DROP TABLE IF EXISTS `" + tbl + "`")
	}
	// Legacy schema (pre bot_id) to exercise the migration.
	db.Exec("CREATE TABLE bot_group_settings (id bigint unsigned AUTO_INCREMENT PRIMARY KEY, chat_id varchar(64) NOT NULL, title varchar(255), username varchar(128), chat_type varchar(32), push_enabled tinyint(1) NOT NULL DEFAULT 0, enable_6_code tinyint(1) NOT NULL DEFAULT 0, enable_7_code tinyint(1) NOT NULL DEFAULT 0, enable_runner_up_6_code tinyint(1) NOT NULL DEFAULT 0, enable_runner_up_7_code tinyint(1) NOT NULL DEFAULT 0, enable_ai_tron tinyint(1) NOT NULL DEFAULT 0, enable_auto_tron tinyint(1) NOT NULL DEFAULT 0, created_at datetime(3), updated_at datetime(3), UNIQUE KEY idx_bot_group_settings_chat_id (chat_id))")
	db.Exec("CREATE TABLE bot_group_ads (id bigint unsigned AUTO_INCREMENT PRIMARY KEY, chat_id varchar(64) NOT NULL, prefix_ad text, suffix_ad text, created_at datetime(3), updated_at datetime(3), UNIQUE KEY idx_bot_group_ads_chat_id (chat_id))")
	db.Exec("INSERT INTO bot_group_settings (chat_id, push_enabled, created_at, updated_at) VALUES ('-100', 1, NOW(3), NOW(3))")
	db.Exec("INSERT INTO bot_group_ads (chat_id, prefix_ad, suffix_ad, created_at, updated_at) VALUES ('-100', 'OLD', '', NOW(3), NOW(3))")
	if err := db.AutoMigrate(&domain.BotAdmin{}, &domain.BotGroupAdmin{}, &domain.BotGroupAd{}, &domain.BotGroupSettings{}, &domain.BotGroupMember{}); err != nil {
		t.Fatal(err)
	}
	if err := lotterybroadcast.EnsureSchema(db); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // idempotent
		if err := botscope.EnsureSchema(db); err != nil {
			t.Fatal(err)
		}
	}
	if err := botscope.BackfillMother(db, "111"); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestBotIsolationSameChatID(t *testing.T) {
	db := openTestDB(t)
	base := botperm.NewStore(db, []string{"14210512"})
	a, b := base.ForBot("111", true), base.ForBot("222", false)
	const chat = "-100"

	// Legacy row now belongs to the mother.
	if s, _ := a.GetGroupSettings(chat); !s.PushEnabled {
		t.Fatal("legacy row not backfilled to mother")
	}
	if s, _ := ads.LoadForChat(db, "111", chat); s.PrefixAd != "OLD" {
		t.Fatalf("legacy ad lost: %+v", s)
	}
	if s, _ := b.GetGroupSettings(chat); s.PushEnabled || s.ID != 0 {
		t.Fatalf("bot B sees bot A's group: %+v", s)
	}
	if err := b.SetCodeMode(chat, 7, true); err != nil {
		t.Fatal(err)
	}
	sa, _ := a.GetGroupSettings(chat)
	sb, _ := b.GetGroupSettings(chat)
	if sa.Enable7Code || !sb.Enable7Code {
		t.Fatalf("code mode leaked: A=%+v B=%+v", sa, sb)
	}
	if err := b.SetAITronEnabled(chat, true); err != nil {
		t.Fatal(err)
	}
	if ids, _ := a.ListAITronChatIDs(); len(ids) != 0 {
		t.Fatalf("tron flag leaked: %v", ids)
	}
	keys, _ := base.ListBroadcastFlagKeysAllBots("enable_ai_tron")
	if len(keys) != 1 || keys[0] != botscope.Key("222", chat) {
		t.Fatalf("all-bots keys: %v", keys)
	}
	all, _ := base.ListPushTargetsAllBots()
	if len(all) != 2 {
		t.Fatalf("push targets across bots: %+v", all)
	}

	// Ads
	p := "B-AD"
	if err := ads.SaveGroup(db, "222", chat, &p, nil); err != nil {
		t.Fatal(err)
	}
	if out, _ := ads.Wrap(db, "111", chat, "body"); out != "OLD\n\nbody" {
		t.Fatalf("mother ad: %q", out)
	}
	if out, _ := ads.Wrap(db, "222", chat, "body"); out != "B-AD\n\nbody" {
		t.Fatalf("clone ad: %q", out)
	}
	_ = ads.ClearGroup(db, "222", chat)
	if out, _ := ads.Wrap(db, "111", chat, "body"); out != "OLD\n\nbody" {
		t.Fatal("clearing B removed A's ad")
	}

	// Admins / members
	if err := b.AddAdmin("42", "克隆申请人"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := a.IsAdmin("42"); ok {
		t.Fatal("clone admin leaked to mother")
	}
	if ok, _ := b.IsAdmin("42"); !ok {
		t.Fatal("clone admin missing")
	}
	if ok, _ := b.IsAdmin("14210512"); ok {
		t.Fatal("mother developer must not be admin on clone by default")
	}
	_ = a.AddGroupAdmin("7", chat, "")
	if ok, _ := b.IsGroupAdmin("7", chat); ok {
		t.Fatal("group admin leaked")
	}
	_ = a.UpsertMember(chat, "7", "u", "U", false, false)
	_ = b.UpsertMember(chat, "7", "u", "U", true, false)
	ma, _, _ := a.ListMembers(chat, 0, 10)
	mb, _, _ := b.ListMembers(chat, 0, 10)
	if len(ma) != 1 || len(mb) != 1 || ma[0].IsAdmin || !mb[0].IsAdmin {
		t.Fatalf("members leaked: %+v / %+v", ma, mb)
	}

	// Lottery subscriptions + outbox
	lb := lotterybroadcast.NewStore(db)
	la, lbB := lb.ForBot("111"), lb.ForBot("222")
	if err := lbB.AddSubscription(chat, "all", "42"); err != nil {
		t.Fatal(err)
	}
	if codes, _ := la.SubscribedCodesForChat(chat); len(codes) != 0 {
		t.Fatalf("subscription leaked: %v", codes)
	}
	ids, _ := lb.SubscriberChatIDs("ssq")
	if len(ids) != 1 || ids[0] != botscope.Key("222", chat) {
		t.Fatalf("subscriber keys: %v", ids)
	}
	if ok, err := lb.Enqueue(ids[0], "ssq", "1", "x"); !ok || err != nil {
		t.Fatalf("enqueue: %v %v", ok, err)
	}
	items, _ := lb.PendingOutbox(10)
	if len(items) != 1 || items[0].BotID != "222" || items[0].ChatID != chat {
		t.Fatalf("outbox: %+v", items)
	}
	if n, _ := la.DisableChat(chat); n != 0 {
		t.Fatal("A disabled B's outbox")
	}
}
