package lotterybroadcast

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mustJSON(t *testing.T, s string) any {
	t.Helper()
	v, err := DecodeJSON([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func bj(t *testing.T, s string) time.Time {
	t.Helper()
	v, ok := ParseDrawTime(s)
	if !ok {
		t.Fatalf("parse %q", s)
	}
	return v
}

func TestLatestWaitsForAllProvidersAndSelectsNewestIssue(t *testing.T) {
	old := Result{Source: "cwl", GameCode: "ssq", GameName: "双色球", Issue: "2026001", DrawTime: "2026-01-01",
		Primary: []string{"01", "02", "03", "04", "05", "06"}, Secondary: []string{"16"}}
	newR := Result{Source: "realtime168", GameCode: "ssq", GameName: "双色球", Issue: "2026002", DrawTime: "2026-01-03",
		Primary: []string{"07", "08", "09", "10", "11", "12"}, Secondary: []string{"15"}}
	svc := NewService(SourcesFromEnv())
	var completed int32
	slowOld := func(ctx context.Context) (Result, error) {
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&completed, 1)
		return old, nil
	}
	fastNew := func(ctx context.Context) (Result, error) {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&completed, 1)
		return newR, nil
	}
	svc.providersFn = func(Game) []provider { return []provider{fastNew, slowOld, slowOld, slowOld, slowOld} }
	r, err := svc.Latest(context.Background(), "ssq", "2026001")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r.Issue, "2026002")
	eq(t, r.Source, "realtime168")
	eq(t, atomic.LoadInt32(&completed), int32(5))
}

func TestEveryLotteryHasAtLeastFiveProviders(t *testing.T) {
	svc := NewService(SourcesFromEnv())
	for _, code := range GameOrder {
		if n := len(svc.latestProviders(Games[code])); n < 5 {
			t.Fatalf("%s has %d providers", code, n)
		}
	}
}

func TestParseCWLSSQ(t *testing.T) {
	r, err := ParseCWL(Games["ssq"], asMap(mustJSON(t, `{"code":"2026099","date":"2026-08-25","red":"01,02,03,04,05,06","blue":"16","detailsLink":"/c/2026/demo.shtml"}`)), "")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r.Primary, []string{"01", "02", "03", "04", "05", "06"})
	eq(t, r.Secondary, []string{"16"})
	eq(t, r.DetailURL, "https://www.cwl.gov.cn/c/2026/demo.shtml")
	out := FormatResult(r)
	for _, want := range []string{"福彩双色球第:2026099期开奖结果:", "🔴01 02 03 04 05 06", "🔵16", "开奖时间：2026-08-25"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	// 双色球 broadcast has no 「彩票开奖播报」 prefix; others do.
	if strings.HasPrefix(BroadcastBody(r), BroadcastPrefix) {
		t.Fatal("ssq must not carry prefix")
	}
}

func TestParseSportDLTAndAliases(t *testing.T) {
	r, err := ParseSport(Games["dlt"], asMap(mustJSON(t, `{"lotteryDrawNum":"26099","lotteryDrawTime":"2026-08-25","lotteryDrawResult":"01 02 03 04 05 06 07"}`)))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r.Primary, []string{"01", "02", "03", "04", "05"})
	eq(t, r.Secondary, []string{"06", "07"})
	eq(t, ResolveCode("大乐透"), "dlt")
	eq(t, ResolveCode("福彩"), "cwl")
	eq(t, ResolveCode("福彩3D"), "fc3d")
	if !strings.HasPrefix(BroadcastBody(r), BroadcastPrefix) {
		t.Fatal("dlt broadcast must carry prefix")
	}
}

func TestParsePublicRepoHistory(t *testing.T) {
	rows := ParsePublicRepoHistory(Games["dlt"], mustJSON(t, `{"draws":[{"issue":"26096","draw_date":"2026-08-24","number_raw":"08 09 10 11 25 04 12"}]}`), "")
	eq(t, rows[0].Primary, []string{"08", "09", "10", "11", "25"})
	eq(t, rows[0].Secondary, []string{"04", "12"})
	eq(t, rows[0].Source, "sport")
}

func TestHistoryParserAndKeyword(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"result":[`)
	for i := 100; i >= 1; i-- {
		if i != 100 {
			b.WriteString(",")
		}
		b.WriteString(`{"code":"2026` + pad3(i) + `","date":"2026-08-25","red":"01,02,03","blue":""}`)
	}
	b.WriteString(`]}`)
	rows, err := ParseCWLHistory(Games["kl8"], mustJSON(t, b.String()), "")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, len(rows), 100)
	eq(t, rows[0].Issue, "2026100")
	cases := map[string]string{"双色球历史": "ssq", "福彩3D历史": "fc3d", "七乐彩历史": "qlc", "快乐8历史": "kl8",
		"大乐透历史": "dlt", "排列3历史": "pl3", "排列5历史": "pl5", "7星彩历史": "qxc", "双色球 历史": "ssq", "澳门六合彩历史": "macau_lhc"}
	for kw, code := range cases {
		eq(t, ResolveHistoryKeyword(kw), code)
	}
	eq(t, ResolveHistoryKeyword("快乐8"), "")
}

func pad3(i int) string { return fmt.Sprintf("%03d", i) }

func TestMarkSixParsersAndFormatting(t *testing.T) {
	hk := ParseHKJCHistory(Games["hklhc"], mustJSON(t, `{"data":{"lotteryDraws":[{"year":2026,"no":93,"drawDate":"2026-08-25T00:00:00+08:00","status":"Result","drawResult":{"drawnNo":[1,18,19,25,34,38],"xDrawnNo":7}}]}}`))
	eq(t, hk[0].Issue, "2026093")
	eq(t, hk[0].Secondary, []string{"07"})
	eq(t, hk[0].DrawTime, "2026-08-25")

	latest, err := ParseMarksix6Latest(Games["macau_lhc"], mustJSON(t, `{"expect":"2026238","openTime":"2026-08-26 22:32:32","numbers":["16","11","25","36","03","07","48"]}`))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, latest.Primary, []string{"16", "11", "25", "36", "03", "07"})

	page := `<section class="card" id="newMacau">
        <div class="history-line"><span class="period">2026238期</span>
        <span class="ball-sm red">35</span><span class="ball-sm blue">44</span>
        <span class="ball-sm red">23</span><span class="ball-sm blue">04</span>
        <span class="ball-sm green">07</span><span class="ball-sm red">21</span>
        <span class="ball-sm blue">17</span></div></section>`
	rows := ParseMarksix6HistoryHTML(Games["new_macau_lhc"], page)
	eq(t, rows[0].Secondary, []string{"17"})
	eq(t, rows[0].Issue, "2026238")
	eq(t, ResolveCode("香港六合彩"), "hklhc")

	mj := ParseMacaujcPayload(Games["new_macau_lhc"], mustJSON(t, `[{"expect":"2026239","openTime":"2026-08-27 21:32:32","openCode":"47,43,34,17,22,07,05","wave":"blue,green,red,green,green,red,green","zodiac":"猴,鼠,雞,虎,雞,鼠,虎"}]`))[0]
	out := FormatResult(mj)
	for _, want := range []string{"47 43 34 17 22 07 + 05", "猴 鼠 雞 虎 雞 鼠 + 虎", "🔵 🟢 🔴 🟢 🟢 🔴 + 🟢", "macaujc.com"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q", want, out)
		}
	}
	eq(t, mj.Zodiac, []string{"猴", "鼠", "雞", "虎", "雞", "鼠", "虎"})
	eq(t, strings.Count(FormatMarkSixNumbers("2026239", "", mj.Primary, mj.Secondary, mj.Zodiac, mj.Wave), "\n"), 2)
	// {"data": [...]} wrapper is accepted too.
	eq(t, len(ParseMacaujcPayload(Games["new_macau_lhc"], mustJSON(t, `{"data":[{"expect":"1","openCode":"01,02,03,04,05,06,07"}]}`))), 1)
}

func TestParseRealtime(t *testing.T) {
	r, err := ParseRealtime(Games["ssq"], mustJSON(t, `{"errorCode":0,"result":{"data":{"preDrawIssue":"2026099","preDrawTime":"2026-08-27 21:30:00","preDrawCode":"01,12,14,18,30,31,02"}}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r.Source, "realtime168")
	eq(t, r.Primary, []string{"01", "12", "14", "18", "30", "31"})
	eq(t, r.Secondary, []string{"02"})
	r2, err := ParseRealtime(Games["ssq"], mustJSON(t, `{"errorCode":0,"result":{"data":{"preDrawIssue":2026099,"preDrawTime":"2026-08-27 21:30:00","preDrawCode":"01,12,14,18,30,31,02","drawTime":"2026-08-30 21:30:00"}}}`), "")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r2.NextDrawTime, "2026-08-30 21:30:00")
	eq(t, r2.Issue, "2026099")
	if _, err := ParseRealtime(Games["ssq"], mustJSON(t, `{"errorCode":1,"message":"bad"}`), ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseHuiniao(t *testing.T) {
	r, err := ParseHuiniao(Games["fc3d"], mustJSON(t, `{"data":{"last":{"code":"2026103","open_time":"2026-09-10","one":"02","two":"07","three":"2"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r.Primary, []string{"2", "7", "2"})
	r, err = ParseHuiniao(Games["ssq"], mustJSON(t, `{"data":{"last":{"code":"2026099","day":"2026-08-27","one":"1","two":"12","three":"14","four":"18","five":"30","six":"31","seven":"2"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r.Primary, []string{"01", "12", "14", "18", "30", "31"})
	eq(t, r.Secondary, []string{"02"})
}

func TestDrawWindowSchedule(t *testing.T) {
	eq(t, NextDraw("ssq", bj(t, "2026-08-28 22:00:00")).Format("2006-01-02 15:04:05"), "2026-08-30 21:15:00")
	if _, ok := PollWindow("ssq", bj(t, "2026-08-30 21:59:59")); !ok {
		t.Fatal("within 45 minutes should poll")
	}
	if _, ok := PollWindow("ssq", bj(t, "2026-08-30 22:00:01")); ok {
		t.Fatal("after 45 minutes should stop")
	}
	if _, ok := PollWindow("ssq", bj(t, "2026-08-30 21:13:30")); !ok {
		t.Fatal("2 minutes before draw should poll")
	}
	if _, ok := PollWindow("ssq", bj(t, "2026-08-30 21:12:30")); ok {
		t.Fatal("more than 2 minutes before should not poll")
	}
}

func TestThreeDigitClassifyAndFormat(t *testing.T) {
	cases := map[string]string{"222": "豹子", "123": "顺子", "890": "顺子", "901": "顺子", "112": "组三", "272": "组三", "147": "组六"}
	for digits, want := range cases {
		got, err := ClassifyThreeDigit(strings.Split(digits, ""))
		if err != nil || got != want {
			t.Fatalf("%s: %s %v want %s", digits, got, err, want)
		}
	}
	d, _ := NormalizeThreeDigit([]string{"02", "07", "02"})
	eq(t, d, []string{"2", "7", "2"})
	r, err := ParseCWL(Games["fc3d"], asMap(mustJSON(t, `{"code":"2026103","date":"2026-09-10","red":"02,07,02","blue":""}`)), "")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, r.Primary, []string{"2", "7", "2"})
	if !strings.Contains(FormatResult(r), "开奖号码：2 7 2（组三）") {
		t.Fatal(FormatResult(r))
	}
	pl3, err := ParseSport(Games["pl3"], asMap(mustJSON(t, `{"lotteryDrawNum":"25123","lotteryDrawTime":"2026-09-10","lotteryDrawResult":"8 9 0"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(FormatResult(pl3), "开奖号码：8 9 0（顺子）") {
		t.Fatal(FormatResult(pl3))
	}
}

func TestThreeDigitRejectsIncomplete(t *testing.T) {
	for _, bad := range [][]string{{"02"}, {"02", "07"}, {"10", "1", "2"}} {
		if IsValidThreeDigit(bad) {
			t.Fatalf("%v should be invalid", bad)
		}
	}
	if _, err := ParseRealtime(Games["fc3d"], mustJSON(t, `{"errorCode":0,"result":{"data":{"preDrawIssue":"2026103","preDrawTime":"2026-09-10 21:30:00","preDrawCode":"02"}}}`), ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkSixRejectsZeroAndIncomplete(t *testing.T) {
	if IsValidMarkSix([]string{"00", "01", "02", "03", "04", "05"}, []string{"06"}) {
		t.Fatal("00 invalid")
	}
	if IsValidMarkSix([]string{"01", "02", "03", "04", "05"}, []string{"06"}) {
		t.Fatal("5 numbers invalid")
	}
	if _, _, err := NormalizeMarkSixNumbers([]string{"00", "11", "25", "36", "03", "07", "48"}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseMarksix6Latest(Games["macau_lhc"], mustJSON(t, `{"expect":"2026238","numbers":["00","11","25","36","03","07","48"]}`)); err == nil {
		t.Fatal("expected error")
	}
	eq(t, len(ParseMacaujcPayload(Games["new_macau_lhc"], mustJSON(t, `[{"expect":"2026239","openCode":"00,43,34,17,22,07,05"}]`))), 0)
	lines := strings.Split(FormatMarkSixNumbers("2026001", "2026-01-01", []string{"10", "06", "39", "47", "37", "17"}, []string{"14"}, nil, nil), "\n")
	eq(t, lines[0], "10 06 39 47 37 17 + 14")
	eq(t, strings.Count(lines[2], " + "), 1)
}

func TestHistoryPagePaginates(t *testing.T) {
	var rows []Result
	for i := 100; i >= 1; i-- {
		rows = append(rows, Result{Source: "cwl", GameCode: "kl8", GameName: "快乐8", Issue: "2026" + pad3(i), DrawTime: "2026-08-25", Primary: []string{"01", "02", "03"}})
	}
	text, page, pages := HistoryPage("kl8", rows, 0)
	eq(t, page, 0)
	eq(t, pages, 10)
	if !strings.Contains(text, "第 1/10 页") || !strings.Contains(text, "第2026100期") || strings.Contains(text, "第2026089期") {
		t.Fatal(text)
	}
}

func TestIssueComparison(t *testing.T) {
	if !IsNewerIssue("2026002", "2026001") || IsNewerIssue("2026001", "2026001") || IsNewerIssue("26099", "2026099") {
		t.Fatal("issue ordering wrong")
	}
	if !IsNewerIssue("2026100", "") {
		t.Fatal("empty known is always newer")
	}
}

// ---- engine fakes ----

type memRepo struct {
	mu      sync.Mutex
	results map[string][]Result
	subs    map[string][]string // chat -> selectors
	outbox  []string            // chat|code|issue|body
	status  map[string]string
}

func newMemRepo() *memRepo {
	return &memRepo{results: map[string][]Result{}, subs: map[string][]string{}, status: map[string]string{}}
}

func (m *memRepo) LatestIssue(code string) (string, error) {
	best := ""
	for _, r := range m.results[code] {
		if best == "" || CompareIssues(r.Issue, best) > 0 {
			best = r.Issue
		}
	}
	return best, nil
}

func (m *memRepo) SaveResult(r Result) (bool, error) {
	for i, x := range m.results[r.GameCode] {
		if x.Issue == r.Issue {
			m.results[r.GameCode][i] = r
			return false, nil
		}
	}
	m.results[r.GameCode] = append(m.results[r.GameCode], r)
	return true, nil
}

func (m *memRepo) SubscriberChatIDs(code string) ([]string, error) {
	var out []string
	for chat, sels := range m.subs {
		for _, c := range ExpandSelectors(sels) {
			if c == code {
				out = append(out, chat)
				break
			}
		}
	}
	return out, nil
}

func (m *memRepo) Enqueue(chatID, code, issue, body string) (bool, error) {
	key := chatID + "|" + code + "|" + issue
	for _, e := range m.outbox {
		if strings.HasPrefix(e, key+"|") {
			return false, nil
		}
	}
	m.outbox = append(m.outbox, key+"|"+body)
	return true, nil
}

func (m *memRepo) SetSourceStatus(source string, ok bool, errText string) error {
	m.status[source] = errText
	return nil
}

func (m *memRepo) SubscribedCodes() ([]string, error) {
	var all []string
	for _, sels := range m.subs {
		all = append(all, sels...)
	}
	return ExpandSelectors(all), nil
}

type fakeFetcher struct {
	result Result
	called []string
	errs   map[string]string
}

func (f *fakeFetcher) LatestAll(_ context.Context, codes []string, _ map[string]string) ([]Result, map[string]string) {
	f.called = append([]string(nil), codes...)
	if f.result.GameCode == "" {
		return nil, f.errs
	}
	return []Result{f.result}, f.errs
}

func allowAll(string) bool { return true }

func TestNewIssueIsQueuedOnce(t *testing.T) {
	first := Result{Source: "cwl", GameCode: "ssq", GameName: "双色球", Issue: "2026001", DrawTime: "2026-01-01", Primary: []string{"01", "02", "03", "04", "05", "06"}, Secondary: []string{"16"}}
	second := Result{Source: "cwl", GameCode: "ssq", GameName: "双色球", Issue: "2026002", DrawTime: "2026-01-03", Primary: []string{"07", "08", "09", "10", "11", "12"}, Secondary: []string{"15"}}
	repo := newMemRepo()
	repo.subs["-1001"] = []string{"all"}
	f := &fakeFetcher{result: first}
	ctx := context.Background()
	if _, err := Refresh(ctx, repo, f, []string{"ssq"}, allowAll); err != nil {
		t.Fatal(err)
	}
	eq(t, len(repo.outbox), 0) // first sighting = baseline only
	f.result = second
	out, _ := Refresh(ctx, repo, f, []string{"ssq"}, allowAll)
	eq(t, len(repo.outbox), 1)
	eq(t, out.NewIssues, []string{"ssq:2026002"})
	Refresh(ctx, repo, f, []string{"ssq"}, allowAll)
	eq(t, len(repo.outbox), 1)
	if !strings.Contains(repo.outbox[0], "福彩双色球第:2026002期开奖结果:") {
		t.Fatal(repo.outbox[0])
	}
}

func TestOlderIssueFromLaggingSourceIsIgnored(t *testing.T) {
	repo := newMemRepo()
	repo.subs["-1001"] = []string{"sport"}
	repo.results["dlt"] = []Result{{GameCode: "dlt", Issue: "2026099"}}
	f := &fakeFetcher{result: Result{Source: "sport", GameCode: "dlt", GameName: "超级大乐透", Issue: "26099", Primary: []string{"01", "02", "03", "04", "05"}, Secondary: []string{"06", "07"}}}
	Refresh(context.Background(), repo, f, []string{"dlt"}, allowAll)
	eq(t, len(repo.outbox), 0)
	eq(t, len(repo.results["dlt"]), 1)
}

func TestBroadcastSwitchOffStillStoresButDoesNotQueue(t *testing.T) {
	repo := newMemRepo()
	repo.subs["-1001"] = []string{"ssq"}
	repo.results["ssq"] = []Result{{GameCode: "ssq", Issue: "2026001"}}
	f := &fakeFetcher{result: Result{Source: "cwl", GameCode: "ssq", GameName: "双色球", Issue: "2026002", Primary: []string{"01", "02", "03", "04", "05", "06"}, Secondary: []string{"16"}}}
	sw := DefaultSwitches()
	sw.Games["ssq"] = false
	Refresh(context.Background(), repo, f, []string{"ssq"}, sw.AllowBroadcast)
	eq(t, len(repo.outbox), 0)
	eq(t, len(repo.results["ssq"]), 2)
	sw.Games["ssq"] = true
	sw.BroadcastEnabled = false
	if sw.AllowBroadcast("ssq") {
		t.Fatal("master switch off must block")
	}
}

func TestOnlySubscribedChatsReceive(t *testing.T) {
	repo := newMemRepo()
	repo.subs["-1"] = []string{"marksix"}
	repo.subs["-2"] = []string{"cwl"}
	repo.results["hklhc"] = []Result{{GameCode: "hklhc", Issue: "2026092"}}
	f := &fakeFetcher{result: Result{Source: "hkjc", GameCode: "hklhc", GameName: "香港六合彩", Issue: "2026093", Primary: []string{"01", "18", "19", "25", "34", "38"}, Secondary: []string{"07"}}}
	Refresh(context.Background(), repo, f, []string{"hklhc"}, allowAll)
	eq(t, len(repo.outbox), 1)
	if !strings.HasPrefix(repo.outbox[0], "-1|hklhc|2026093|"+BroadcastPrefix) {
		t.Fatal(repo.outbox[0])
	}
}

func TestInvalidThreeDigitNotSavedOrBroadcast(t *testing.T) {
	repo := newMemRepo()
	repo.subs["-1001"] = []string{"fc3d"}
	f := &fakeFetcher{result: Result{Source: "huiniao", GameCode: "fc3d", GameName: "福彩3D", Issue: "2026999", DrawTime: "2026-09-10", Primary: []string{"02"}}}
	Refresh(context.Background(), repo, f, []string{"fc3d"}, allowAll)
	eq(t, len(repo.results["fc3d"]), 0)
	eq(t, len(repo.outbox), 0)
	repo.SaveResult(Result{Source: "cwl", GameCode: "fc3d", GameName: "福彩3D", Issue: "2026998", DrawTime: "2026-09-09", Primary: []string{"1", "2", "3"}})
	f.result = Result{Source: "cwl", GameCode: "fc3d", GameName: "福彩3D", Issue: "2026999", DrawTime: "2026-09-10", Primary: []string{"2", "7", "2"}}
	Refresh(context.Background(), repo, f, []string{"fc3d"}, allowAll)
	eq(t, len(repo.outbox), 1)
	if !strings.Contains(repo.outbox[0], "开奖号码：2 7 2（组三）") {
		t.Fatal(repo.outbox[0])
	}
}

func TestPollAlwaysIncludesSubscribedCodesOutsideWindow(t *testing.T) {
	repo := newMemRepo()
	repo.subs["-2001"] = []string{"ssq", "dlt"}
	far := bj(t, "2026-08-26 12:00:00")
	if _, ok := PollWindow("ssq", far); ok {
		t.Fatal("unexpected window")
	}
	f := &fakeFetcher{}
	p := &Poller{Repo: repo, Fetcher: f, Now: func() time.Time { return far }}
	codes, err := p.Tick(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	eq(t, codes, []string{"ssq", "dlt"})
	eq(t, f.called, []string{"ssq", "dlt"})
	// Within 300s no second slow poll.
	codes, _ = p.Tick(context.Background())
	eq(t, len(codes), 0)
}

func TestPollFastWindowOnlyPollsDueGames(t *testing.T) {
	repo := newMemRepo()
	repo.subs["-1"] = []string{"all"}
	at := bj(t, "2026-08-30 21:20:00") // Sunday: ssq 21:15, fc3d 21:15, kl8 21:30 (-2min no), pl3/pl5/qxc 21:25 (no, >2min)
	f := &fakeFetcher{}
	p := &Poller{Repo: repo, Fetcher: f, Now: func() time.Time { return at }}
	codes, _ := p.Tick(context.Background())
	eq(t, codes, []string{"ssq", "fc3d"})
}

func TestPollSkipsWhenQueryDisabledOrNoSubscribers(t *testing.T) {
	repo := newMemRepo()
	f := &fakeFetcher{}
	p := &Poller{Repo: repo, Fetcher: f}
	codes, _ := p.Tick(context.Background())
	eq(t, len(codes), 0)
	repo.subs["-1"] = []string{"all"}
	p.Switches = func() (Switches, error) { s := DefaultSwitches(); s.QueryEnabled = false; return s, nil }
	codes, _ = p.Tick(context.Background())
	eq(t, len(codes), 0)
}

func TestSelectorsExpandAndLabels(t *testing.T) {
	eq(t, CodesForSelector("cwl"), []string{"ssq", "fc3d", "qlc", "kl8"})
	eq(t, CodesForSelector("sport"), []string{"dlt", "pl3", "pl5", "qxc"})
	eq(t, len(CodesForSelector("all")), 11)
	eq(t, SelectorLabel("marksix"), "全部六合彩")
	eq(t, ExpandSelectors([]string{"hklhc", "marksix", "ssq"}), []string{"ssq", "hklhc", "macau_lhc", "new_macau_lhc"})
}

// ---- history backfill ----

type memHistory struct {
	*memRepo
	counts map[string]int // optional override; nil → derive from results
}

func newMemHistory() *memHistory {
	return &memHistory{memRepo: newMemRepo()}
}

func (m *memHistory) CountResults(code string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counts != nil {
		return m.counts[code], nil
	}
	return len(m.results[code]), nil
}

func (m *memHistory) TrimHistory(code string, keep int) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := make([]ResultRow, 0, len(m.results[code]))
	for i, r := range m.results[code] {
		rows = append(rows, ResultRow{ID: uint64(i + 1), Issue: r.Issue, GameCode: code})
	}
	ids := TrimIDsBeyondKeep(rows, keep)
	if len(ids) == 0 {
		return 0, nil
	}
	drop := map[uint64]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	var kept []Result
	for i, r := range m.results[code] {
		if !drop[uint64(i+1)] {
			kept = append(kept, r)
		}
	}
	m.results[code] = kept
	return int64(len(ids)), nil
}

type fakeHistoryFetcher struct {
	byCode map[string][]Result
	errs   map[string]string
	calls  []string
}

func (f *fakeHistoryFetcher) CacheHistory(_ context.Context, code string, limit int) ([]Result, error) {
	f.calls = append(f.calls, code)
	if e, ok := f.errs[code]; ok {
		return nil, fmt.Errorf("%s", e)
	}
	rows := append([]Result(nil), f.byCode[code]...)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func ssqRow(issue string) Result {
	return Result{Source: "cwl", GameCode: "ssq", GameName: "双色球", Issue: issue, DrawTime: "2026-01-01",
		Primary: []string{"01", "02", "03", "04", "05", "06"}, Secondary: []string{"16"}}
}

func TestParseSportHistoryList(t *testing.T) {
	payload := mustJSON(t, `{"value":{"list":[
		{"lotteryDrawNum":"26099","lotteryDrawTime":"2026-08-25","lotteryDrawResult":"01 02 03 04 05 06 07"},
		{"lotteryDrawNum":"26098","lotteryDrawTime":"2026-08-23","lotteryDrawResult":"08 09 10 11 12 13 14"}
	]}}`)
	rows, err := ParseSportHistory(Games["dlt"], payload)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, len(rows), 2)
	eq(t, rows[0].Issue, "26099")
	eq(t, rows[0].Primary, []string{"01", "02", "03", "04", "05"})
	eq(t, rows[1].Issue, "26098")
}

func TestParseHKJCAndMacaujcHistoryLists(t *testing.T) {
	hk := ParseHKJCHistory(Games["hklhc"], mustJSON(t, `{"data":{"lotteryDraws":[
		{"year":2026,"no":93,"drawDate":"2026-08-25T00:00:00+08:00","status":"Result","drawResult":{"drawnNo":[1,18,19,25,34,38],"xDrawnNo":7}},
		{"year":2026,"no":92,"drawDate":"2026-08-22T00:00:00+08:00","status":"Result","drawResult":{"drawnNo":[2,3,4,5,6,7],"xDrawnNo":8}}
	]}}`))
	eq(t, len(hk), 2)
	eq(t, hk[0].Issue, "2026093")
	eq(t, hk[1].Issue, "2026092")

	mj := ParseMacaujcPayload(Games["new_macau_lhc"], mustJSON(t, `[
		{"expect":"2026239","openTime":"2026-08-27 21:32:32","openCode":"47,43,34,17,22,07,05"},
		{"expect":"2026238","openTime":"2026-08-26 21:32:32","openCode":"01,02,03,04,05,06,07"}
	]`))
	eq(t, len(mj), 2)
	eq(t, mj[0].Issue, "2026239")
}

func TestParsePublicRepoHistoryList(t *testing.T) {
	rows := ParsePublicRepoHistory(Games["ssq"], mustJSON(t, `{"draws":[
		{"issue":"2026100","draw_date":"2026-08-26","number_raw":"01 02 03 04 05 06 07"},
		{"issue":"2026099","draw_date":"2026-08-24","number_raw":"08 09 10 11 12 13 14"}
	]}`), "")
	eq(t, len(rows), 2)
	eq(t, rows[0].Issue, "2026100")
	eq(t, rows[0].Secondary, []string{"07"})
}

func TestTrimIDsBeyondKeep(t *testing.T) {
	var rows []ResultRow
	for i := 1; i <= 105; i++ {
		rows = append(rows, ResultRow{ID: uint64(i), Issue: fmt.Sprintf("2026%03d", i)})
	}
	ids := TrimIDsBeyondKeep(rows, 100)
	eq(t, len(ids), 5)
	// Oldest five (issues 001-005) should be trimmed.
	want := map[uint64]bool{1: true, 2: true, 3: true, 4: true, 5: true}
	for _, id := range ids {
		if !want[id] {
			t.Fatalf("unexpected trim id %d", id)
		}
	}
	eq(t, len(TrimIDsBeyondKeep(rows[:100], 100)), 0)
}

func TestBackfillDoesNotEnqueueBroadcasts(t *testing.T) {
	store := newMemHistory()
	store.subs["-1001"] = []string{"all"}
	var hist []Result
	for i := 100; i >= 1; i-- {
		hist = append(hist, ssqRow(fmt.Sprintf("2026%03d", i)))
	}
	fetcher := &fakeHistoryFetcher{byCode: map[string][]Result{"ssq": hist}}
	out := BackfillGame(context.Background(), store, fetcher, "ssq")
	eq(t, out.Err, "")
	eq(t, out.Saved, 100)
	eq(t, len(store.outbox), 0) // backfill never broadcasts
	n, _ := store.CountResults("ssq")
	eq(t, n, 100)
	issue, _ := store.LatestIssue("ssq")
	eq(t, issue, "2026100")
}

func TestNewIssueAfterBackfillBroadcastsOnce(t *testing.T) {
	store := newMemHistory()
	store.subs["-1001"] = []string{"ssq"}
	var hist []Result
	for i := 50; i >= 1; i-- {
		hist = append(hist, ssqRow(fmt.Sprintf("2026%03d", i)))
	}
	fetcher := &fakeHistoryFetcher{byCode: map[string][]Result{"ssq": hist}}
	BackfillGame(context.Background(), store, fetcher, "ssq")
	eq(t, len(store.outbox), 0)

	// A brand-new issue after backfill must still broadcast exactly once.
	live := &fakeFetcher{result: ssqRow("2026051")}
	Refresh(context.Background(), store.memRepo, live, []string{"ssq"}, allowAll)
	eq(t, len(store.outbox), 1)
	Refresh(context.Background(), store.memRepo, live, []string{"ssq"}, allowAll)
	eq(t, len(store.outbox), 1)
	if !strings.Contains(store.outbox[0], "第:2026051期") {
		t.Fatal(store.outbox[0])
	}
}

func TestBackfillSkipsNewerThanKnown(t *testing.T) {
	store := newMemHistory()
	// Baseline already at 2026050 (first-seen by poller).
	store.results["ssq"] = []Result{ssqRow("2026050")}
	fetcher := &fakeHistoryFetcher{byCode: map[string][]Result{"ssq": {
		ssqRow("2026051"), // live new — must NOT be claimed by backfill
		ssqRow("2026050"),
		ssqRow("2026049"),
	}}}
	out := BackfillGame(context.Background(), store, fetcher, "ssq")
	eq(t, out.Err, "")
	issue, _ := store.LatestIssue("ssq")
	eq(t, issue, "2026050") // 2026051 still free for Refresh to broadcast
	n, _ := store.CountResults("ssq")
	eq(t, n, 2) // 50 + 49

	store.subs["-1"] = []string{"ssq"}
	Refresh(context.Background(), store.memRepo, &fakeFetcher{result: ssqRow("2026051")}, []string{"ssq"}, allowAll)
	eq(t, len(store.outbox), 1)
}

func TestBackfillAllContinuesOnPerGameFailure(t *testing.T) {
	store := newMemHistory()
	fetcher := &fakeHistoryFetcher{
		byCode: map[string][]Result{"ssq": {ssqRow("2026001")}},
		errs:   map[string]string{"dlt": "sporttery down"},
	}
	outs := BackfillAll(context.Background(), store, fetcher, []string{"ssq", "dlt"})
	eq(t, len(outs), 2)
	var sawFail, sawOK bool
	for _, o := range outs {
		if o.Code == "dlt" && o.Err != "" {
			sawFail = true
		}
		if o.Code == "ssq" && o.Err == "" {
			sawOK = true
		}
	}
	if !sawFail || !sawOK {
		t.Fatalf("outs=%+v", outs)
	}
	eq(t, store.status["sport"], "sporttery down")
}

func TestCodesNeedingBackfill(t *testing.T) {
	store := newMemHistory()
	store.counts = map[string]int{"ssq": 100, "dlt": 3, "fc3d": 0}
	need, err := CodesNeedingBackfill(store, []string{"ssq", "dlt", "fc3d"}, 100)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, need, []string{"dlt", "fc3d"})
}

func TestBackfillTrimsTo100(t *testing.T) {
	store := newMemHistory()
	// Empty cache: fetch returns 100; after insert+trim we keep exactly 100.
	var hist []Result
	for i := 110; i >= 1; i-- { // fetcher will be capped by BackfillGame to 100
		hist = append(hist, ssqRow(fmt.Sprintf("2026%03d", i)))
	}
	out := BackfillGame(context.Background(), store, &fakeHistoryFetcher{byCode: map[string][]Result{"ssq": hist}}, "ssq")
	eq(t, out.Err, "")
	n, _ := store.CountResults("ssq")
	eq(t, n, 100)
	issue, _ := store.LatestIssue("ssq")
	eq(t, issue, "2026110")

	// Extra older rows (below the known latest) get trimmed away.
	for i := 1; i <= 20; i++ {
		_, _ = store.SaveResult(ssqRow(fmt.Sprintf("2025%03d", i)))
	}
	n, _ = store.CountResults("ssq")
	if n <= 100 {
		t.Fatalf("expected >100 before trim, got %d", n)
	}
	trimmed, err := store.TrimHistory("ssq", 100)
	if err != nil {
		t.Fatal(err)
	}
	if trimmed < 20 {
		t.Fatalf("trimmed=%d", trimmed)
	}
	n, _ = store.CountResults("ssq")
	eq(t, n, 100)
	issue, _ = store.LatestIssue("ssq")
	eq(t, issue, "2026110")
}

func TestParseHuiniaoHistoryList(t *testing.T) {
	payload := mustJSON(t, `{"code":1,"data":{"data":{"list":[
		{"code":"2026113","day":"2026-09-29","one":"03","two":"04","three":"20","four":"24","five":"29","six":"30","seven":"11","open_time":"2026-09-29 21:15:00"},
		{"code":"2026112","day":"2026-09-27","one":"01","two":"04","three":"11","four":"12","five":"17","six":"29","seven":"11","open_time":"2026-09-27 21:15:00"}
	]}}}`)
	rows := ParseHuiniaoHistory(Games["ssq"], payload)
	eq(t, len(rows), 2)
	eq(t, rows[0].Issue, "2026113")
	eq(t, rows[0].Primary, []string{"03", "04", "20", "24", "29", "30"})
	eq(t, rows[0].Secondary, []string{"11"})
	eq(t, rows[0].Source, "huiniao")
}

func TestParseRealtimeHistoryList(t *testing.T) {
	payload := mustJSON(t, `{"errorCode":0,"result":{"data":[
		{"preDrawIssue":2026113,"preDrawTime":"2026-09-29 21:30:00","preDrawCode":"03,04,20,24,29,30,11"},
		{"preDrawIssue":2026112,"preDrawTime":"2026-09-27 21:30:00","preDrawCode":"01,04,11,12,17,29,11"}
	]}}`)
	rows, err := ParseRealtimeHistory(Games["ssq"], payload, "")
	if err != nil {
		t.Fatal(err)
	}
	eq(t, len(rows), 2)
	eq(t, rows[0].Issue, "2026113")
	eq(t, rows[0].Source, "realtime168")
}

func TestMergeHistorySourcesPrefersEarlier(t *testing.T) {
	game := Games["ssq"]
	sources := []historySource{
		{"a", func(context.Context) ([]Result, error) {
			return []Result{ssqRow("2026002"), ssqRow("2026001")}, nil
		}},
		{"b", func(context.Context) ([]Result, error) {
			r := ssqRow("2026002")
			r.Source = "other"
			return []Result{r, ssqRow("2026003")}, nil
		}},
	}
	rows, err := mergeHistorySources(context.Background(), game, 100, sources, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, len(rows), 3)
	eq(t, rows[0].Issue, "2026003")
	eq(t, rows[1].Source, "cwl") // earlier source kept for 2026002
}
