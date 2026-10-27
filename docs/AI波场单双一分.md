# 波场单双一分按群播报：AI预测 + 自动最优

两个播报共用同一套代码、模板、发送队列，各自独立开关、独立发件箱表、独立去重：

| | AI波场单双一分 | 自动最优波场单双一分 |
|---|---|---|
| 开启/关闭 | `/开启AI波场单双一分` / `/关闭AI波场单双一分` | `/开启自动最优波场单双一分` / `/关闭自动最优波场单双一分` |
| 群开关列 | `bot_group_settings.enable_ai_tron` | `bot_group_settings.enable_auto_tron` |
| 发件箱表 | `ai_tron_outbox` | `auto_tron_outbox` |
| 数据接口 | `GET /api/tron-draw/ai-broadcast`（404 时降级到 A） | `GET /api/tron-draw/auto-broadcast`（404 / `enabled=false` 静默跳过，无降级） |
| 小标题 | `AI预测（近15期）` | `自动最优预测（近15期）` |
| 整体关闭 | `AI_TRON_ENABLED=0` | `AUTO_TRON_ENABLED=0` |

下文以 AI 为例；自动最优除上表差异外完全相同。

## AI波场单双一分（按群播报）

`/开启AI波场单双一分` / `/关闭AI波场单双一分`（也接受 `/开启 AI波场单双一分`）。权限同 `/开启6码`：开发者或超级管理员。
开关存于 `bot_group_settings.enable_ai_tron`（启动时自动加列），**独立于 `/push` 和冠军/亚军 6/7 码**。

开启后，safew-bot 每 5 秒向 tg-directory-bot 取一次数据；每出现一个新的待开奖区块（约每分钟）就给每个已开启的群发一条消息：

```
波场一分单双

AI预测（近15期）
区块号 | 结果 | 预测 | 状态        ← 等宽 <pre> 表格，旧→新，最后一行是待开奖
...
────────────────
本期预测：🔴双
📊 周期胜率概览
180期：49.1%（已预测 8966 期 49.1%）
近三小时 最大连中 5✅ 连错 4❌
走势预测统计网址  188.ms
```

* 「近三小时 最大连中 X✅ 连错 Y❌」**由 tg-directory-bot 计算并返回**（`window.max_win` / `window.max_loss`：最近 180 个已开奖区块里 AI 连续命中/连续未中的最大期数），safe **不自行计算**；接口没返回这两个字段（老版本）时这一行**整行不显示**（降级 A 同理）。胜率两行照旧用 `window` / `overall`。
* 🟢单 / 🔴双；状态 ✅ 预测对 / ❌ 预测错 / ⏳ 待开奖。没有留底 AI 预测的期（AI 启用之前）不显示，也不计入命中率。
* 去重：表 `ai_tron_outbox` 对 `(chat_id, block)` 唯一，同一期同一群只发一次（重启后也不重发）。
* 重试 / 永久错误：与开奖播报相同——临时错误最多 3 次；群不可达（被踢、频道私有、403 等）立即标记失败并**自动关闭该群的开关**。过期（>3 分钟或已有更新一期）的待发消息直接作废，不会补发旧预测。
* 限速：每条之间间隔 50 ms 串行发送；每群每分钟 1 条；静默发送（不置顶、不提醒）。
* 数据不可用（tg-directory-bot 没开 AI / 宕机 / 数据过期 10 分钟）：静默跳过，同一原因只记一行日志，恢复后记一行；不影响其他队列（独立 goroutine）。
* 环境变量：`AI_TRON_API_URL`（默认 `http://127.0.0.1:8080`）、`AI_TRON_WITH_ADS`（默认 1，带**本群自己的**广告，没设置广告的群不带；0 一律不带）、`AI_TRON_ENABLED=0` 整体关闭 AI 播报。自动最优：`AUTO_TRON_ENABLED=0` 整体关闭（与 AI 互不影响）；`AUTO_TRON_API_URL`、`AUTO_TRON_WITH_ADS` 未设置时沿用 `AI_TRON_API_URL`、`AI_TRON_WITH_ADS`。

## 数据契约

### A. 现有接口（今天即可工作，作为降级）

`GET {base}/api/tron-draw?limit=1&ai_limit=1`（每次轮询，轻量探测）→ 取 `ai.next.{block,pick}`；`ai` 为 `null` = AI 未启用；`ai.next` 为 `null` = 本期预测尚未生成（静默等待）。
`GET {base}/api/tron-draw?limit=180&ai_limit=5000`（每期仅一次）→ 使用：

| 消息内容 | 字段 |
|---|---|
| 近15期 区块号/结果/预测/状态 | `ai.trend.items[]` 的 `block`、`result`、`predicted`（`null` 的跳过），取最后 15 条，`predicted==result` 为 ✅ |
| 本期（待开奖）区块号与预测 | `ai.next.block`、`ai.next.pick` |
| 180期命中率 | 对 `ai.trend.items` 最后 180 项统计（有预测的项） |
| 已预测期数与总命中率 | 对返回的全部 `ai.trend.items` 统计 |
| 近三小时 最大连中/连错 | **只在 `ai.window.max_win` / `ai.window.max_loss` 存在时显示**（数字，或 `{"len":N,…}` 对象取 `len`）；现有接口没有这两个字段，所以降级 A 目前不显示这一行。**不会**用 `ai.trend.max_win`（那是整个 `ai_limit`=5000 期的口径，不是 180 期）。 |

**缺口**：`ai_limit` 最大 5000（`parse_limit(..., maximum=5000)`），没有「全部历史」的 AI 总数/总命中率字段。当 AI 留底超过 5000 期（如 8966）时只能统计最近 5000 期，消息会写成 `（近5000期 xx.x%）` 而不是 `已预测 8966 期`。另外每期要拉一次 5000 条的 JSON（约几百 KB，本机回环，可接受）。

### B. 专用接口（AI：`/api/tron-draw/ai-broadcast`；自动最优：`/api/tron-draw/auto-broadcast`，JSON 形状完全相同）

`GET /api/tron-draw/ai-broadcast` 与 `GET /api/tron-draw/auto-broadcast`　无参数，`Cache-Control: no-store`，仅本机访问。safe 端优先使用。

```json
{
  "enabled": true,
  "reason": "",
  "latest_block": 87473340,
  "latest_ts": 1761558000000,
  "next_block": 87473360,
  "next_pick": "双",
  "recent": [
    {"block": 87473160, "result": "单", "pick": "单"},
    "... 最近 15 期「有留底预测」的已开奖期，从旧到新，result/pick 只能是 单 或 双 ..."
  ],
  "window": {"periods": 180, "hits": 88, "total": 179, "max_win": 6, "max_loss": 5},
  "overall": {"hits": 4402, "total": 8966}
}
```

* `enabled=false` → HTTP 200，`{"enabled": false, "reason": "AI_API_KEY 未配置"}`（自动最优同理，例如未启用/没有数据）。safe 端静默跳过，同一原因只记一行日志。
* `next_pick` 为 `null` = 下一期预测尚未生成（safe 端静默等待下一轮）。
* `window`：最近 `periods`(=180) 个**已开奖区块**里有留底预测的期的 hits/total；`overall`：留底预测中已有开奖结果的全部预测的 hits/total（不设 5000 上限）。
* **`window.max_win` / `window.max_loss`（新增）**：同一个 180 期窗口内，最长连续命中 / 最长连续未中的期数（非负整数）。**由 web 程序计算，safe 不再自算**；无预测的期跳过（与 `ai_trend` 口径一致），窗口内没有预测则都为 0。缺任一字段 → safe 不显示「近三小时 最大连中/连错」这一行。
* 旧字段 `streak`（当前连续）已不再使用，可保留也可删除，safe 忽略它。
* `latest_ts`：最新已记录开奖区块的时间戳（毫秒），safe 端用于判断数据是否过期（>10 分钟则跳过）；0 表示不检查。
* 计算只可使用开奖前已留底的预测（写一次规则），AI 按 `config.ai_model` 过滤；**自动最优**用开奖前留底的「自动最优/最终预测」（页面算法走势里的那一条），口径与页面一致。
* `recent` 建议返回 15 条；若 AI 接口返回少于 15 条，safe 会用 A 的数据补足表格（统计数字仍用本接口）。自动最优不补足、不降级。

safe 端：HTTP 200 → 使用；AI 接口 HTTP 404 → 自动降级到 A（自动最优接口 404 → 静默跳过并记一行日志）；其他错误/连不上 → 视为数据暂不可用。
