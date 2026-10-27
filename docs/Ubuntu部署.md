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

### AI波场单双一分 / 自动最优波场单双一分（可选）

`/开启AI波场单双一分` / `/关闭AI波场单双一分`（开发者或超级管理员）按群开关；需要同机 tg-directory-bot 的 web 服务（默认 `http://127.0.0.1:8080`）且已配置 `AI_API_KEY`。可选环境变量：`AI_TRON_API_URL`、`AI_TRON_WITH_ADS=0`（不带群广告）、`AI_TRON_ENABLED=0`（整体关闭）。

自动最优：`/开启自动最优波场单双一分` / `/关闭自动最优波场单双一分`，同样按群开关，使用独立的开关列 `enable_auto_tron` 与发件箱表 `auto_tron_outbox`（启动时自动建/加列），数据来自同机 tg-directory-bot 的 `GET /api/tron-draw/auto-broadcast`（接口不存在或 `enabled=false` 时静默跳过，只记一行日志）。可选环境变量：`AUTO_TRON_ENABLED=0`（整体关闭自动最优，不影响 AI）、`AUTO_TRON_API_URL`、`AUTO_TRON_WITH_ADS`（未设置时沿用 `AI_TRON_*`）。

消息里的「近三小时 最大连中/连错」由 tg-directory-bot 返回（`window.max_win` / `window.max_loss`），safe 不自行计算；对方还没升级时这一行不显示。数据契约与降级见 `docs/AI波场单双一分.md`。

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

## 自动加密备份：safe 数据一并备份（备份钩子）

tg-directory-bot 的备份脚本（每 2 天一次，也可 `sudo tgbot-backup` 手动触发，加密后推送到私有 GitHub 仓库）会以 root 依次执行 `/etc/tg-bot-backup.d/*.sh`，并把暂存目录作为 `$1` 传入。safe 提供了对应钩子：

```bash
cd ~/safe && git pull origin main && sudo bash deploy/ubuntu/install-backup-hook.sh        # 安装/更新（可重复执行）
sudo bash deploy/ubuntu/install-backup-hook.sh --test                                      # 可选：试跑一次，只打印清单，随后删除
```

钩子（`deploy/ubuntu/safe-backup-hook.sh` → `/etc/tg-bot-backup.d/safe.sh`，root、755）写入 `$1/safe/`：

| 路径 | 内容 |
|---|---|
| `safe/safe.sql` | 应用数据库的一致性 `mysqldump`（`--single-transaction --routines --triggers --events --databases`）。**含 SafeW 机器人 Token（`system_configs`）与后台账号数据** |
| `safe/secrets/` | `/root/safex` 下的 `.env`、`.env.*`、`*.env`、`config/*.yml`（含数据库密码、JWT 密钥），以及实际生效的 `CONFIG_FILE` / `/etc/safe/safe.env`（在运行目录外的放 `secrets/_abs/` 下）。**权限保持 600，目录 700** |
| `safe/files/` | 运行目录下其它小型数据文件（相对路径保持不变） |
| `safe/MANIFEST.txt` | 本次清单：只有文件名与配置来源，不含任何密码 |

**不备份**：`bin/`（二进制，可重新编译）、`logs/` 与 `*.log`、`.git`、`node_modules`、可执行文件、符号链接、大于 5 MiB 的文件、`*.pem`/`*.key` 等密钥文件、内容里含 password/secret/token 字样的其它数据文件（`.env` 与 config 除外，它们专门放进 `secrets/`）。MySQL 之外的 Redis 队列是临时数据，不备份。

> ⚠️ `safe/secrets/` 和 `safe.sql` 含真实密码/令牌：只能以**加密后的备份包**形式离开服务器，绝不能明文进入 git。钩子本身不会把任何密钥打印到屏幕或日志；数据库密码只写入权限 600 的临时凭据文件并在用完后删除。

读取数据库连接信息的顺序与应用一致（`internal/config`）：运行中 `safe` 进程的环境变量（`/proc/<pid>/environ`）→ `/root/safex/.env`、`/root/safex/safe.env`、`/etc/safe/safe.env`（只解析 `KEY=VALUE`，不 `source`）→ `CONFIG_FILE`（默认 `/root/safex/config/app.yml`）的 `mysql:` 段 → 默认值。需要系统里有 `mysqldump`：没有时钩子会提示并以非 0 退出（备份脚本只记录，不影响其它钩子），安装命令：

```bash
sudo apt update && sudo apt install -y mysql-client     # 系统用 MariaDB 时：sudo apt install -y mariadb-client
```

### 恢复（新服务器一条命令）

把备份包解密、解压到 `/tmp/tg-restore`（方法见 tg-directory-bot 备份文档；里面应有 `safe/` 目录），然后在新服务器上、仓库根目录运行（`git clone https://github.com/davidmuller5273-boop/safe.git ~/safe` 后）：

```bash
sudo BACKUP_DECRYPTED=/tmp/tg-restore bash ~/safe/deploy/ubuntu/restore-from-backup.sh
```

也支持把目录作为第一个参数：`sudo bash deploy/ubuntu/restore-from-backup.sh /tmp/tg-restore`；安装成 `/etc/tg-bot-backup.restore.d/safe.sh` 后由备份恢复流程以 `$1`=解密目录调用（`install -m 755 deploy/ubuntu/restore-from-backup.sh /etc/tg-bot-backup.restore.d/safe.sh`）。

脚本按顺序做（每步有中文进度，可重复执行）：

1. 检查 `safe/safe.sql` 完整、`safe/secrets/` 存在；**读不到数据库配置就停止，不猜**。
2. 读取备份里的 `.env` / `config/app.yml`（与应用相同的优先级：环境变量 > yaml > 默认），得到库名/用户/密码。
3. 安装依赖：`mysql-server`（已有 MySQL/MariaDB 则跳过）、`mysql-client`、`redis-server`、`git`、`make`、`curl`，设置时区 Asia/Shanghai，安装 Go（版本取 `go.mod`，下载 dl.google.com 官方包并校验 sha256）。
4. 建库、建用户并授权（与备份配置里的密码一致，密码只经 stdin，不上命令行）；数据库在远程时不建库/用户。
5. 导入 `safe.sql`。**目标库已有数据时拒绝导入**并提示；确认覆盖请加 `FORCE=1`，只想还原配置并启动请加 `SKIP_DB=1`。
6. 把 `secrets/` 还原到 `/root/safex/…` 和 `secrets/_abs/` 对应的绝对路径（如 `/etc/safe/safe.env`），权限 600；已存在且内容不同的旧文件另存为 `*.pre-restore.<时间>`。
7. 还原 `files/`（不覆盖已有文件）。
8. 取得源码（脚本所在仓库，或 `~/safe`；不存在则 `git clone`）并 `make build`，安装到 `/root/safex/bin/safe`。
9. 停掉旧进程，按「更新二进制」的方式用 nohup 启动 `admin`、`lottery-collector`、`safew-bot`（`.env` 里的变量会导出给进程，因为应用本身不读 `.env`），日志在 `/root/safex/logs/`，最后打印进程列表。

常用变量：`FORCE=1`、`SKIP_DB=1`、`SAFE_RUN_DIR`（默认 `/root/safex`）、`SAFE_REPO_DIR`、`SAFE_REPO_URL`、`RESTORE_SKIP_PREREQS=1`（已装好依赖）、`RESTORE_NO_START=1`（只还原不启动）。

仓库如果是**私有**的，`git clone` 会失败，脚本会提示：需要一个对 `davidmuller5273-boop/safe` 有 Contents 只读权限的令牌，用 `sudo GITHUB_TOKEN=<令牌> BACKUP_DECRYPTED=/tmp/tg-restore bash restore-from-backup.sh`（令牌不写入 git 配置、不打印）；或者先自己把源码放到 `~/safe`。

注意：

- 解密目录里是**明文密码**，恢复完成并核对 `safe/MANIFEST.txt` 后请立刻 `rm -rf /tmp/tg-restore`。
- 进程用 nohup 启动，**服务器重启后不会自动拉起**，需要再运行一次本命令（或自行改用 systemd，见 `scripts/install-ubuntu.sh`）。
- 脚本不恢复 nginx/域名/证书等其它组件；bot 的 Redis 队列数据也不在备份里（队列会重新生成）。

#### 手动恢复（脚本不可用时）

先停掉 safe 的三个进程（见上文「更新二进制」），假设已解密到 `/root/restore`：

```bash
# 1) 恢复数据库（dump 自带 CREATE DATABASE IF NOT EXISTS 和 USE；会覆盖同名表）
sudo mysql < /root/restore/safe/safe.sql

# 2) 恢复配置与密钥（保持 600；.env、config/app.yml 回到 /root/safex）
sudo bash -c 'cd /root/restore/safe/secrets && for f in $(find . -type f ! -path "./_abs/*"); do install -D -m 600 "$f" "/root/safex/$f"; done'

# 3) （如有）运行目录之外的配置，例如 /etc/safe/safe.env，按原绝对路径还原
[ -d /root/restore/safe/secrets/_abs ] && sudo cp -a --no-preserve=ownership /root/restore/safe/secrets/_abs/. / && sudo chmod 600 /etc/safe/safe.env 2>/dev/null; true

# 4) 恢复其它数据文件（如有）
[ -d /root/restore/safe/files ] && sudo cp -a /root/restore/safe/files/. /root/safex/; true

# 5) 用 MANIFEST 核对，然后启动三个进程；最后清掉解密出来的明文
cat /root/restore/safe/MANIFEST.txt
sudo rm -rf /root/restore
```

数据库用户不存在时（全新机器），先按上文「3. 配置」创建库与用户，再执行第 1 步：库名/用户/密码以 `secrets/.env` 或 `secrets/config/app.yml` 里的为准。`bin/safe` 请用 `make build` 重新编译，不在备份里。

