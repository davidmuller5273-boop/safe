#!/usr/bin/env bash
# 新服务器一键恢复 SafeW 机器人（safe 项目）。以 root 运行，可重复执行。
#
#   sudo BACKUP_DECRYPTED=/tmp/tg-restore bash deploy/ubuntu/restore-from-backup.sh
#   或：  restore-from-backup.sh /tmp/tg-restore        （也可安装为 /etc/tg-bot-backup.restore.d/safe.sh，$1 = 解密目录）
#
# 解密目录里应有 safe/（由 deploy/ubuntu/safe-backup-hook.sh 生成）：
#   safe/safe.sql  safe/secrets/（.env、config/*.yml，权限 600；secrets/_abs/ = 位于别处的配置）  safe/files/  safe/MANIFEST.txt
#
# 步骤：①检查备份 ②读取配置 ③安装依赖(MySQL/Redis/Go…) ④建库建用户 ⑤导入 safe.sql ⑥还原配置到原位置(600)
#       ⑦还原数据文件 ⑧取得源码并 make build ⑨安装二进制 ⑩启动 admin / lottery-collector / safew-bot 并列出进程
#
# 环境变量：
#   BACKUP_DECRYPTED   解密目录（或用第一个参数）
#   FORCE=1            目标库已有数据时仍然导入（默认拒绝，避免覆盖）
#   SKIP_DB=1          不动数据库（只还原配置/二进制并启动）
#   SAFE_RUN_DIR       运行目录，默认 /root/safex
#   SAFE_REPO_DIR      源码目录，默认脚本所在仓库；否则 ~/safe（不存在则 git clone）
#   SAFE_REPO_URL      默认 https://github.com/davidmuller5273-boop/safe.git
#   GITHUB_TOKEN       仓库为私有时用于 clone/pull 的只读令牌（不会写入 remote、不会打印）
#   RESTORE_SKIP_PREREQS=1  跳过 apt/Go 安装（已装好时）；RESTORE_NO_START=1 只还原不启动
# 绝不打印任何密码/令牌。
set -uo pipefail

TAG="[safe-restore]"
step() { printf '\n%s ▶ %s\n' "$TAG" "$*"; }
log()  { printf '%s %s\n' "$TAG" "$*"; }
warn() { printf '%s ⚠ %s\n' "$TAG" "$*" >&2; }
die()  { printf '%s ✖ %s\n' "$TAG" "$*" >&2; exit "${2:-1}"; }

RUN_DIR="${SAFE_RUN_DIR:-/root/safex}"
ROOT_PREFIX="${RESTORE_ROOT_PREFIX:-}"       # 仅测试用：_abs 文件还原到 $ROOT_PREFIX/<绝对路径>
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
umask 077

if [ "$(id -u)" -ne 0 ] && [ "${SAFE_RESTORE_ALLOW_NONROOT:-}" != "1" ]; then
  die "请用 root 运行：sudo BACKUP_DECRYPTED=/tmp/tg-restore bash deploy/ubuntu/restore-from-backup.sh"
fi

DEFS=""
cleanup() { [ -n "$DEFS" ] && rm -f -- "$DEFS"; [ -n "${TMPW:-}" ] && rm -rf -- "$TMPW"; }
trap cleanup EXIT
trap 'exit 130' INT TERM
TMPW="$(mktemp -d "${TMPDIR:-/tmp}/safe-restore.XXXXXX")"

# ───────────── ① 检查备份 ─────────────
step "① 检查备份目录"
B="${1:-${BACKUP_DECRYPTED:-}}"
[ -n "$B" ] || die "未指定备份目录。用法：sudo BACKUP_DECRYPTED=/tmp/tg-restore bash deploy/ubuntu/restore-from-backup.sh"
[ -d "$B" ] || die "备份目录不存在: $B"
if [ -f "$B/safe/safe.sql" ]; then S="$B/safe"
elif [ -f "$B/safe.sql" ] && [ -d "$B/secrets" ]; then S="$B"; log "传入的已是 safe/ 目录本身，直接使用"
else die "在 $B 里找不到 safe/safe.sql。请确认已解密，且目录结构是 <解密目录>/safe/{safe.sql,secrets,files,MANIFEST.txt}"; fi
[ -d "$S/secrets" ] || die "备份里没有 $S/secrets/（.env / config）。无法知道数据库名、用户和密码——我不会猜，已停止。请确认备份包完整。"
tail -n 5 "$S/safe.sql" | grep -q -- '-- Dump completed' || die "$S/safe.sql 不完整（缺少 '-- Dump completed' 结尾），已停止。"
[ -f "$S/MANIFEST.txt" ] && log "备份清单: $(head -n 1 "$S/MANIFEST.txt")"
log "备份目录: $S"

# ───────────── ② 读取配置（与应用 internal/config 一致：环境变量 > yaml > 默认）─────────────
step "② 读取备份里的配置（内容不会显示）"
# secrets/<rel> ↔ $RUN_DIR/<rel>； secrets/_abs/<abs> ↔ /<abs>
ENV_CANDIDATES=("$S/secrets/.env" "$S/secrets/safe.env" "$S/secrets/_abs/etc/safe/safe.env")

dotenv_get() { # KEY FILE
  local key="$1" file="$2" line v val="" found=1
  [ -f "$file" ] && [ -r "$file" ] || return 1
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line%$'\r'}"
    if [[ "$line" =~ ^[[:space:]]*(export[[:space:]]+)?${key}[[:space:]]*=(.*)$ ]]; then
      v="${BASH_REMATCH[2]}"; v="${v#"${v%%[![:space:]]*}"}"
      if [[ "$v" == \"* ]]; then v="${v#\"}"; v="${v%\"*}"
      elif [[ "$v" == \'* ]]; then v="${v#\'}"; v="${v%\'*}"
      else v="${v%%[[:space:]]#*}"; v="${v%"${v##*[![:space:]]}"}"; fi
      if [ -n "$v" ]; then val="$v"; found=0; fi
    fi
  done <"$file"
  [ "$found" -eq 0 ] && printf '%s' "$val"
  return "$found"
}

read -r -d '' AWK_YAML <<'AWK' || true
function trim(s) { gsub(/^[ \t]+|[ \t]+$/, "", s); return s }
/^[^ \t#]/ { insec = ($0 ~ ("^" sect "[ \t]*:[ \t]*(#.*)?$")); next }
insec && /^[ \t]+[A-Za-z_]+[ \t]*:/ {
  line = $0; sub(/^[ \t]+/, "", line)
  k = line; sub(/[ \t]*:.*/, "", k)
  if (k != want) next
  v = line; sub(/^[^:]*:[ \t]*/, "", v)
  c0 = substr(v, 1, 1)
  if (c0 == "\"") {
    rest = substr(v, 2); out = ""
    while (length(rest) > 0) {
      c = substr(rest, 1, 1)
      if (c == "\\") { out = out substr(rest, 2, 1); rest = substr(rest, 3); continue }
      if (c == "\"") break
      out = out c; rest = substr(rest, 2)
    }
    val = out
  } else if (c0 == "'") {
    rest = substr(v, 2); out = ""
    while (length(rest) > 0) {
      c = substr(rest, 1, 1)
      if (c == "'") { if (substr(rest, 2, 1) == "'") { out = out "'"; rest = substr(rest, 3); continue } break }
      out = out c; rest = substr(rest, 2)
    }
    val = out
  } else { sub(/[ \t]+#.*$/, "", v); val = trim(v) }
  found = 1
}
END { if (found) printf "%s", val }
AWK
yaml_get() { [ -f "$1" ] && [ -r "$1" ] || return 1; local v; v=$(awk -v sect="$2" -v want="$3" "$AWK_YAML" "$1" 2>/dev/null) || return 1; [ -n "$v" ] || return 1; printf '%s' "$v"; }

CFG_VAL=""; CFG_FOUND=0
cfg_get() { # ENVKEY SECTION YAMLKEY DEFAULT   →  CFG_VAL；CFG_FOUND=1 表示备份里确实有这个值
  local key="$1" sect="$2" ykey="$3" def="$4" v f
  for f in "${ENV_CANDIDATES[@]}"; do
    if v=$(dotenv_get "$key" "$f") && [ -n "$v" ]; then CFG_VAL="$v"; CFG_FOUND=1; return 0; fi
  done
  if [ -n "$sect" ] && [ -n "${CONFIG_IN_BACKUP:-}" ] && v=$(yaml_get "$CONFIG_IN_BACKUP" "$sect" "$ykey") && [ -n "$v" ]; then
    CFG_VAL="$v"; CFG_FOUND=1; return 0
  fi
  CFG_VAL="$def"; CFG_FOUND=0
}

# CONFIG_FILE：相对路径按运行目录解析；备份里对应的文件：运行目录内 → secrets/<rel>；其它 → secrets/_abs/<abs>
cfg_get CONFIG_FILE "" "" "config/app.yml"; cf="$CFG_VAL"
case "$cf" in
  /*) CONFIG_ORIG="$cf" ;;
  *)  CONFIG_ORIG="$RUN_DIR/$cf" ;;
esac
case "$CONFIG_ORIG" in
  "$RUN_DIR"/*) CONFIG_IN_BACKUP="$S/secrets/${CONFIG_ORIG#"$RUN_DIR"/}" ;;
  *)            CONFIG_IN_BACKUP="$S/secrets/_abs/${CONFIG_ORIG#/}" ;;
esac
[ -f "$CONFIG_IN_BACKUP" ] || CONFIG_IN_BACKUP=""

HAVE_ANY=0
for f in "${ENV_CANDIDATES[@]}"; do [ -f "$f" ] && HAVE_ANY=1; done
[ -n "$CONFIG_IN_BACKUP" ] && HAVE_ANY=1
[ "$HAVE_ANY" -eq 1 ] || die "备份的 secrets/ 里没有找到 .env 或 CONFIG_FILE 指向的配置（$CONFIG_ORIG）。无法确定数据库连接信息，我不会猜，已停止。请检查备份包 MANIFEST.txt。"

cfg_get MYSQL_HOST mysql host 127.0.0.1;           DB_HOST="$CFG_VAL"
cfg_get MYSQL_PORT mysql port 3306;                DB_PORT="$CFG_VAL"
cfg_get MYSQL_USER mysql user "";                  DB_USER="$CFG_VAL"
cfg_get MYSQL_PASSWORD mysql password "";          DB_PASS="$CFG_VAL"
cfg_get MYSQL_DATABASE mysql database "";          DB_NAME="$CFG_VAL"
[ -n "$DB_USER" ] && [ -n "$DB_PASS" ] && [ -n "$DB_NAME" ] || die "备份的配置里没有完整的 MySQL 用户/密码/库名（mysql.user / mysql.password / mysql.database 或 MYSQL_* 变量）。我不会用默认值去猜，已停止。"
[ "$DB_PASS" != "CHANGE_ME" ] || die "备份的配置里数据库密码仍是示例值 CHANGE_ME，说明备份没带到真实配置。已停止。"
[[ "$DB_PORT" =~ ^[0-9]+$ ]] || die "MYSQL_PORT 不是数字"
[[ "$DB_NAME" =~ ^[A-Za-z0-9_]+$ ]] || die "数据库名含不支持的字符，已停止"
[[ "$DB_USER" =~ ^[A-Za-z0-9_.-]+$ ]] || die "数据库用户名含不支持的字符，已停止"
DUMP_DB="$(grep -m1 '^USE `' "$S/safe.sql" | sed 's/^USE `\(.*\)`;$/\1/')"
if [ -n "$DUMP_DB" ] && [ "$DUMP_DB" != "$DB_NAME" ]; then
  die "safe.sql 里的库名（$DUMP_DB）与备份配置里的库名（$DB_NAME）不一致，我不会猜，已停止。"
fi
LOCAL_DB=0
case "$DB_HOST" in 127.0.0.1|localhost|::1) LOCAL_DB=1 ;; esac
log "库: $DB_NAME  用户: $DB_USER  地址: $DB_HOST:$DB_PORT（$([ $LOCAL_DB -eq 1 ] && echo 本机 || echo 远程)；密码不显示）"
[ -n "$CONFIG_IN_BACKUP" ] && log "CONFIG_FILE: $CONFIG_ORIG（备份里有）" || warn "CONFIG_FILE（$CONFIG_ORIG）不在备份里，仅使用 .env 里的值"

# ───────────── ③ 安装依赖 ─────────────
GO_NEED="$(awk '/^go [0-9]/{print $2; exit}' "$SCRIPT_DIR/../../go.mod" 2>/dev/null)"
[ -n "$GO_NEED" ] || GO_NEED="1.24.1"
ver_ge() { [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]; }  # $1 >= $2
find_go() {
  local g
  for g in "$(command -v go 2>/dev/null)" /usr/local/go/bin/go; do
    [ -n "$g" ] && [ -x "$g" ] || continue
    GO_CUR="$("$g" env GOVERSION 2>/dev/null | sed 's/^go//')"
    if [ -n "$GO_CUR" ] && ver_ge "${GO_CUR%%[a-z-]*}" "$GO_NEED"; then GO_BIN="$g"; return 0; fi
  done
  return 1
}
step "③ 安装依赖（MySQL、Redis、git、make、Go ≥ $GO_NEED）"
if [ "${RESTORE_SKIP_PREREQS:-}" = "1" ]; then
  log "RESTORE_SKIP_PREREQS=1，跳过"
else
  export DEBIAN_FRONTEND=noninteractive
  # 与 scripts/install-ubuntu.sh 相同的基础包（mysql-server、redis-server、ca-certificates、curl）；nginx 不属于恢复范围
  pkgs=(ca-certificates curl git make tar redis-server)
  if ! command -v mysqld >/dev/null 2>&1 && ! command -v mariadbd >/dev/null 2>&1; then pkgs+=(mysql-server); else log "已有 MySQL/MariaDB 服务端，不重复安装"; fi
  command -v mysqldump >/dev/null 2>&1 || pkgs+=(mysql-client)
  apt-get update -y >/dev/null || warn "apt-get update 失败，继续尝试安装"
  apt-get install -y "${pkgs[@]}" >/dev/null || die "apt 安装失败: ${pkgs[*]}（请检查网络/源后重试，脚本可重复执行）"
  timedatectl set-timezone Asia/Shanghai 2>/dev/null || true   # 与 install-ubuntu.sh 一致
  for u in redis-server mysql mariadb; do systemctl list-unit-files "$u.service" 2>/dev/null | grep -q "^$u.service" && systemctl enable --now "$u" >/dev/null 2>&1; done
  if find_go; then log "Go 已满足: $GO_CUR"
  else
    case "$(uname -m)" in x86_64) ga=amd64 ;; aarch64|arm64) ga=arm64 ;; *) die "不支持的 CPU 架构: $(uname -m)" ;; esac
    tgz="go${GO_NEED}.linux-${ga}.tar.gz"
    log "安装 Go $GO_NEED（官方包 dl.google.com，校验 sha256）…"
    curl -fsSL -o "$TMPW/$tgz" "https://dl.google.com/go/$tgz" || die "下载 Go 失败（网络？）"
    want="$(curl -fsSL "https://dl.google.com/go/$tgz.sha256" | tr -d ' \n')" || die "取不到 Go 的 sha256，已停止（不安装未校验的二进制）"
    got="$(sha256sum "$TMPW/$tgz" | awk '{print $1}')"
    [ -n "$want" ] && [ "$want" = "$got" ] || die "Go 安装包 sha256 校验失败，已停止"
    rm -rf /usr/local/go && tar -C /usr/local -xzf "$TMPW/$tgz" || die "解压 Go 失败"
    find_go || die "Go 安装后仍不可用"
    log "Go 已安装: $GO_CUR"
  fi
fi

# ───────────── ④ 数据库：建库建用户 ─────────────
CREDS_CNF="$TMPW/app.cnf"
mk_defaults() {
  local p="$DB_PASS"; p="${p//\\/\\\\}"; p="${p//\"/\\\"}"
  { printf '[client]\nuser="%s"\npassword="%s"\nhost="%s"\nport=%s\n' "$DB_USER" "$p" "$DB_HOST" "$DB_PORT"; } >"$CREDS_CNF"
  chmod 600 "$CREDS_CNF"
}
mk_defaults
# 本机库：用 root（unix socket）管理；远程库：只能用备份里的应用账号
if [ "$LOCAL_DB" -eq 1 ]; then ADMIN_MYSQL=(mysql); else ADMIN_MYSQL=(mysql "--defaults-extra-file=$CREDS_CNF"); fi
sql_escape() { local s="$1"; s="${s//\\/\\\\}"; s="${s//\'/\'\'}"; printf '%s' "$s"; }

if [ "${SKIP_DB:-}" = "1" ]; then
  step "④⑤ SKIP_DB=1：跳过建库与导入"
else
  step "④ 准备数据库 $DB_NAME 与用户 $DB_USER"
  if [ "$LOCAL_DB" -eq 1 ]; then
    if [ "${RESTORE_SKIP_PREREQS:-}" != "1" ]; then
      for i in $(seq 1 30); do "${ADMIN_MYSQL[@]}" -N -B -e 'SELECT 1' >/dev/null 2>&1 && break; sleep 1; done
    fi
    "${ADMIN_MYSQL[@]}" -N -B -e 'SELECT 1' >/dev/null 2>&1 || die "连不上本机 MySQL（root 通过 socket）。请确认 MySQL 已启动：systemctl status mysql；脚本可重复执行。"
    pe="$(sql_escape "$DB_PASS")"
    # 密码只走 stdin，不进命令行；host 同时建 127.0.0.1 与 localhost
    "${ADMIN_MYSQL[@]}" >/dev/null <<SQL || die "建库/建用户失败（权限？）"
CREATE DATABASE IF NOT EXISTS \`$DB_NAME\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '$DB_USER'@'127.0.0.1' IDENTIFIED BY '$pe';
CREATE USER IF NOT EXISTS '$DB_USER'@'localhost' IDENTIFIED BY '$pe';
ALTER USER '$DB_USER'@'127.0.0.1' IDENTIFIED BY '$pe';
ALTER USER '$DB_USER'@'localhost' IDENTIFIED BY '$pe';
GRANT ALL PRIVILEGES ON \`$DB_NAME\`.* TO '$DB_USER'@'127.0.0.1';
GRANT ALL PRIVILEGES ON \`$DB_NAME\`.* TO '$DB_USER'@'localhost';
FLUSH PRIVILEGES;
SQL
    log "库与用户已就绪（用户密码与备份配置一致）"
  else
    log "数据库在远程（$DB_HOST），不创建库/用户，直接使用备份配置里的账号"
  fi

  step "⑤ 导入 safe.sql"
  tables="$("${ADMIN_MYSQL[@]}" -N -B -e "SELECT table_name FROM information_schema.tables WHERE table_schema='$DB_NAME'" 2>/dev/null)" || die "读取 $DB_NAME 的表列表失败（账号/权限？）"
  nonempty=0; nt=0
  while IFS= read -r t; do
    [ -n "$t" ] || continue; nt=$((nt+1))
    r="$("${ADMIN_MYSQL[@]}" -N -B -e "SELECT EXISTS(SELECT 1 FROM \`$DB_NAME\`.\`$t\`)" 2>/dev/null)" || die "检查表 $t 失败"
    [ "$r" = "1" ] && { nonempty=1; break; }
  done <<<"$tables"
  if [ "$nonempty" -eq 1 ] && [ "${FORCE:-}" != "1" ]; then
    die "数据库 $DB_NAME 里已经有数据，为避免覆盖，已拒绝导入。
    · 确认要用备份覆盖：FORCE=1 再运行同一条命令（备份里没有的表不会被删除）
    · 只想还原配置并启动、不动数据库：SKIP_DB=1 再运行" 4
  fi
  [ "$nonempty" -eq 1 ] && warn "FORCE=1：将用备份覆盖 $DB_NAME 中同名表"
  "${ADMIN_MYSQL[@]}" <"$S/safe.sql" || die "导入 safe.sql 失败（已停止；库保持导入前状态以外的部分内容可能已写入，可用 FORCE=1 重试）"
  log "导入完成（$nt 张表原已存在 → 已按备份覆盖/创建）"
fi

# ───────────── ⑥ 还原配置（原位置，600）─────────────
step "⑥ 还原配置/密钥文件（权限 600，不显示内容）"
TS="$(date +%Y%m%d-%H%M%S)"
restore_file() { # 源 目标
  local src="$1" dst="$2"
  mkdir -p "$(dirname "$dst")" || die "无法创建目录 $(dirname "$dst")"
  if [ -f "$dst" ] && ! cmp -s "$src" "$dst"; then
    cp -p -- "$dst" "$dst.pre-restore.$TS" && chmod 600 "$dst.pre-restore.$TS"
    log "  已存在且不同，旧文件另存为 ${dst##*/}.pre-restore.$TS"
  fi
  install -m 600 "$src" "$dst" || die "还原失败: $dst"
  log "  ✔ $dst"
}
mkdir -p "$RUN_DIR" && chmod 700 "$RUN_DIR" 2>/dev/null
nsec=0
while IFS= read -r -d '' f; do
  rel="${f#"$S/secrets/"}"
  case "$rel" in
    _abs/*) a="${rel#_abs/}"; case "/$a/" in */../*|*/./*) die "备份里的路径不安全: $rel" ;; esac; dst="$ROOT_PREFIX/$a" ;;
    *)      case "/$rel/" in */../*) die "备份里的路径不安全: $rel" ;; esac; dst="$RUN_DIR/$rel" ;;
  esac
  restore_file "$f" "$dst"; nsec=$((nsec+1))
done < <(find "$S/secrets" -type f -print0 | sort -z)
[ "$nsec" -gt 0 ] || die "secrets/ 里没有任何文件"
log "已还原 $nsec 个配置文件"

# ───────────── ⑦ 还原数据文件 ─────────────
step "⑦ 还原数据文件（已存在的不覆盖）"
if [ -d "$S/files" ] && [ -n "$(ls -A "$S/files" 2>/dev/null)" ]; then
  cp -a -n "$S/files/." "$RUN_DIR/" && log "已复制到 $RUN_DIR"
else log "备份里没有数据文件，跳过"; fi
mkdir -p "$RUN_DIR/bin" "$RUN_DIR/logs"

# ───────────── ⑧ 源码与构建 ─────────────
step "⑧ 取得源码并构建（make build）"
REPO_URL="${SAFE_REPO_URL:-https://github.com/davidmuller5273-boop/safe.git}"
is_safe_repo() { [ -f "$1/go.mod" ] && grep -q '^module github.com/davidmuller5273-boop/safe' "$1/go.mod" && [ -f "$1/Makefile" ]; }
git_auth() { # 用环境变量传令牌（不进命令行、不写 remote）
  if [ -n "${GITHUB_TOKEN:-}" ]; then
    export GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0="http.extraHeader" \
      GIT_CONFIG_VALUE_0="Authorization: Basic $(printf 'x-access-token:%s' "$GITHUB_TOKEN" | base64 -w0)"
  fi
  export GIT_TERMINAL_PROMPT=0
}
REPO_DIR=""
if [ -n "${SAFE_REPO_DIR:-}" ]; then REPO_DIR="$SAFE_REPO_DIR"
elif is_safe_repo "$SCRIPT_DIR/../.."; then REPO_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
else REPO_DIR="${HOME:-/root}/safe"; fi
if is_safe_repo "$REPO_DIR"; then
  log "使用源码目录 $REPO_DIR"
  if [ -d "$REPO_DIR/.git" ] && [ "$REPO_DIR" != "$(cd "$SCRIPT_DIR/../.." && pwd)" ]; then
    ( git_auth; cd "$REPO_DIR" && git pull --ff-only origin main >/dev/null 2>&1 ) && log "已 git pull" || warn "git pull 失败，使用现有源码"
  fi
else
  [ ! -e "$REPO_DIR" ] || [ -z "$(ls -A "$REPO_DIR" 2>/dev/null)" ] || die "$REPO_DIR 已存在但不是 safe 仓库，请移走或设置 SAFE_REPO_DIR"
  log "克隆 $REPO_URL → $REPO_DIR"
  if ! ( git_auth; git clone -q "$REPO_URL" "$REPO_DIR" ) 2>"$TMPW/clone.err"; then
    die "git clone 失败。若仓库是私有的，需要二选一：
    · 带只读令牌重跑：sudo GITHUB_TOKEN=<只读令牌> BACKUP_DECRYPTED=$B bash $0
    · 或先自己把源码放到 $REPO_DIR（或设置 SAFE_REPO_DIR）再重跑
    （令牌需要对 davidmuller5273-boop/safe 有 Contents 只读权限；令牌不会被写入 git 配置或打印）"
  fi
fi
is_safe_repo "$REPO_DIR" || die "$REPO_DIR 不是有效的 safe 源码目录"
if [ -n "${RESTORE_BUILT_BINARY:-}" ]; then
  BIN_SRC="$RESTORE_BUILT_BINARY"     # 仅测试用
else
  [ -n "${GO_BIN:-}" ] || find_go || die "找不到 Go ≥ $GO_NEED（去掉 RESTORE_SKIP_PREREQS 让脚本自动安装）"
  case "$(uname -m)" in aarch64|arm64) ga=arm64 ;; *) ga=amd64 ;; esac
  ( cd "$REPO_DIR" && PATH="$(dirname "$GO_BIN"):$PATH" GOARCH="$ga" make build ) || die "make build 失败（网络拉取 Go 依赖？）。修复后重跑即可。"
  BIN_SRC="$REPO_DIR/bin/safe"
fi
[ -f "$BIN_SRC" ] || die "构建后没有找到 $BIN_SRC"

# ───────────── ⑨ 安装二进制（先停旧进程）─────────────
step "⑨ 停止旧进程并安装二进制到 $RUN_DIR/bin/safe"
BIN_DST="$RUN_DIR/bin/safe"
stop_safe() {
  local pids i
  pids="$(pgrep -f "($RUN_DIR/)?(bin/)?safe (admin|lottery-collector|safew-bot)( |\$)" 2>/dev/null | grep -vx "$$" || true)"
  [ -n "$pids" ] || return 0
  # shellcheck disable=SC2086
  kill $pids 2>/dev/null
  for i in $(seq 1 20); do
    kill -0 $pids 2>/dev/null || return 0
    sleep 0.5
  done
  # shellcheck disable=SC2086
  kill -9 $pids 2>/dev/null; sleep 1
}
[ "${RESTORE_NO_START:-}" = "1" ] || stop_safe
install -m 755 "$BIN_SRC" "$BIN_DST.new" && mv -f "$BIN_DST.new" "$BIN_DST" || die "安装二进制失败（旧进程未退出？）"
log "已安装 $BIN_DST"

# ───────────── ⑩ 启动三个进程 ─────────────
if [ "${RESTORE_NO_START:-}" = "1" ]; then
  step "⑩ RESTORE_NO_START=1：不启动进程"
else
  step "⑩ 启动 admin / lottery-collector / safew-bot（nohup，日志在 $RUN_DIR/logs/）"
  # 应用本身不读取 .env：这里只解析 KEY=VALUE 并导出给子进程（不 source，不经命令行传递）
  ENV_LINES=()
  if [ -f "$RUN_DIR/.env" ]; then
    while IFS= read -r line || [ -n "$line" ]; do
      line="${line%$'\r'}"
      [[ "$line" =~ ^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=(.*)$ ]] || continue
      k="${BASH_REMATCH[2]}"; v="${BASH_REMATCH[3]}"; v="${v#"${v%%[![:space:]]*}"}"
      if [[ "$v" == \"* ]]; then v="${v#\"}"; v="${v%\"*}"
      elif [[ "$v" == \'* ]]; then v="${v#\'}"; v="${v%\'*}"
      else v="${v%%[[:space:]]#*}"; v="${v%"${v##*[![:space:]]}"}"; fi
      ENV_LINES+=("$k=$v")
    done <"$RUN_DIR/.env"
  fi
  for svc in admin lottery-collector safew-bot; do
    (
      for kv in "${ENV_LINES[@]:-}"; do [ -n "$kv" ] && export "$kv"; done
      cd "$RUN_DIR" || exit 1
      umask 077
      exec setsid nohup ./bin/safe "$svc" >>"logs/$svc.log" 2>&1 <&- &
    )
  done
  sleep "${RESTORE_START_WAIT:-3}"
  bad=0
  for svc in admin lottery-collector safew-bot; do
    if pgrep -f "bin/safe $svc( |\$)" >/dev/null 2>&1; then log "  ✔ $svc 运行中"
    else warn "  ✖ $svc 已退出，请查看 $RUN_DIR/logs/$svc.log（常见原因：Redis/MySQL 未就绪）"; bad=1; fi
  done
  echo
  log "进程列表："
  ps -eo pid,etime,args 2>/dev/null | grep -E '[b]in/safe (admin|lottery-collector|safew-bot)' | sed "s/^/    /"
  [ "$bad" -eq 0 ] || die "有进程没有起来，见上方提示" 5
fi

echo
log "✅ 恢复完成。进程用 nohup 启动，服务器重启后需再次运行本命令（或自行配置 systemd）。"
log "   核对：cat $S/MANIFEST.txt ；确认无误后删除解密目录：rm -rf $B"
exit 0
