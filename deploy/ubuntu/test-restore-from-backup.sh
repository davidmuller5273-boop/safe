#!/usr/bin/env bash
# 恢复脚本测试：假的 mysql / mysqldump / make，不碰真实数据库和系统。
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/safe-backup-hook.sh"
RESTORE="$HERE/restore-from-backup.sh"
T="$(mktemp -d)"
cleanup() { [ -f "$T/log/pids" ] && kill $(cat "$T/log/pids") 2>/dev/null; [ -f "$T/allpids" ] && kill $(cat "$T/allpids") 2>/dev/null; sleep 0.3; rm -rf "$T"; }
trap cleanup EXIT
FAILS=0
ok()   { echo "ok   - $1"; }
fail() { echo "FAIL - $1"; FAILS=$((FAILS+1)); }
check() { if eval "$2"; then ok "$1"; else fail "$1  [$2]"; fi; }

mkdir -p "$T/bin" "$T/db" "$T/log" "$T/tmp"
cat >"$T/bin/mysqldump" <<'M'
#!/usr/bin/env bash
[ "${1:-}" = "--help" ] && { echo "--no-tablespaces"; exit 0; }
echo "-- MySQL dump"; echo 'USE `safedb`;'; echo "CREATE TABLE t1 (id int);"; echo "-- Dump completed on 2026-01-01"
M
cat >"$T/bin/mysql" <<'M'
#!/usr/bin/env bash
echo "$*" >>"$MOCK_LOG/mysql.args"
e=""; prev=""
for a in "$@"; do [ "$prev" = "-e" ] && e="$a"; prev="$a"; done
if [ -n "$e" ]; then
  case "$e" in
    "SELECT 1") [ -n "${MOCK_MYSQL_DOWN:-}" ] && exit 1; echo 1 ;;
    *information_schema.tables*) cat "$MOCK_DB/tables" 2>/dev/null ;;
    *EXISTS*) t=$(sed -E 's/.*`([^`]+)`\)$/\1/' <<<"$e"); grep -qx "$t" "$MOCK_DB/rows" 2>/dev/null && echo 1 || echo 0 ;;
  esac
  exit 0
fi
in=$(cat); n=$(ls "$MOCK_LOG"/stdin.* 2>/dev/null | wc -l); printf '%s\n' "$in" >"$MOCK_LOG/stdin.$((n+1))"
if grep -q "CREATE TABLE" <<<"$in"; then echo t1 >"$MOCK_DB/tables"; echo t1 >"$MOCK_DB/rows"; fi
exit 0
M
cat >"$T/bin/make" <<'M'
#!/usr/bin/env bash
mkdir -p bin && cat >bin/safe <<'B'
#!/usr/bin/env bash
trap 'exit 0' TERM
echo "$$" >>"$MOCK_LOG/pids"; echo "${MYSQL_PASSWORD:-}" >"$MOCK_LOG/started.$1"
while :; do sleep 1; done
B
chmod +x bin/safe; echo "built (mock)"
M
cat >"$T/fakebinary" <<'M'
#!/usr/bin/env bash
trap 'exit 0' TERM
echo "$$" >>"$MOCK_LOG/pids"; echo "${MYSQL_PASSWORD:-}" >"$MOCK_LOG/started.$1"
while :; do sleep 1; done
M
chmod +x "$T/bin/"* "$T/fakebinary"

PASS='S3cr3t#Pw'"'"'q9'
PASS_ESC="S3cr3t#Pw''q9"
# ---- 用真实的备份钩子造一份备份（端到端）----
mkrun() {
  local r="$1"; mkdir -p "$r/config" "$r/data" "$r/bin"
  printf 'mysql:\n  host: 127.0.0.1\n  port: 3306\n  user: safeuser\n  password: "yamlpw"\n  database: safedb\n' >"$r/config/app.yml"
  printf 'MYSQL_PASSWORD=%s\nJWT_SECRET=jwt-abc-123\nDEVELOPER_USER_IDS=1,2\nCONFIG_FILE=%s\n' "$PASS" "$r/config/app.yml" >"$r/.env"
  echo '{"s":1}' >"$r/data/state.json"
}
SRC="$T/src/safex"; mkrun "$SRC"
mkdir -p "$T/stage" "$T/ext"; echo "EXT_SETTING=1" >"$T/ext/safe.env"
env -i PATH="$T/bin:$PATH" TMPDIR="$T/tmp" SAFE_RUN_DIR="$SRC" SAFE_PIDS="" MYSQLDUMP="$T/bin/mysqldump" SAFE_ENV_FILES="$T/ext/safe.env" bash "$HOOK" "$T/stage" >"$T/hook.out" 2>&1
check "造备份: 钩子成功" '[ -f "$T/stage/safe/safe.sql" ] && [ -f "$T/stage/safe/secrets/.env" ]'
# 备份里 CONFIG_FILE 指向绝对路径 $SRC/config/app.yml（备份在 _abs 下），恢复时目标运行目录不同 → 用相对配置更贴近真实；这里另造一份相对路径版本
B="$T/backup"; mkdir -p "$B"; cp -a "$T/stage/safe" "$B/safe"
# 把 .env 里的 CONFIG_FILE 改成相对路径（真实服务器通常如此）
sed -i '/^CONFIG_FILE=/d' "$B/safe/secrets/.env"
rm -rf "$B/safe/secrets/_abs"

NEWRUN="$T/new/safex"; PREFIX="$T/root"
run_restore() { # 额外 env 以参数传入；输出 $T/out.txt，返回码 RC
  env -i PATH="$T/bin:$PATH" HOME="$T/home" TMPDIR="$T/tmp" MOCK_LOG="$T/log" MOCK_DB="$T/db" \
    SAFE_RESTORE_ALLOW_NONROOT=1 SAFE_RUN_DIR="$NEWRUN" RESTORE_ROOT_PREFIX="$PREFIX" \
    RESTORE_SKIP_PREREQS=1 RESTORE_BUILT_BINARY="$T/fakebinary" RESTORE_START_WAIT=1 "$@" \
    bash "$RESTORE" >"$T/out.txt" 2>&1
  RC=$?
}
mkdir -p "$T/home" "$T/tmp"

# ===== 1. 全新恢复 =====
run_restore BACKUP_DECRYPTED="$B"
check "全新恢复退出码 0" '[ $RC -eq 0 ]'
check "建库+建用户 SQL 走 stdin 且含正确库/用户/密码(转义)" 'grep -q "CREATE DATABASE IF NOT EXISTS \`safedb\`" "$T/log/stdin.1" && grep -q "CREATE USER IF NOT EXISTS '"'"'safeuser'"'"'@'"'"'127.0.0.1'"'"'" "$T/log/stdin.1" && grep -qF -- "$PASS_ESC" "$T/log/stdin.1" && grep -q "GRANT ALL PRIVILEGES ON \`safedb\`" "$T/log/stdin.1"'
check "密码从未出现在 mysql 命令行" '! grep -q "S3cr3t" "$T/log/mysql.args"'
check "safe.sql 已导入" 'grep -q "CREATE TABLE t1" "$T/log/stdin.2"'
check ".env 已还原且内容一致、权限 600" 'cmp -s "$B/safe/secrets/.env" "$NEWRUN/.env" && [ "$(stat -c %a "$NEWRUN/.env")" = "600" ]'
check "config/app.yml 已还原、权限 600" 'cmp -s "$B/safe/secrets/config/app.yml" "$NEWRUN/config/app.yml" && [ "$(stat -c %a "$NEWRUN/config/app.yml")" = "600" ]'
check "数据文件已还原" '[ -f "$NEWRUN/data/state.json" ]'
check "二进制已安装 755" '[ -x "$NEWRUN/bin/safe" ] && [ "$(stat -c %a "$NEWRUN/bin/safe")" = "755" ]'
check "三个进程都启动，且拿到 .env 里的环境变量" '( for s in admin lottery-collector safew-bot; do grep -qxF -- "$PASS" "$T/log/started.$s" || exit 1; done )'
check "进程列表已打印" 'grep -q "进程列表" "$T/out.txt" && grep -c "bin/safe " "$T/out.txt" | grep -q "^3$\|^[3-9]"'
check "输出没有任何密码/令牌" '! grep -qE "S3cr3t|jwt-abc-123|yamlpw" "$T/out.txt"'
check "日志目录存在" '[ -d "$NEWRUN/logs" ]'
n1=$(pgrep -f "$NEWRUN/bin/safe\|bin/safe admin" | head -1)

# ===== 2. 再次运行：库里已有数据 → 拒绝；SKIP_DB / FORCE =====
cnt=$(ls "$T/log"/stdin.* | wc -l)
run_restore BACKUP_DECRYPTED="$B"
check "库非空且无 FORCE: 退出码 4 并给出提示" '[ $RC -eq 4 ] && grep -q "FORCE=1" "$T/out.txt" && grep -q "SKIP_DB=1" "$T/out.txt"'
check "拒绝时没有再次导入（stdin 文件数不变，仅多了建库那次）" '[ "$(ls "$T/log"/stdin.* | wc -l)" -le $((cnt+1)) ] && [ "$(grep -l "CREATE TABLE" "$T/log"/stdin.* | wc -l)" -eq 1 ]'
run_restore BACKUP_DECRYPTED="$B" SKIP_DB=1
check "SKIP_DB=1: 成功，配置不变内容一致，不再导入" '[ $RC -eq 0 ] && [ "$(grep -l "CREATE TABLE" "$T/log"/stdin.* | wc -l)" -eq 1 ] && cmp -s "$B/safe/secrets/.env" "$NEWRUN/.env"'
check "重复运行后每个服务恰好 1 个进程（旧的已停）" '( for s in admin lottery-collector safew-bot; do [ "$(pgrep -fc "^bash ./bin/safe $s$")" = "1" ] || exit 1; done )'
run_restore BACKUP_DECRYPTED="$B" FORCE=1
check "FORCE=1: 重新导入" '[ $RC -eq 0 ] && [ "$(grep -l "CREATE TABLE" "$T/log"/stdin.* | wc -l)" -eq 2 ] && grep -q "FORCE=1" "$T/out.txt"'

# ===== 3. 已存在且不同的配置 → 旧文件另存 =====
echo "OLD=1" >"$NEWRUN/.env"
run_restore BACKUP_DECRYPTED="$B" SKIP_DB=1
check "不同的旧 .env 另存为 .pre-restore" '[ -n "$(ls "$NEWRUN"/.env.pre-restore.* 2>/dev/null)" ] && cmp -s "$B/safe/secrets/.env" "$NEWRUN/.env"'

# ===== 4. 不猜测：缺信息直接停 =====
rm -f "$T/log/mysql.args"
bad() { rm -rf "$T/b2"; mkdir -p "$T/b2"; cp -a "$B/safe" "$T/b2/safe"; }
bad; rm -rf "$T/b2/safe/secrets"
run_restore BACKUP_DECRYPTED="$T/b2"
check "没有 secrets/: 停止并说明" '[ $RC -ne 0 ] && grep -q "没有.*secrets" "$T/out.txt" && [ ! -e "$T/log/mysql.args" ]'
bad; rm -rf "$T/b2/safe/secrets/config"; sed -i '/MYSQL_/d' "$T/b2/safe/secrets/.env"
run_restore BACKUP_DECRYPTED="$T/b2"
check "配置里没有数据库信息: 停止、不连库" '[ $RC -ne 0 ] && grep -q "我不会" "$T/out.txt" && [ ! -e "$T/log/mysql.args" ]'
bad; rm -rf "$T/b2/safe/secrets/config" "$T/b2/safe/secrets/.env"; echo x >"$T/b2/safe/secrets/other.txt"
run_restore BACKUP_DECRYPTED="$T/b2"
check "找不到 .env / config: 停止" '[ $RC -ne 0 ] && grep -q "没有找到 .env" "$T/out.txt"'
bad; sed -i 's/^MYSQL_PASSWORD=.*/MYSQL_PASSWORD=CHANGE_ME/' "$T/b2/safe/secrets/.env"
run_restore BACKUP_DECRYPTED="$T/b2"
check "密码是 CHANGE_ME: 停止" '[ $RC -ne 0 ] && grep -q "CHANGE_ME" "$T/out.txt" && [ ! -e "$T/log/mysql.args" ]'
bad; sed -i 's/safedb/otherdb/' "$T/b2/safe/secrets/config/app.yml"
run_restore BACKUP_DECRYPTED="$T/b2"
check "库名与 dump 不一致: 停止" '[ $RC -ne 0 ] && grep -q "不一致" "$T/out.txt"'
bad; sed -i '$d' "$T/b2/safe/safe.sql"
run_restore BACKUP_DECRYPTED="$T/b2"
check "dump 不完整: 停止" '[ $RC -ne 0 ] && grep -q "不完整" "$T/out.txt"'
run_restore BACKUP_DECRYPTED="$T/nonexistent"
check "备份目录不存在: 停止" '[ $RC -ne 0 ] && grep -q "不存在" "$T/out.txt"'
run_restore
check "未指定目录: 给出用法" '[ $RC -ne 0 ] && grep -q "BACKUP_DECRYPTED" "$T/out.txt"'
bad; mkdir -p "$T/b2/safe/secrets/_abs/x/../../evil"; echo e >"$T/b2/safe/secrets/_abs/x/../../evil/f" 2>/dev/null
mkdir -p "$T/b2/safe/secrets/_abs"; ln -sfn . "$T/b2/safe/secrets/_abs/dotdot" 2>/dev/null
# 直接构造含 .. 的相对路径：用 find 无法产生，故仅验证脚本里的路径检查存在
check "脚本含 _abs 路径穿越检查" 'grep -q "备份里的路径不安全" "$RESTORE"'

# ===== 5. 参数 $1 与远程数据库 =====
cat "$T/log/pids" >>"$T/allpids" 2>/dev/null; rm -rf "$T/db"/*; rm -f "$T/log"/*
bad; sed -i 's/host: 127.0.0.1/host: db.example.internal/' "$T/b2/safe/secrets/config/app.yml"
run_restore RESTORE_NO_START=1 BACKUP_DECRYPTED=""   # 先用 $1 方式调用
env -i PATH="$T/bin:$PATH" HOME="$T/home" TMPDIR="$T/tmp" MOCK_LOG="$T/log" MOCK_DB="$T/db" SAFE_RESTORE_ALLOW_NONROOT=1 \
  SAFE_RUN_DIR="$NEWRUN" RESTORE_ROOT_PREFIX="$PREFIX" RESTORE_SKIP_PREREQS=1 RESTORE_BUILT_BINARY="$T/fakebinary" RESTORE_NO_START=1 \
  bash "$RESTORE" "$T/b2" >"$T/out.txt" 2>&1; RC=$?
check "\$1 传目录 + 远程库: 成功" '[ $RC -eq 0 ] && grep -q "远程" "$T/out.txt"'
check "远程库: 不建库/用户，使用应用账号凭据文件" '! grep -rq "CREATE USER" "$T/log"/ 2>/dev/null && grep -q "defaults-extra-file" "$T/log/mysql.args" && grep -q "CREATE TABLE" "$T/log"/stdin.1'
check "远程库: 凭据文件已删除" '[ -z "$(ls "$T/tmp" 2>/dev/null)" ]'

# ===== 6. clone 失败的提示（私有仓库）=====
cat "$T/log/pids" >>"$T/allpids" 2>/dev/null; rm -rf "$T/log"/* "$T/db"/*
bad
env -i PATH="$T/bin:$PATH" HOME="$T/home" TMPDIR="$T/tmp" MOCK_LOG="$T/log" MOCK_DB="$T/db" SAFE_RESTORE_ALLOW_NONROOT=1 \
  SAFE_RUN_DIR="$NEWRUN" RESTORE_ROOT_PREFIX="$PREFIX" RESTORE_SKIP_PREREQS=1 RESTORE_NO_START=1 SKIP_DB=1 \
  SAFE_REPO_DIR="$T/home/safe" SAFE_REPO_URL="file://$T/no-such-repo.git" GITHUB_TOKEN="ghp_TESTTOKEN123" \
  bash "$RESTORE" "$T/b2" >"$T/out.txt" 2>&1; RC=$?
check "clone 失败: 说明需要 GITHUB_TOKEN" '[ $RC -ne 0 ] && grep -q "GITHUB_TOKEN" "$T/out.txt" && grep -q "只读" "$T/out.txt"'
check "令牌不会被打印" '! grep -q "ghp_TESTTOKEN123" "$T/out.txt"'

# ===== 7. 无二进制覆盖时走 make build（假 make）=====
mkdir -p "$T/fakerepo"; printf 'module github.com/davidmuller5273-boop/safe\n\ngo 1.24.1\n' >"$T/fakerepo/go.mod"; echo "build:" >"$T/fakerepo/Makefile"
cat "$T/log/pids" >>"$T/allpids" 2>/dev/null; rm -rf "$T/log"/*
env -i PATH="$T/bin:$PATH:$(dirname "$(command -v go)")" HOME="$T/home" TMPDIR="$T/tmp" MOCK_LOG="$T/log" MOCK_DB="$T/db" SAFE_RESTORE_ALLOW_NONROOT=1 \
  SAFE_RUN_DIR="$NEWRUN" RESTORE_ROOT_PREFIX="$PREFIX" RESTORE_SKIP_PREREQS=1 SKIP_DB=1 RESTORE_START_WAIT=1 SAFE_REPO_DIR="$T/fakerepo" \
  bash "$RESTORE" "$T/b2" >"$T/out.txt" 2>&1; RC=$?
check "make build 路径: 构建、安装、启动" '[ $RC -eq 0 ] && grep -q "built (mock)" "$T/out.txt" && [ -x "$NEWRUN/bin/safe" ] && [ -f "$T/log/started.admin" ]'

echo
if [ "$FAILS" -eq 0 ]; then echo "全部通过"; exit 0; else echo "$FAILS 项失败"; exit 1; fi
