#!/usr/bin/env bash
# /etc/tg-bot-backup.d/safe.sh — SafeW 机器人（safe 项目）备份钩子。
# 由 tg-directory-bot 的备份脚本以 root 身份执行：  safe.sh <暂存目录>
# 本钩子只往 "<暂存目录>/safe/" 写文件；由备份脚本统一加密并推送。
#
# 备份内容：
#   safe/safe.sql        应用数据库的一致性 mysqldump（--single-transaction，含存储过程/触发器/事件）
#   safe/secrets/...     运行目录里的 .env / *.env / config/*.yml 等配置文件（含密码、JWT、令牌；
#                        按用户要求纳入备份，权限保持 600，目录 700；整个备份由 tgbot-backup 加密后才离开本机，
#                        绝不能以明文进入 git），以及实际生效的 CONFIG_FILE / env 文件（若在运行目录之外，放在 secrets/_abs/ 下）
#   safe/files/...       /root/safex 下的小型、不含敏感信息的数据文件
#   safe/MANIFEST.txt    本次备份清单（只有文件名，不含任何密码/令牌）
# 跳过（files/ 部分）：bin/、logs/、*.log、.git、node_modules、可执行文件、>5MiB 的文件、符号链接、
#      *.pem/*.key 等密钥文件，以及内容里出现 password/secret/token/api_key/私钥 字样的其它文件。
# 脚本任何时候都不会把密码/令牌打印到屏幕或日志；数据库密码只写入权限 600 的临时 --defaults-extra-file，用完即删。
#
# 可用环境变量（一般无需设置）：SAFE_RUN_DIR（默认 /root/safex）、MYSQLDUMP、
#   SAFE_ENV_FILES（额外的 env 文件，冒号分隔）、SAFE_BACKUP_MAX_FILE_BYTES。
set -u
set -o pipefail

TAG="[safe-backup]"
RUN_DIR="${SAFE_RUN_DIR:-/root/safex}"
PROC_ROOT="${PROC_ROOT:-/proc}"
MYSQLDUMP="${MYSQLDUMP:-mysqldump}"
MAX_FILE_BYTES="${SAFE_BACKUP_MAX_FILE_BYTES:-5242880}"

log() { printf '%s %s\n' "$TAG" "$*"; }
err() { printf '%s %s\n' "$TAG" "$*" >&2; }

STAGE="${1:-}"
if [ -z "$STAGE" ] || [ ! -d "$STAGE" ]; then
  err "用法: safe.sh <暂存目录>（目录必须已存在）"
  exit 64
fi
OUT="$STAGE/safe"
umask 077
mkdir -p "$OUT" || { err "无法创建 $OUT"; exit 1; }
chmod 700 "$OUT" 2>/dev/null || true

# ---------- 清理：临时文件一律删除 ----------
DEFAULTS_FILE=""
ERR_FILE=""
cleanup() {
  [ -n "$DEFAULTS_FILE" ] && rm -f -- "$DEFAULTS_FILE"
  [ -n "$ERR_FILE" ] && rm -f -- "$ERR_FILE"
  [ -n "${DUMP_TMP:-}" ] && rm -f -- "$DUMP_TMP"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

# ---------- 读取配置：与应用 internal/config 一致 ----------
# 应用优先级：环境变量 > CONFIG_FILE 指向的 yaml（默认 config/app.yml，相对运行目录）> 默认值。
# 应用本身不读取 .env 文件（由 systemd EnvironmentFile 或 shell 导出），所以这里依次查：
#   1) 正在运行的 safe 进程的真实环境（/proc/<pid>/environ，root 可读）
#   2) $RUN_DIR/.env、$RUN_DIR/safe.env、/etc/safe/safe.env、SAFE_ENV_FILES（只解析 KEY=VALUE，绝不 source）
#   3) yaml
#   4) 应用默认值
ENV_FILES=("$RUN_DIR/.env" "$RUN_DIR/safe.env" "/etc/safe/safe.env")
if [ -n "${SAFE_ENV_FILES:-}" ]; then
  IFS=':' read -r -a _extra <<<"$SAFE_ENV_FILES"
  ENV_FILES+=("${_extra[@]}")
fi

safe_pids() {
  if [ -n "${SAFE_PIDS:-}" ]; then
    printf '%s\n' $SAFE_PIDS
  elif command -v pgrep >/dev/null 2>&1; then
    pgrep -f '(^|/)safe (admin|lottery-collector|safew-bot)( |$)' 2>/dev/null
  fi
}

proc_env_get() { # KEY
  local key="$1" pid f line
  while read -r pid; do
    [ -n "$pid" ] || continue
    f="$PROC_ROOT/$pid/environ"
    [ -r "$f" ] || continue
    line=$(tr '\0' '\n' <"$f" 2>/dev/null | grep -m1 -- "^${key}=" || true)
    if [ -n "$line" ] && [ -n "${line#*=}" ]; then
      printf '%s' "${line#*=}"
      return 0
    fi
  done < <(safe_pids)
  return 1
}

dotenv_get() { # KEY FILE  — 最后一次出现的非空值生效
  local key="$1" file="$2" line v val="" found=1
  [ -r "$file" ] && [ -f "$file" ] || return 1
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line%$'\r'}"
    if [[ "$line" =~ ^[[:space:]]*(export[[:space:]]+)?${key}[[:space:]]*=(.*)$ ]]; then
      v="${BASH_REMATCH[2]}"
      v="${v#"${v%%[![:space:]]*}"}"
      if [[ "$v" == \"* ]]; then
        v="${v#\"}"; v="${v%\"*}"
      elif [[ "$v" == \'* ]]; then
        v="${v#\'}"; v="${v%\'*}"
      else
        v="${v%%[[:space:]]#*}"
        v="${v%"${v##*[![:space:]]}"}"
      fi
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
  } else {
    sub(/[ \t]+#.*$/, "", v); val = trim(v)
  }
  found = 1
}
END { if (found) printf "%s", val }
AWK

yaml_get() { # FILE SECTION KEY
  [ -r "$1" ] && [ -f "$1" ] || return 1
  local v
  v=$(awk -v sect="$2" -v want="$3" "$AWK_YAML" "$1" 2>/dev/null) || return 1
  [ -n "$v" ] || return 1
  printf '%s' "$v"
}

CFG_SRC=""
# cfg_get ENVKEY YAML_SECTION YAML_KEY DEFAULT  -> 值写入 CFG_VAL，来源写入 CFG_SRC（不用子 shell，变量才能保留）
CFG_VAL=""
cfg_get() {
  local key="$1" sect="$2" ykey="$3" def="$4" v f
  if v=$(proc_env_get "$key") && [ -n "$v" ]; then CFG_SRC="运行中进程环境"; CFG_VAL="$v"; return 0; fi
  for f in "${ENV_FILES[@]}"; do
    if v=$(dotenv_get "$key" "$f") && [ -n "$v" ]; then CFG_SRC="$f"; CFG_VAL="$v"; return 0; fi
  done
  if [ -n "$sect" ] && [ -n "${CONFIG_PATH:-}" ] && v=$(yaml_get "$CONFIG_PATH" "$sect" "$ykey") && [ -n "$v" ]; then
    CFG_SRC="$CONFIG_PATH"; CFG_VAL="$v"; return 0
  fi
  CFG_SRC="默认值"
  CFG_VAL="$def"
}

# CONFIG_FILE（相对路径按运行目录解析，与应用 WorkingDirectory 一致）
cfg_get CONFIG_FILE "" "" "config/app.yml"; cf="$CFG_VAL"
case "$cf" in /*) CONFIG_PATH="$cf" ;; *) CONFIG_PATH="$RUN_DIR/$cf" ;; esac
CONFIG_NOTE="$CONFIG_PATH"
[ -f "$CONFIG_PATH" ] || CONFIG_NOTE="$CONFIG_PATH（不存在，使用环境变量/默认值）"

cfg_get MYSQL_HOST mysql host 127.0.0.1; DB_HOST="$CFG_VAL"; S_HOST="$CFG_SRC"
cfg_get MYSQL_PORT mysql port 3306; DB_PORT="$CFG_VAL"; S_PORT="$CFG_SRC"
cfg_get MYSQL_USER mysql user safe; DB_USER="$CFG_VAL"; S_USER="$CFG_SRC"
cfg_get MYSQL_PASSWORD mysql password CHANGE_ME; DB_PASS="$CFG_VAL"; S_PASS="$CFG_SRC"
cfg_get MYSQL_DATABASE mysql database safe; DB_NAME="$CFG_VAL"; S_NAME="$CFG_SRC"

if ! [[ "$DB_PORT" =~ ^[0-9]+$ ]]; then err "MYSQL_PORT 不是数字: $DB_PORT"; exit 2; fi
if ! [[ "$DB_NAME" =~ ^[A-Za-z0-9_\$-]+$ ]]; then err "数据库名含有不支持的字符，已中止"; exit 2; fi
if [ "$DB_PASS" = "CHANGE_ME" ]; then
  err "警告：读到的数据库密码仍是示例值 CHANGE_ME，可能没找到真实配置（已查 $CONFIG_PATH、${ENV_FILES[*]}、运行中进程环境）"
fi

# ---------- mysqldump ----------
if ! command -v "$MYSQLDUMP" >/dev/null 2>&1; then
  err "未找到 mysqldump，无法备份 safe 数据库。请先安装客户端："
  err "    sudo apt update && sudo apt install -y mysql-client     # 若系统用 MariaDB：sudo apt install -y mariadb-client"
  err "安装后重新执行 sudo tgbot-backup 即可。"
  exit 3
fi

mk_defaults_file() {
  local p="$DB_PASS"
  p="${p//\\/\\\\}"; p="${p//\"/\\\"}"
  DEFAULTS_FILE=$(mktemp "${TMPDIR:-/tmp}/safe-backup-my.XXXXXX") || return 1
  chmod 600 "$DEFAULTS_FILE"
  {
    printf '[client]\n'
    printf 'user="%s"\n' "${DB_USER//\"/\\\"}"
    printf 'password="%s"\n' "$p"
    printf 'host="%s"\n' "$DB_HOST"
    printf 'port=%s\n' "$DB_PORT"
  } >"$DEFAULTS_FILE"
}
mk_defaults_file || { err "无法创建临时凭据文件"; exit 1; }
DB_PASS=""   # 之后不再需要明文

help_text=$("$MYSQLDUMP" --help 2>/dev/null || true)
dump_opts=(--single-transaction --routines --triggers --events --hex-blob --default-character-set=utf8mb4 --databases)
grep -q -- '--no-tablespaces' <<<"$help_text" && dump_opts+=(--no-tablespaces)
grep -q -- '--column-statistics' <<<"$help_text" && dump_opts+=(--column-statistics=0)

DUMP_TMP="$OUT/safe.sql.partial"
ERR_FILE=$(mktemp "${TMPDIR:-/tmp}/safe-backup-err.XXXXXX") || ERR_FILE=""
log "开始导出数据库 $DB_NAME（$DB_USER@$DB_HOST:$DB_PORT）…"
if ! "$MYSQLDUMP" "--defaults-extra-file=$DEFAULTS_FILE" "${dump_opts[@]}" "$DB_NAME" >"$DUMP_TMP" 2>"${ERR_FILE:-/dev/null}"; then
  err "mysqldump 失败（退出码非 0），safe 数据库未备份。错误信息："
  [ -n "$ERR_FILE" ] && sed 's/^/    /' "$ERR_FILE" >&2
  rm -f -- "$DUMP_TMP"
  exit 1
fi
if [ -n "$ERR_FILE" ] && [ -s "$ERR_FILE" ]; then
  sed 's/^/    mysqldump: /' "$ERR_FILE" >&2   # 警告类信息
fi
if ! tail -n 5 "$DUMP_TMP" | grep -q -- '-- Dump completed'; then
  err "导出文件不完整（缺少 '-- Dump completed' 结尾），已丢弃。"
  rm -f -- "$DUMP_TMP"
  exit 1
fi
mv -f -- "$DUMP_TMP" "$OUT/safe.sql" && DUMP_TMP=""
chmod 600 "$OUT/safe.sql"
rm -f -- "$DEFAULTS_FILE"; DEFAULTS_FILE=""
DUMP_BYTES=$(wc -c <"$OUT/safe.sql" | tr -d ' ')
log "数据库已导出: safe/safe.sql（${DUMP_BYTES} 字节）"

# ---------- 运行目录下的小型数据文件 ----------
COPIED=()
SKIPPED=()
SECRET_RE='password|passwd|secret|token|api[_-]?key|private[ _-]?key|BEGIN [A-Z ]*PRIVATE'
if [ -d "$RUN_DIR" ]; then
  while IFS= read -r -d '' f; do
    rel="${f#"$RUN_DIR"/}"
    base="${f##*/}"
    reason=""
    case "$base" in
      .env|.env.*|*.env|*.yml|*.yaml|*.pem|*.key|*.p12|*.pfx|*.crt|*.sock|*.pid|*.log|*.log.*|id_rsa*|id_ed25519*|*.swp|*~)
        reason="配置/密钥/日志类文件名（.env 与 config 已单独放入 secrets/）" ;;
    esac
    if [ -z "$reason" ] && [ -x "$f" ]; then reason="可执行文件"; fi
    if [ -z "$reason" ]; then
      size=$(wc -c <"$f" 2>/dev/null | tr -d ' ')
      if [ -z "$size" ]; then reason="无法读取"
      elif [ "$size" -gt "$MAX_FILE_BYTES" ]; then reason="超过大小上限"; fi
    fi
    if [ -z "$reason" ] && grep -qaiE -- "$SECRET_RE" "$f" 2>/dev/null; then reason="内容含 password/secret/token 等敏感字样"; fi
    if [ -n "$reason" ]; then SKIPPED+=("$rel  [$reason]"); continue; fi
    dest="$OUT/files/$rel"
    if mkdir -p "$(dirname "$dest")" && cp -p -- "$f" "$dest"; then COPIED+=("$rel"); else SKIPPED+=("$rel  [复制失败]"); fi
  done < <(find "$RUN_DIR" \
      \( -type d \( -name bin -o -name logs -o -name log -o -name config -o -name .git -o -name node_modules -o -name tmp -o -name cache \) -prune \) \
      -o \( -type f -print0 \) 2>/dev/null)
else
  err "运行目录 $RUN_DIR 不存在，跳过数据文件。"
fi
log "数据文件: 已复制 ${#COPIED[@]} 个，已跳过 ${#SKIPPED[@]} 个"

# ---------- 配置/密钥文件（.env、config）：按用户要求备份，权限 600，只记录文件名 ----------
SECRET_FILES=()
SECRET_DIR="$OUT/secrets"
add_secret_file() { # 绝对路径
  local f="$1" rel dest s
  [ -f "$f" ] && [ ! -L "$f" ] || return 0
  for s in "${SECRET_FILES[@]:-}"; do [ "$s" = "$f" ] && return 0; done
  case "$f" in
    "$RUN_DIR"/*) rel="${f#"$RUN_DIR"/}" ;;
    *)            rel="_abs/${f#/}" ;;
  esac
  dest="$SECRET_DIR/$rel"
  if mkdir -p "$(dirname "$dest")" && cp -- "$f" "$dest" && chmod 600 "$dest"; then
    SECRET_FILES+=("$f")
  else
    err "复制配置文件失败: $f"
  fi
}
if [ -d "$RUN_DIR" ]; then
  mkdir -p "$SECRET_DIR" && chmod 700 "$SECRET_DIR"
  while IFS= read -r -d '' f; do add_secret_file "$f"; done < <(find "$RUN_DIR" -maxdepth 3 \
      \( -type d \( -name bin -o -name logs -o -name log -o -name .git -o -name node_modules \) -prune \) \
      -o \( -type f \( -name '.env' -o -name '.env.*' -o -name '*.env' -o -name '*.yml' -o -name '*.yaml' \) -print0 \) 2>/dev/null)
fi
# 实际生效的配置来源（可能在运行目录之外，例如 /etc/safe/safe.env）
add_secret_file "$CONFIG_PATH"
for f in "${ENV_FILES[@]}"; do add_secret_file "$f"; done
if [ -d "$SECRET_DIR" ]; then
  find "$SECRET_DIR" -type d -exec chmod 700 {} + 2>/dev/null || true
  find "$SECRET_DIR" -type f -exec chmod 600 {} + 2>/dev/null || true
fi
log "配置/密钥文件（.env、config）: 已备份 ${#SECRET_FILES[@]} 个到 safe/secrets/（权限 600；内容不显示）"

# ---------- 清单（不含任何密码/令牌）----------
{
  echo "safe 备份清单  $(date '+%F %T %z')"
  echo "数据库: $DB_NAME  连接: $DB_USER@$DB_HOST:$DB_PORT  导出大小: ${DUMP_BYTES} 字节"
  echo "配置来源(仅路径/类型，不含密码): host=$S_HOST port=$S_PORT user=$S_USER password=$S_PASS database=$S_NAME"
  echo "配置文件: $CONFIG_NOTE"
  echo "运行目录: $RUN_DIR"
  echo
  echo "配置/密钥文件（safe/secrets/ 下，权限 600，仅列文件名，备份包整体加密）:"
  if [ ${#SECRET_FILES[@]} -eq 0 ]; then echo "  （无）"; else printf '  %s\n' "${SECRET_FILES[@]}"; fi
  echo
  echo "已复制的数据文件（safe/files/ 下）:"
  if [ ${#COPIED[@]} -eq 0 ]; then echo "  （无）"; else printf '  %s\n' "${COPIED[@]}"; fi
  echo
  echo "未复制到 files/（日志/二进制/过大/含敏感字样的其它文件；bin/ logs/ .git 整体跳过；.env 与 config 见上方 secrets/）:"
  if [ ${#SKIPPED[@]} -eq 0 ]; then echo "  （无）"; else printf '  %s\n' "${SKIPPED[@]}"; fi
} >"$OUT/MANIFEST.txt"
chmod 600 "$OUT/MANIFEST.txt"
log "完成。清单: safe/MANIFEST.txt"
exit 0
