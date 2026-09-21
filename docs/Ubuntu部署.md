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
