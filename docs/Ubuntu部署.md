# Ubuntu 部署指南

适用 Ubuntu 22.04 / 24.04 LTS。时区建议 `Asia/Shanghai`。

## 1. 编译

在任意有 Go 1.22+ 的机器上：

```bash
make build   # 输出 bin/safe （linux/amd64）
```

前端（可选）：

```bash
cd chatadmin && npm install && npm run build
# 将 dist/ 内容拷到服务器 /opt/safe/web/
```

## 2. 安装

```bash
sudo bash scripts/install-ubuntu.sh
```

脚本会：

- 安装 nginx / mysql / redis（若尚未安装）
- 创建系统用户 `safe` 与目录 `/opt/safe`、`/etc/safe`
- 安装二进制与示例配置、systemd unit

## 3. 配置

编辑：

- `/opt/safe/config/app.yml`
- `/etc/safe/safe.env`（至少设置 `JWT_SECRET`、`MYSQL_PASSWORD`、`DEVELOPER_USER_IDS`）

创建数据库：

```sql
CREATE DATABASE `safe` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'safe'@'127.0.0.1' IDENTIFIED BY '替换为高强度密码';
GRANT ALL PRIVILEGES ON `safe`.* TO 'safe'@'127.0.0.1';
FLUSH PRIVILEGES;
```

初始化：

```bash
sudo -u safe bash -c 'set -a; source /etc/safe/safe.env; set +a; /opt/safe/bin/safe setup'
```

## 4. 启动

```bash
sudo systemctl enable --now safe-admin safe-lottery-collector safe-safew-bot
sudo systemctl status safe-admin safe-lottery-collector safe-safew-bot
```

Nginx 参考 `deploy/nginx-safe.conf.example`。**不要**把 3306/6379/8081 暴露到公网。

## 5. 验证

- `curl -s http://127.0.0.1:8081/health`
- 管理后台登录后配置 SafeW Token 与群 ID
- 私聊机器人 `/whoami` 确认 developer 生效

## 6. 6码 / 7码热号推送（运维核对）

`/开启6码` 会自动开推送并关闭冠军 7码；`/开启7码` 同理（冠军只推一种）。`/开启亚军6码` / `/开启亚军7码` 同理作用于亚军，且**不影响**冠军标志。仅 `/push on` 且冠军/亚军 6/7 标志全为 false 时不会推送任何码。

### 更新二进制（示例：源码在 `~/safe`，运行目录 `/root/safex`）

```bash
cd ~/safe
git pull origin main
make build
cp -f bin/safe /root/safex/safe
# 按实际进程名停止后重启（systemd 或手动均可）
pkill -f 'safe admin' || true
pkill -f 'safe lottery-collector' || true
pkill -f 'safe safew-bot' || true
cd /root/safex
nohup ./safe admin > admin.log 2>&1 &
nohup ./safe lottery-collector > lottery-collector.log 2>&1 &
nohup ./safe safew-bot > safew-bot.log 2>&1 &
# 若使用 systemd：
# sudo systemctl restart safe-admin safe-lottery-collector safe-safew-bot
```

启动时 `database.Open` 会自动补 `enable_6_code` / `enable_7_code` / `enable_runner_up_6_code` / `enable_runner_up_7_code` 列，以及 `hot_number_predictions.position`（默认 1=冠军），并把唯一索引升级为 `(lottery_type_id, issue_number, code_size, position)`。

### 数据库自检

```sql
SELECT chat_id, push_enabled, enable_6_code, enable_7_code, enable_runner_up_6_code, enable_runner_up_7_code FROM bot_group_settings;
SHOW INDEX FROM hot_number_predictions;
-- 期望存在 UNIQUE：idx_hot_predictions_lottery_issue_size_pos (lottery_type_id, issue_number, code_size, position)
-- 不应再有省略 position 的旧唯一索引 idx_hot_predictions_lottery_issue_size / idx_hot_predictions_lottery_issue
```

若迁移早已跑过但索引仍不对，可手工执行（注意先备份）：

```sql
-- 优先依赖启动时自动迁移；仅在异常时手工执行（注意先备份）：
-- 见 migrations/009_runner_up.sql
ALTER TABLE hot_number_predictions ADD COLUMN IF NOT EXISTS code_size INT NOT NULL DEFAULT 7;
ALTER TABLE hot_number_predictions ADD COLUMN IF NOT EXISTS position INT NOT NULL DEFAULT 1;
UPDATE hot_number_predictions SET code_size = 7 WHERE code_size = 0 OR code_size IS NULL;
UPDATE hot_number_predictions SET position = 1 WHERE position = 0 OR position IS NULL;
-- 有旧唯一索引时再 DROP：
-- ALTER TABLE hot_number_predictions DROP INDEX idx_hot_predictions_lottery_issue;
-- ALTER TABLE hot_number_predictions DROP INDEX idx_hot_predictions_lottery_issue_size;
ALTER TABLE hot_number_predictions
  ADD UNIQUE KEY idx_hot_predictions_lottery_issue_size_pos (lottery_type_id, issue_number, code_size, position);
ALTER TABLE bot_group_settings
  ADD COLUMN IF NOT EXISTS enable_6_code TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS enable_7_code TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS enable_runner_up_6_code TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS enable_runner_up_7_code TINYINT(1) NOT NULL DEFAULT 0;
```

### 机器人命令（只要冠军 6码 / 只要亚军 7码）

```text
/开启6码
/开启亚军7码
```

`/开启6码`：自动 `push=on`、冠军 `enable_6=true`、`enable_7=false`（亚军标志不变）。
`/开启亚军7码`：自动 `push=on`、亚军 `enable_runner_up_7=true`、`enable_runner_up_6=false`（冠军标志不变）。
回复里会带当前状态。

### 验证

下一期冠军推送标题应为 `{彩种名} 6码热号预测`；亚军推送标题应为 `{彩种名} 亚军7码热号预测`。
若开启亚军后仍无亚军消息，确认已更新到含亚军逻辑的二进制，且群 `push=on` 且对应 `enable_runner_up_*` 为 true。

## 7. 开奖播报 / 按钮菜单 / 自动置顶（升级说明）

**升级只需标准流程，无需手工 SQL**：`git pull` → `make build` → 复制二进制 → 重启 `admin`、`lottery-collector`、`safew-bot` 三个进程（不新增进程）。

- 启动时自动 `CREATE TABLE IF NOT EXISTS`：`lottery_broadcast_results`、`lottery_broadcast_subscriptions`、`lottery_broadcast_source_status`、`lottery_broadcast_outbox`（不走 AutoMigrate，不会重复建索引；建表失败只记日志，不影响其他功能）。
- 开关存于 `system_configs`：`lottery_query_enabled`、`lottery_broadcast_enabled`、`lottery_broadcast_with_ads`、`lottery_broadcast_games`，**缺省即全部开启**。
- 轮询与发送在 `safew-bot` 进程内（它持有机器人 Token）：每 5 秒轮询，新一期出现且开关为开时入队，3 秒内发送并 `pinChatMessage` 置顶。
- **自动置顶要求机器人是群管理员并有「置顶消息」权限**；无权限时仍发送，只是不置顶（`safew-bot` 日志与后台「最近播报」可见）。
- 服务器需能访问外网数据源（福彩 cwl.gov.cn、体彩 sporttery.cn、api.api16868.com、raw.githubusercontent.com、香港赛马会、marksix6 / macaujc 等）。可选环境变量覆盖（一般不用设）：

```bash
# CWL_LOTTERY_URL=...
# SPORTTERY_LOTTERY_URL=...
# LOTTERY_REALTIME_URL=https://api.api16868.com
# LOTTERY_PUBLIC_DATA_BASE_URL=https://raw.githubusercontent.com/wenjinliuu/lottery-data-repo/main/public_data
```

- 后台页面：不重建前端即可访问 `https://你的域名/admin/lottery-broadcast/ui`（经 nginx `/admin/` 代理到 8081，后台账号登录，需 `lottery:manage` 权限）。重建前端（`cd chatadmin && npm install && npm run build`）后，「彩票管理」菜单下也会出现「开奖播报」。

### 自检

```sql
SHOW TABLES LIKE 'lottery_broadcast_%';
SELECT chat_id, selector FROM lottery_broadcast_subscriptions;
SELECT chat_id, game_code, issue, status, pinned, last_error FROM lottery_broadcast_outbox ORDER BY id DESC LIMIT 20;
```
