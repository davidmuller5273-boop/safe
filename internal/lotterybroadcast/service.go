package lotterybroadcast

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Data source endpoints (same as lottery.py / config.py).
const (
	HKJCGraphQLURL     = "https://info.cld.hkjc.com/graphql/base/"
	Marksix6APIURL     = "https://api3.marksix6.net/lottery_api.php"
	Marksix6HistoryURL = "https://api2.marksix6.net/"
	MacaujcLatestURL   = "https://macaumarksix.com/api/macaujc2.com"
	MacaujcHistoryURL  = "https://history.macaumarksix.com/history/macaujc2/y/%d"
	// 澳门六合彩 (old, 22:32) — same draw that Marksix6 "macau" mirrors.
	MacauOldLatestURL  = "https://macaumarksix.com/api/macaujc.com"
	MacauOldHistoryURL = "https://history.macaumarksix.com/history/macaujc/y/%d"
	// List endpoints used only for the 100-issue history cache.
	HuiniaoHistoryLimit = 100
	RealtimeHistoryPath = "QuanGuoCai/getHistoryLotteryInfo.do"
	HuiniaoURL          = "https://api.huiniao.top/interface/home/lotteryHistory"
	PublicRepoJSDelivr  = "https://cdn.jsdelivr.net/gh/wenjinliuu/lottery-data-repo@main/public_data/draws/%s.json"
	PublicRepoGitHub    = "https://api.github.com/repos/wenjinliuu/lottery-data-repo/contents/public_data/draws/%s.json"
)

const hkjcQuery = "\n        fragment lotteryDrawsFragment on LotteryDraw {\n    id\n    year\n    no\n" +
	"    openDate\n    closeDate\n    drawDate\n    status\n    snowballCode\n" +
	"    snowballName_en\n    snowballName_ch\n    lotteryPool {\n      sell\n      status\n" +
	"      totalInvestment\n      jackpot\n      unitBet\n      estimatedPrize\n" +
	"      derivedFirstPrizeDiv\n      lotteryPrizes {\n        type\n        winningUnit\n" +
	"        dividend\n      }\n    }\n    drawResult {\n      drawnNo\n      xDrawnNo\n    }\n" +
	"  }\n        query marksixResult($lastNDraw: Int, $startDate: String, $endDate: String, " +
	"$drawType: LotteryDrawType) {\n            lotteryDraws(lastNDraw: $lastNDraw, " +
	"startDate: $startDate, endDate: $endDate, drawType: $drawType) {\n" +
	"              ...lotteryDrawsFragment\n            }\n        }\n    "

var cwlReferers = map[string]string{
	"ssq":  "https://www.cwl.gov.cn/ygkj/wqkjgg/ssq/",
	"fc3d": "https://www.cwl.gov.cn/ygkj/wqkjgg/fc3d/",
	"qlc":  "https://www.cwl.gov.cn/ygkj/wqkjgg/qlc/",
	"kl8":  "https://www.cwl.gov.cn/ygkj/wqkjgg/kl8/",
}

var realtimeCodes = map[string][2]string{
	"ssq":  {"10039", "QuanGuoCai/getLotteryInfo.do"},
	"dlt":  {"10040", "QuanGuoCai/getLotteryInfo.do"},
	"fc3d": {"10041", "QuanGuoCai/getLotteryInfo1.do"},
	"qlc":  {"10042", "QuanGuoCai/getLotteryInfo.do"},
	"pl3":  {"10043", "QuanGuoCai/getLotteryInfo1.do"},
	"pl5":  {"10044", "QuanGuoCai/getLotteryInfo.do"},
	"qxc":  {"10045", "QuanGuoCai/getLotteryInfo.do"},
}

var huiniaoCodes = map[string]string{
	"ssq": "ssq", "fc3d": "fcsd", "qlc": "qlc", "kl8": "klb",
	"dlt": "dlt", "pl3": "pls", "pl5": "plw", "qxc": "qxc",
}

// Sources holds overridable endpoints (env names identical to the Python config).
type Sources struct {
	CWLURL         string // CWL_LOTTERY_URL
	SportteryURL   string // SPORTTERY_LOTTERY_URL
	RealtimeURL    string // LOTTERY_REALTIME_URL
	PublicDataBase string // LOTTERY_PUBLIC_DATA_BASE_URL
}

// SourcesFromEnv returns defaults, overridden by optional environment variables.
func SourcesFromEnv() Sources {
	get := func(key, def string) string {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
		return def
	}
	return Sources{
		CWLURL:         get("CWL_LOTTERY_URL", "https://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice"),
		SportteryURL:   get("SPORTTERY_LOTTERY_URL", "https://webapi.sporttery.cn/gateway/lottery/getHistoryPageListV1.qry"),
		RealtimeURL:    strings.TrimRight(get("LOTTERY_REALTIME_URL", "https://api.api16868.com"), "/"),
		PublicDataBase: strings.TrimRight(get("LOTTERY_PUBLIC_DATA_BASE_URL", "https://raw.githubusercontent.com/wenjinliuu/lottery-data-repo/main/public_data"), "/"),
	}
}

// Service fetches draw results from several providers (LotteryService).
type Service struct {
	Sources Sources
	// ProviderTimeout bounds each provider (Python: asyncio.wait_for(..., 9)).
	ProviderTimeout time.Duration
	// CacheSourceTimeout bounds each history-cache source (0 = 20s).
	CacheSourceTimeout time.Duration
	// providersFn is overridable in tests.
	providersFn func(game Game) []provider
	headers     map[string]string
}

type provider func(ctx context.Context) (Result, error)

// NewService builds a Service with Python-equivalent headers.
func NewService(src Sources) *Service {
	s := &Service{Sources: src, ProviderTimeout: 9 * time.Second, headers: map[string]string{
		"accept":          "application/json, text/plain, */*",
		"accept-language": "zh-CN,zh;q=0.9,en;q=0.8",
		"cache-control":   "no-cache",
		"pragma":          "no-cache",
		"user-agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	}}
	s.providersFn = s.latestProviders
	return s
}

// Latest mirrors LotteryService.latest: run all providers concurrently, keep valid
// results, prefer issues newer than knownIssue, and pick the highest issue
// (macaujc wins ties).
func (s *Service) Latest(ctx context.Context, code, knownIssue string) (Result, error) {
	game, ok := Games[code]
	if !ok {
		return Result{}, errors.New("未知彩种")
	}
	providers := s.providersFn(game)
	type outcome struct {
		r   Result
		err error
	}
	outs := make([]outcome, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func(i int, p provider) {
			defer wg.Done()
			pctx, cancel := context.WithTimeout(ctx, s.ProviderTimeout)
			defer cancel()
			r, err := p(pctx)
			outs[i] = outcome{r, err}
		}(i, p)
	}
	wg.Wait()
	var results []Result
	var errs []string
	for _, o := range outs {
		if o.err != nil {
			errs = append(errs, o.err.Error())
		} else {
			results = append(results, o.r)
		}
	}
	if len(results) == 0 {
		return Result{}, fmt.Errorf("%s多数据源均查询失败：%s", game.Name, strings.Join(errs, "；"))
	}
	var valid []Result
	for _, r := range results {
		if IsMarkSix(r.GameCode) || IsThreeDigit(r.GameCode) {
			if !IsValidResult(r) {
				continue
			}
			nr, err := NormalizeResult(r)
			if err != nil {
				continue
			}
			valid = append(valid, nr)
		} else {
			valid = append(valid, r)
		}
	}
	if len(valid) == 0 {
		if len(errs) == 0 {
			errs = []string{"结果校验未通过"}
		}
		return Result{}, fmt.Errorf("%s多数据源均查询失败：%s", game.Name, strings.Join(errs, "；"))
	}
	var newer []Result
	for _, r := range valid {
		if knownIssue == "" || CompareIssues(r.Issue, knownIssue) > 0 {
			newer = append(newer, r)
		}
	}
	candidates := newer
	if len(candidates) == 0 {
		candidates = valid
	}
	return pickBest(candidates), nil
}

func pickBest(candidates []Result) Result {
	best := candidates[0]
	for _, r := range candidates[1:] {
		c := CompareIssues(r.Issue, best.Issue)
		if c > 0 || (c == 0 && r.Source == "macaujc" && best.Source != "macaujc") {
			best = r
		}
	}
	return best
}

// LatestAll mirrors latest_all: one Latest per code, errors keyed by code.
func (s *Service) LatestAll(ctx context.Context, codes []string, known map[string]string) ([]Result, map[string]string) {
	if len(codes) == 0 {
		codes = GameOrder
	}
	type outcome struct {
		r   Result
		err error
	}
	outs := make([]outcome, len(codes))
	var wg sync.WaitGroup
	for i, code := range codes {
		wg.Add(1)
		go func(i int, code string) {
			defer wg.Done()
			r, err := s.Latest(ctx, code, known[code])
			outs[i] = outcome{r, err}
		}(i, code)
	}
	wg.Wait()
	var results []Result
	errs := map[string]string{}
	for i, o := range outs {
		if o.err != nil {
			errs[codes[i]] = o.err.Error()
		} else {
			results = append(results, o.r)
		}
	}
	return results, errs
}

// History mirrors LotteryService.history (up to 100 issues, newest first).
func (s *Service) History(ctx context.Context, code string, limit int) ([]Result, error) {
	game, ok := Games[code]
	if !ok {
		return nil, errors.New("未知彩种")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	switch game.Source {
	case "cwl":
		return s.historyCWL(ctx, game, limit)
	case "sport":
		return s.historySport(ctx, game, limit)
	case "hkjc":
		return s.historyHKJC(ctx, game, limit)
	case "macaujc":
		return s.historyMacaujc(ctx, game, limit)
	}
	return s.historyMarksix6(ctx, game, limit)
}

func (s *Service) latestProviders(game Game) []provider {
	ps := []provider{
		func(ctx context.Context) (Result, error) { return s.latestNative(ctx, game) },
		func(ctx context.Context) (Result, error) { return s.latestHuiniao(ctx, game) },
		func(ctx context.Context) (Result, error) {
			return s.latestPublicURL(ctx, game, s.Sources.PublicDataBase+"/draws/"+game.Code+".json", "public-raw")
		},
		func(ctx context.Context) (Result, error) {
			return s.latestPublicURL(ctx, game, fmt.Sprintf(PublicRepoJSDelivr, game.Code), "public-jsdelivr")
		},
		func(ctx context.Context) (Result, error) { return s.latestPublicGitHub(ctx, game) },
	}
	if _, ok := realtimeCodes[game.Code]; ok {
		ps = append([]provider{func(ctx context.Context) (Result, error) { return s.latestRealtime(ctx, game) }}, ps...)
	}
	if game.Code == "hklhc" || game.Code == "new_macau_lhc" {
		ps = append(ps, func(ctx context.Context) (Result, error) {
			rows, err := s.historyMarksix6(ctx, game, 1)
			if err != nil {
				return Result{}, err
			}
			return rows[0], nil
		})
	}
	return ps
}

// ---- HTTP helpers ----

func (s *Service) newClient(withJar bool) *http.Client {
	c := &http.Client{Timeout: 20 * time.Second}
	if withJar {
		jar, _ := cookiejar.New(nil)
		c.Jar = jar
	}
	return c
}

func (s *Service) get(ctx context.Context, client *http.Client, rawURL string, params url.Values, extra map[string]string) ([]byte, error) {
	if len(params) > 0 {
		sep := "?"
		if strings.Contains(rawURL, "?") {
			sep = "&"
		}
		rawURL += sep + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return s.send(client, req, extra)
}

func (s *Service) send(client *http.Client, req *http.Request, extra map[string]string) ([]byte, error) {
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

func (s *Service) getJSON(ctx context.Context, client *http.Client, rawURL string, params url.Values, extra map[string]string) (any, error) {
	body, err := s.get(ctx, client, rawURL, params, extra)
	if err != nil {
		return nil, err
	}
	return DecodeJSON(body)
}

// ---- providers ----

func (s *Service) latestRealtime(ctx context.Context, game Game) (Result, error) {
	rc := realtimeCodes[game.Code]
	u := s.Sources.RealtimeURL + "/" + rc[1]
	payload, err := s.getJSON(ctx, s.newClient(false), u, url.Values{"lotCode": {rc[0]}}, nil)
	if err == nil {
		var r Result
		if r, err = ParseRealtime(game, payload, u); err == nil {
			return r, nil
		}
	}
	return Result{}, fmt.Errorf("%s实时接口查询失败：%v", game.Name, err)
}

func (s *Service) latestNative(ctx context.Context, game Game) (Result, error) {
	var rows []Result
	var err error
	switch game.Source {
	case "cwl":
		rows, err = s.historyCWL(ctx, game, 1)
	case "sport":
		rows, err = s.historySport(ctx, game, 1)
	default:
		rows, err = s.History(ctx, game.Code, 1)
	}
	if err != nil {
		return Result{}, err
	}
	return rows[0], nil
}

func (s *Service) latestHuiniao(ctx context.Context, game Game) (Result, error) {
	code, ok := huiniaoCodes[game.Code]
	if !ok {
		return Result{}, fmt.Errorf("慧鸟接口暂不支持%s", game.Name)
	}
	payload, err := s.getJSON(ctx, s.newClient(false), HuiniaoURL, url.Values{"type": {code}, "page": {"1"}, "limit": {"1"}}, nil)
	if err == nil {
		var r Result
		if r, err = ParseHuiniao(game, payload); err == nil {
			return r, nil
		}
	}
	return Result{}, fmt.Errorf("慧鸟 %s 查询失败：%v", game.Name, err)
}

func (s *Service) latestPublicURL(ctx context.Context, game Game, u, source string) (Result, error) {
	payload, err := s.getJSON(ctx, s.newClient(false), u, nil, nil)
	if err == nil {
		rows := ParsePublicRepoHistory(game, payload, u)
		if len(rows) > 0 {
			r := rows[0]
			r.Source = source
			return r, nil
		}
		err = errors.New("接口返回缺少开奖记录")
	}
	return Result{}, fmt.Errorf("%s %s 查询失败：%v", source, game.Name, err)
}

func (s *Service) latestPublicGitHub(ctx context.Context, game Game) (Result, error) {
	u := fmt.Sprintf(PublicRepoGitHub, game.Code)
	payload, err := s.getJSON(ctx, s.newClient(false), u, nil, nil)
	if err == nil {
		encoded := strings.ReplaceAll(str(asMap(payload)["content"]), "\n", "")
		var decoded []byte
		if decoded, err = base64.StdEncoding.DecodeString(encoded); err == nil {
			var inner any
			if inner, err = DecodeJSON(decoded); err == nil {
				rows := ParsePublicRepoHistory(game, inner, u)
				if len(rows) > 0 {
					r := rows[0]
					r.Source = "public-github"
					return r, nil
				}
				err = errors.New("接口返回缺少开奖记录")
			}
		}
	}
	return Result{}, fmt.Errorf("GitHub %s 查询失败：%v", game.Name, err)
}

func (s *Service) historyPublicRepo(ctx context.Context, game Game, limit int) ([]Result, error) {
	u := s.Sources.PublicDataBase + "/draws/" + game.Code + ".json"
	payload, err := s.getJSON(ctx, s.newClient(false), u, nil, map[string]string{"accept": "application/json"})
	if err != nil {
		return nil, err
	}
	rows := ParsePublicRepoHistory(game, payload, u)
	if len(rows) == 0 {
		return nil, errors.New("备用源返回缺少开奖记录")
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (s *Service) historyCWL(ctx context.Context, game Game, limit int) ([]Result, error) {
	pageSize := limit
	if pageSize > 100 {
		pageSize = 100
	}
	referer := cwlReferers[game.Code]
	if referer == "" {
		referer = "https://www.cwl.gov.cn/ygkj/wqkjgg/"
	}
	extra := map[string]string{"referer": referer}
	client := s.newClient(true)
	results, err := func() ([]Result, error) {
		// Warm-up request for cookies (errors ignored like the Python client which only awaits it).
		if _, err := s.get(ctx, client, referer, nil, extra); err != nil && ctx.Err() != nil {
			return nil, err
		}
		var out []Result
		seen := map[string]bool{}
		pages := (limit + pageSize - 1) / pageSize
		for pageNo := 1; pageNo <= pages; pageNo++ {
			params := url.Values{"name": {game.APICode}, "issueCount": {""}, "issueStart": {""}, "issueEnd": {""},
				"dayStart": {""}, "dayEnd": {""}, "pageNo": {strconv.Itoa(pageNo)}, "pageSize": {strconv.Itoa(pageSize)},
				"week": {""}, "systemType": {"PC"}}
			payload, err := s.getJSON(ctx, client, s.Sources.CWLURL, params, extra)
			if err != nil {
				return nil, err
			}
			page, err := ParseCWLHistory(game, payload, s.Sources.CWLURL)
			if err != nil {
				return nil, err
			}
			for _, r := range page {
				if !seen[r.Issue] {
					seen[r.Issue] = true
					out = append(out, r)
				}
			}
			if len(page) < pageSize || len(out) >= limit {
				break
			}
		}
		if len(out) == 0 {
			return nil, errors.New("官方返回缺少开奖记录")
		}
		if len(out) > limit {
			out = out[:limit]
		}
		return out, nil
	}()
	if err == nil {
		return results, nil
	}
	fallback, ferr := s.historyPublicRepo(ctx, game, limit)
	if ferr == nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("中国福彩网 %s 查询失败：%v；备用源失败：%v", game.Name, err, ferr)
}

func (s *Service) historySport(ctx context.Context, game Game, limit int) ([]Result, error) {
	pageSize := limit
	if pageSize > 100 {
		pageSize = 100
	}
	extra := map[string]string{
		"referer":    "https://m.lottery.gov.cn/",
		"origin":     "https://m.lottery.gov.cn",
		"user-agent": "Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Mobile Safari/537.36",
	}
	client := s.newClient(false)
	results, err := func() ([]Result, error) {
		var out []Result
		seen := map[string]bool{}
		pages := (limit + pageSize - 1) / pageSize
		for pageNo := 1; pageNo <= pages; pageNo++ {
			params := url.Values{"gameNo": {game.APICode}, "provinceId": {"0"}, "pageSize": {strconv.Itoa(pageSize)},
				"isVerify": {"1"}, "pageNo": {strconv.Itoa(pageNo)}}
			payload, err := s.getJSON(ctx, client, s.Sources.SportteryURL, params, extra)
			if err != nil {
				return nil, err
			}
			page, err := ParseSportHistory(game, payload)
			if err != nil {
				return nil, err
			}
			for _, r := range page {
				if !seen[r.Issue] {
					seen[r.Issue] = true
					out = append(out, r)
				}
			}
			if len(page) < pageSize || len(out) >= limit {
				break
			}
		}
		if len(out) == 0 {
			return nil, errors.New("官方返回缺少开奖记录")
		}
		if len(out) > limit {
			out = out[:limit]
		}
		return out, nil
	}()
	if err == nil {
		return results, nil
	}
	fallback, ferr := s.historyPublicRepo(ctx, game, limit)
	if ferr == nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("中国体彩网 %s 查询失败：%v；备用源失败：%v", game.Name, err, ferr)
}

func (s *Service) historyHKJC(ctx context.Context, game Game, limit int) ([]Result, error) {
	body := fmt.Sprintf(`{"query":%s,"variables":{"lastNDraw":%d}}`, strconv.Quote(hkjcQuery), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, HKJCGraphQLURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	raw, err := s.send(s.newClient(false), req, map[string]string{
		"content-type": "application/json", "origin": "https://bet.hkjc.com", "referer": "https://bet.hkjc.com/",
	})
	if err == nil {
		var payload any
		if payload, err = DecodeJSON(raw); err == nil {
			if e := asMap(payload)["errors"]; e != nil && len(asList(e)) > 0 {
				err = fmt.Errorf("%v", e)
			} else {
				rows := ParseHKJCHistory(game, payload)
				if len(rows) > 0 {
					if len(rows) > limit {
						rows = rows[:limit]
					}
					return rows, nil
				}
				err = errors.New("官方返回缺少开奖记录")
			}
		}
	}
	return nil, fmt.Errorf("香港赛马会 %s 查询失败：%v", game.Name, err)
}

func sortByIssueDesc(rows []Result) {
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Issue > rows[j].Issue })
}

func (s *Service) historyMarksix6(ctx context.Context, game Game, limit int) ([]Result, error) {
	client := s.newClient(false)
	var (
		latestRaw, histRaw []byte
		lerr, herr         error
		wg                 sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		latestRaw, lerr = s.get(ctx, client, Marksix6APIURL, url.Values{"type": {game.APICode}}, nil)
	}()
	go func() { defer wg.Done(); histRaw, herr = s.get(ctx, client, Marksix6HistoryURL, nil, nil) }()
	wg.Wait()
	err := lerr
	if err == nil {
		err = herr
	}
	if err == nil {
		var payload any
		if payload, err = DecodeJSON(latestRaw); err == nil {
			var latest Result
			if latest, err = ParseMarksix6Latest(game, payload); err == nil {
				merged := map[string]Result{}
				for _, r := range ParseMarksix6HistoryHTML(game, string(histRaw)) {
					merged[r.Issue] = r
				}
				merged[latest.Issue] = latest
				rows := make([]Result, 0, len(merged))
				for _, r := range merged {
					rows = append(rows, r)
				}
				sortByIssueDesc(rows)
				if len(rows) > limit {
					rows = rows[:limit]
				}
				return rows, nil
			}
		}
	}
	return nil, fmt.Errorf("第三方 %s 查询失败（非澳门官方）：%v", game.Name, err)
}

func (s *Service) historyMacaujc(ctx context.Context, game Game, limit int) ([]Result, error) {
	rows, err := s.historyMacaujcFrom(ctx, game, limit, MacaujcLatestURL, MacaujcHistoryURL)
	if err != nil {
		return nil, fmt.Errorf("macaujc.com 新澳六合彩查询失败：%v", err)
	}
	return rows, nil
}

// historyMacauOld reads 澳门六合彩 from macaujc.com's yearly history (used for the cache).
func (s *Service) historyMacauOld(ctx context.Context, game Game, limit int) ([]Result, error) {
	rows, err := s.historyMacaujcFrom(ctx, game, limit, MacauOldLatestURL, MacauOldHistoryURL)
	if err != nil {
		return nil, fmt.Errorf("macaujc.com 澳门六合彩历史查询失败：%v", err)
	}
	return rows, nil
}

// historyMacaujcFrom merges latest + this year + last year; succeeds when any page parses.
func (s *Service) historyMacaujcFrom(ctx context.Context, game Game, limit int, latestURL, historyFmt string) ([]Result, error) {
	now := time.Now().UTC().Add(8 * time.Hour)
	urls := []string{latestURL, fmt.Sprintf(historyFmt, now.Year()), fmt.Sprintf(historyFmt, now.Year()-1)}
	client := s.newClient(false)
	bodies := make([][]byte, len(urls))
	errs := make([]error, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) { defer wg.Done(); bodies[i], errs[i] = s.get(ctx, client, u, nil, nil) }(i, u)
	}
	wg.Wait()
	merged := map[string]Result{}
	var firstErr error
	for i, b := range bodies {
		if errs[i] != nil {
			if firstErr == nil {
				firstErr = errs[i]
			}
			continue
		}
		payload, perr := DecodeJSON(b)
		if perr != nil {
			if firstErr == nil {
				firstErr = perr
			}
			continue
		}
		for _, r := range ParseMacaujcPayload(game, payload) {
			merged[r.Issue] = r
		}
	}
	if len(merged) == 0 {
		if firstErr == nil {
			firstErr = errors.New("接口返回缺少开奖记录")
		}
		return nil, firstErr
	}
	rows := make([]Result, 0, len(merged))
	for _, r := range merged {
		rows = append(rows, r)
	}
	sortByIssueDesc(rows)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

// ---- history cache (最近100期) ----

type historySource struct {
	name string
	fn   func(ctx context.Context) ([]Result, error)
}

func (s *Service) cacheSources(game Game, limit int) []historySource {
	huiniao := historySource{"慧鸟", func(ctx context.Context) ([]Result, error) { return s.historyHuiniao(ctx, game, limit) }}
	realtime := historySource{"api16868", func(ctx context.Context) ([]Result, error) { return s.historyRealtime(ctx, game) }}
	marksix6 := historySource{"Marksix6", func(ctx context.Context) ([]Result, error) { return s.historyMarksix6(ctx, game, limit) }}
	switch game.Source {
	case "cwl":
		return []historySource{{"中国福彩网", func(ctx context.Context) ([]Result, error) { return s.historyCWL(ctx, game, limit) }}, huiniao, realtime}
	case "sport":
		return []historySource{{"中国体彩网", func(ctx context.Context) ([]Result, error) { return s.historySport(ctx, game, limit) }}, huiniao, realtime}
	case "hkjc":
		return []historySource{{"香港赛马会", func(ctx context.Context) ([]Result, error) { return s.historyHKJC(ctx, game, limit) }}, marksix6}
	case "macaujc":
		return []historySource{{"macaujc.com", func(ctx context.Context) ([]Result, error) { return s.historyMacaujc(ctx, game, limit) }}, marksix6}
	}
	// marksix6 (澳门六合彩): macaujc.com yearly list first (Marksix6 only exposes ~10 issues).
	return []historySource{{"macaujc.com", func(ctx context.Context) ([]Result, error) { return s.historyMacauOld(ctx, game, limit) }}, marksix6}
}

// CacheHistory returns up to limit issues (newest first) for the history cache,
// merging list-capable sources in priority order until limit is reached.
// Earlier (more official) sources win when two sources report the same issue.
func (s *Service) CacheHistory(ctx context.Context, code string, limit int) ([]Result, error) {
	game, ok := Games[code]
	if !ok {
		return nil, errors.New("未知彩种")
	}
	if limit < 1 || limit > HistoryCacheSize {
		limit = HistoryCacheSize
	}
	return mergeHistorySources(ctx, game, limit, s.cacheSources(game, limit), s.cacheSourceTimeout())
}

func (s *Service) cacheSourceTimeout() time.Duration {
	if s.CacheSourceTimeout > 0 {
		return s.CacheSourceTimeout
	}
	return 20 * time.Second
}

func mergeHistorySources(ctx context.Context, game Game, limit int, sources []historySource, perSource time.Duration) ([]Result, error) {
	merged := map[string]Result{}
	var errs []string
	for _, src := range sources {
		if len(merged) >= limit || ctx.Err() != nil {
			break
		}
		sctx, cancel := context.WithTimeout(ctx, perSource)
		rows, err := src.fn(sctx)
		cancel()
		if err != nil {
			errs = append(errs, src.name+"："+err.Error())
			continue
		}
		for _, r := range rows {
			if r.Issue == "" {
				continue
			}
			if _, dup := merged[r.Issue]; !dup {
				merged[r.Issue] = r
			}
		}
	}
	if len(merged) == 0 {
		if len(errs) == 0 {
			errs = []string{"无可用数据源"}
		}
		return nil, fmt.Errorf("%s历史数据源均失败：%s", game.Name, strings.Join(errs, "；"))
	}
	rows := make([]Result, 0, len(merged))
	for _, r := range merged {
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool { return CompareIssues(rows[i].Issue, rows[j].Issue) > 0 })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (s *Service) historyHuiniao(ctx context.Context, game Game, limit int) ([]Result, error) {
	code, ok := huiniaoCodes[game.Code]
	if !ok {
		return nil, fmt.Errorf("慧鸟接口暂不支持%s", game.Name)
	}
	if limit > HuiniaoHistoryLimit {
		limit = HuiniaoHistoryLimit
	}
	payload, err := s.getJSON(ctx, s.newClient(false), HuiniaoURL, url.Values{"type": {code}, "page": {"1"}, "limit": {strconv.Itoa(limit)}}, nil)
	if err != nil {
		return nil, err
	}
	rows := ParseHuiniaoHistory(game, payload)
	if len(rows) == 0 {
		return nil, errors.New("接口返回缺少开奖记录")
	}
	return rows, nil
}

func (s *Service) historyRealtime(ctx context.Context, game Game) ([]Result, error) {
	rc, ok := realtimeCodes[game.Code]
	if !ok {
		return nil, fmt.Errorf("实时接口暂不支持%s", game.Name)
	}
	u := s.Sources.RealtimeURL + "/" + RealtimeHistoryPath
	payload, err := s.getJSON(ctx, s.newClient(false), u, url.Values{"lotCode": {rc[0]}}, nil)
	if err != nil {
		return nil, err
	}
	rows, err := ParseRealtimeHistory(game, payload, u)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("接口返回缺少开奖记录")
	}
	return rows, nil
}
