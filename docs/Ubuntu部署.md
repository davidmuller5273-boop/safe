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

`/开启6码` 会自动开推送并关闭 7码；`/开启7码` 同理（只推一种）。仅 `/push on` 且两边标志都为 false 时不会推送任何码，需显式开启 6 或 7。

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

启动时 `database.Open` 会自动补 `enable_6_code` / `enable_7_code` 列，并把 `hot_number_predictions` 唯一索引升级为 `(lottery_type_id, issue_number, code_size)`。

### 数据库自检

```sql
SELECT chat_id, push_enabled, enable_6_code, enable_7_code FROM bot_group_settings;
SHOW INDEX FROM hot_number_predictions;
-- 期望存在 UNIQUE：idx_hot_predictions_lottery_issue_size (lottery_type_id, issue_number, code_size)
-- 不应再有仅 (lottery_type_id, issue_number) 的旧唯一索引 idx_hot_predictions_lottery_issue
```

若迁移早已跑过但索引仍不对，可手工执行（注意先备份）：

```sql
ALTER TABLE hot_number_predictions ADD COLUMN IF NOT EXISTS code_size INT NOT NULL DEFAULT 7;
UPDATE hot_number_predictions SET code_size = 7 WHERE code_size = 0 OR code_size IS NULL;
-- 有旧唯一索引时再 DROP：
-- ALTER TABLE hot_number_predictions DROP INDEX idx_hot_predictions_lottery_issue;
ALTER TABLE hot_number_predictions
  ADD UNIQUE KEY idx_hot_predictions_lottery_issue_size (lottery_type_id, issue_number, code_size);
ALTER TABLE bot_group_settings
  ADD COLUMN IF NOT EXISTS enable_6_code TINYINT(1) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS enable_7_code TINYINT(1) NOT NULL DEFAULT 0;
```

### 机器人命令（只要 6码）

```text
/开启6码
```

一条命令即可：自动 `push=on`、`enable_6=true`、`enable_7=false`。回复里会带当前状态。

### 验证

下一期推送标题应为 `{彩种名} 6码热号预测`，预测列应为 **6 位**热号（如 `234579`），而不是 7 位。
若仍看到 `7码热号预测`，确认已更新到含「开启即互斥」逻辑的二进制后再发一次 `/开启6码`。
