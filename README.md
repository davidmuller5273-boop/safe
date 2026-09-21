# Safe — SafeW/SafeX 聊天机器人与管理后台

基于参考项目 `safewcp/gochat` + `chatadmin` 适配的 Ubuntu 可部署项目。模块路径：`github.com/davidmuller5273-boop/safe`。

## 功能概览

- **admin**：Gin 管理后台 API（RBAC、系统配置、彩种、开奖记录）
- **lottery-collector**：币安极速飞艇采集 → MySQL + Redis 队列
- **safew-bot**：消费开奖队列发送热号预测；长轮询处理机器人命令
- **权限分级（机器人侧）**：developer / admin / group_admin
- **消息前后广告**：`prefix_ad` + `suffix_ad`，组装后 **一次** `sendMessage`
- **chatadmin**：Vue 3 + Element Plus 管理前端

详细使用说明见 [`docs/使用教程.md`](docs/使用教程.md)（**仅文档**，不会作为机器人消息下发）。  
Ubuntu 部署见 [`docs/Ubuntu部署.md`](docs/Ubuntu部署.md)。

## 快速开始（开发）

```bash
cp config/app.example.yml config/app.yml
cp .env.example .env   # 按需 export
# 准备 MySQL 库 `safe`、Redis
make build-local
./bin/safe setup
./bin/safe admin
./bin/safe lottery-collector
./bin/safe safew-bot
```

前端：

```bash
cd chatadmin && npm install && npm run build
```

生产交叉编译：

```bash
make build   # GOOS=linux GOARCH=amd64 → bin/safe
sudo bash scripts/install-ubuntu.sh
```

## 配置

- YAML：`config/app.example.yml` → `config/app.yml`
- 环境变量覆盖：见 `.env.example`（含 `DEVELOPER_USER_IDS`）
- SafeW API 默认：`https://api.safew.bot`

## 默认管理账号

首次 `setup` / 自动迁移会创建 Web 管理员 `admin` / `admin123`（请立即修改）。  
机器人 developer 从 `DEVELOPER_USER_IDS` / `bot.developer_user_ids` 引导，不入库。
