#!/usr/bin/env bash
# 备份钩子测试（使用假的 mysqldump，不连接任何真实数据库）：bash deploy/ubuntu/test-backup-hook.sh
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK="$HERE/safe-backup-hook.sh"
INSTALL="$HERE/install-backup-hook.sh"
T="$(mktemp -d)"; trap 'rm -rf "$T"' EXIT
FAILS=0
ok()   { echo "ok   - $1"; }
fail() { echo "FAIL - $1"; FAILS=$((FAILS+1)); }
check() { if eval "$2"; then ok "$1"; else fail "$1  [$2]"; fi; }

# ---- 假 mysqldump ----
mkdir -p "$T/bin"
cat >"$T/bin/mysqldump" <<'MOCK'
#!/usr/bin/env bash
if [ "${1:-}" = "--help" ]; then echo "  --no-tablespaces"; echo "  --column-statistics"; exit 0; fi
f=""
for a in "$@"; do case "$a" in --defaults-extra-file=*) f="${a#*=}";; esac; done
cp "$f" "$MOCK_LOG/defaults.copy"; stat -c %a "$f" >"$MOCK_LOG/defaults.mode"; echo "$f" >"$MOCK_LOG/defaults.path"
echo "$*" >"$MOCK_LOG/args"
if [ -n "${MOCK_FAIL:-}" ]; then echo "mysqldump: Got error: 1045: Access denied" >&2; exit 2; fi
echo "-- MySQL dump"; echo "CREATE TABLE t (id int);"
[ -z "${MOCK_INCOMPLETE:-}" ] && echo "-- Dump completed on 2026-01-01 00:00:00"
exit 0
MOCK
chmod +x "$T/bin/mysqldump"

mkrun() { # 目录
  local r="$1"
  mkdir -p "$r/config" "$r/bin" "$r/logs" "$r/data/sub"
  cat >"$r/config/app.yml" <<'Y'
mysql:
  host: yaml-host   # 注释
  port: 3307
  user: "yuser"
  password: "yaml#Pass\"1"
  database: safedb
redis:
  password: "redis-should-not-matter"
Y
  printf 'MYSQL_PASSWORD=envFilePass99\nexport MYSQL_HOST=envhost\nJWT_SECRET=jwt-secret-xyz\n' >"$r/.env"
  chmod 600 "$r/.env"
  echo 'SAFEW=tok' >"$r/.env.local"
  echo '{"state":1}' >"$r/data/state.json"
  echo 'nested ok' >"$r/data/sub/list.txt"
  echo 'bot token = 123:ABC' >"$r/data/notes.txt"
  echo 'x' >"$r/logs/a.log"; echo 'x' >"$r/run.log"; echo 'x' >"$r/admin.log.1"
  printf '\x7fELF' >"$r/bin/safe"; cp "$r/bin/safe" "$r/safe"; chmod +x "$r/safe" "$r/bin/safe"
  echo 'k' >"$r/server.pem"
  head -c 6000000 /dev/zero >"$r/data/big.bin"
  ln -s "$r/.env" "$r/data/link"
}

run_hook() { # RUN_DIR STAGE [extra env…]  → 输出到 $T/out.txt，返回码在 $RC
  local r="$1" s="$2"; shift 2
  mkdir -p "$s" "$T/log"
  env -i PATH="$T/bin:$PATH" HOME="$T" TMPDIR="$T/tmp" SAFE_RUN_DIR="$r" MYSQLDUMP="$T/bin/mysqldump" \
      MOCK_LOG="$T/log" SAFE_PIDS="" "$@" bash "$HOOK" "$s" >"$T/out.txt" 2>&1
  RC=$?
}
mkdir -p "$T/tmp"

# ===== 1. 正常路径：.env 覆盖 yaml =====
R1="$T/run1"; mkrun "$R1"; S1="$T/stage1"
run_hook "$R1" "$S1"
check "退出码 0" '[ $RC -eq 0 ]'
check "safe.sql 存在且完整" 'grep -q "Dump completed" "$S1/safe/safe.sql"'
check "没有残留 .partial" '[ ! -e "$S1/safe/safe.sql.partial" ]'
check ".env 优先于 yaml: 密码来自 .env" 'grep -q "^password=\"envFilePass99\"" "$T/log/defaults.copy"'
check ".env 里的 MYSQL_HOST(export 形式) 生效" 'grep -q "^host=\"envhost\"" "$T/log/defaults.copy"'
check "yaml 的 port/user 生效" 'grep -q "^port=3307" "$T/log/defaults.copy" && grep -q "^user=\"yuser\"" "$T/log/defaults.copy"'
check "mysqldump 参数含 single-transaction/routines/triggers" 'grep -q -- "--single-transaction" "$T/log/args" && grep -q -- "--routines" "$T/log/args" && grep -q -- "--triggers" "$T/log/args"'
check "参数里没有密码" '! grep -q "envFilePass99" "$T/log/args"'
check "凭据文件权限 600" '[ "$(cat "$T/log/defaults.mode")" = "600" ]'
check "凭据文件已删除" '[ ! -e "$(cat "$T/log/defaults.path")" ]'
check "tmp 目录无残留" '[ -z "$(ls -A "$T/tmp")" ]'
check "输出中没有出现任何密码/令牌" '! grep -qE "envFilePass99|yaml#Pass|jwt-secret-xyz|123:ABC" "$T/out.txt"'
check "MANIFEST 没有密码" '! grep -qE "envFilePass99|yaml#Pass|jwt-secret-xyz|123:ABC" "$S1/safe/MANIFEST.txt"'
check "safe.sql 权限 600" '[ "$(stat -c %a "$S1/safe/safe.sql")" = "600" ]'
check "MANIFEST 记录配置来源(非空)" 'grep -q "password=.*/.env" "$S1/safe/MANIFEST.txt" && grep -q "host=.*/.env" "$S1/safe/MANIFEST.txt" && grep -q "port=.*app.yml" "$S1/safe/MANIFEST.txt"'
# 配置/密钥文件：按要求备份，权限 600
check ".env 已备份到 secrets/ 且内容一致" 'cmp -s "$R1/.env" "$S1/safe/secrets/.env"'
check "config/app.yml 已备份到 secrets/config/" 'cmp -s "$R1/config/app.yml" "$S1/safe/secrets/config/app.yml"'
check ".env.local 已备份" '[ -f "$S1/safe/secrets/.env.local" ]'
check "secrets 下文件权限 600" '[ -z "$(find "$S1/safe/secrets" -type f ! -perm 600)" ]'
check "secrets 目录权限 700" '[ -z "$(find "$S1/safe/secrets" -type d ! -perm 700)" ]'
check "MANIFEST 列出 secrets 文件名" 'grep -q "/.env" "$S1/safe/MANIFEST.txt" && grep -q "config/app.yml" "$S1/safe/MANIFEST.txt"'
# 数据文件
check "state.json 已复制到 files/" '[ -f "$S1/safe/files/data/state.json" ]'
check "子目录文件已复制" '[ -f "$S1/safe/files/data/sub/list.txt" ]'
check "files/ 里没有 .env / yml" '[ -z "$(find "$S1/safe/files" -name ".env*" -o -name "*.yml")" ]'
check "含 token 字样的文件被跳过" '[ ! -e "$S1/safe/files/data/notes.txt" ] && grep -q "notes.txt" "$S1/safe/MANIFEST.txt"'
check "日志被跳过" '[ ! -e "$S1/safe/files/run.log" ] && [ ! -e "$S1/safe/files/logs" ]'
check "bin/ 与可执行文件被跳过" '[ ! -e "$S1/safe/files/bin" ] && [ ! -e "$S1/safe/files/safe" ]'
check "pem 与超大文件被跳过" '[ ! -e "$S1/safe/files/server.pem" ] && [ ! -e "$S1/safe/files/data/big.bin" ]'
check "符号链接被跳过" '[ ! -e "$S1/safe/files/data/link" ]'
check "整个暂存目录里 files/ 外不再出现 JWT 明文以外的意外位置" '[ -z "$(grep -rl "jwt-secret-xyz" "$S1" | grep -v "/safe/secrets/")" ]'

# ===== 2. 运行中进程环境优先 =====
R2="$T/run2"; mkrun "$R2"; mkdir -p "$T/proc/4242"
printf 'PATH=/bin\0MYSQL_PASSWORD=procPass42\0MYSQL_USER=procuser\0' >"$T/proc/4242/environ"
run_hook "$R2" "$T/stage2" PROC_ROOT="$T/proc" SAFE_PIDS="4242"
check "进程环境优先于 .env/yaml" 'grep -q "^password=\"procPass42\"" "$T/log/defaults.copy" && grep -q "^user=\"procuser\"" "$T/log/defaults.copy"'
check "进程环境密码不外泄" '! grep -rq "procPass42" "$T/out.txt" "$T/stage2/safe/MANIFEST.txt"'

# ===== 3. 只有 yaml：引号、# 和转义 =====
R3="$T/run3"; mkrun "$R3"; rm -f "$R3/.env" "$R3/.env.local"
run_hook "$R3" "$T/stage3"
check "yaml 带引号/#/转义的密码解析正确" 'grep -qF "password=\"yaml#Pass\\\"1\"" "$T/log/defaults.copy"'
check "yaml host（行尾注释被去掉）" 'grep -q "^host=\"yaml-host\"" "$T/log/defaults.copy"'

# ===== 4. CONFIG_FILE 指向运行目录之外 =====
R4="$T/run4"; mkrun "$R4"; rm -f "$R4/.env" "$R4/.env.local"; mkdir -p "$T/etc"
printf 'mysql:\n  password: extPass\n  database: extdb\n' >"$T/etc/app.yml"
echo "CONFIG_FILE=$T/etc/app.yml" >"$R4/.env"
run_hook "$R4" "$T/stage4"
check "CONFIG_FILE 外部路径生效" 'grep -q "^password=\"extPass\"" "$T/log/defaults.copy" && grep -q "extdb" "$T/log/args"'
check "外部配置文件备份到 secrets/_abs/" '[ -f "$T/stage4/safe/secrets/_abs${T}/etc/app.yml" ]'

# ===== 5. 缺少 mysqldump =====
R5="$T/run5"; mkrun "$R5"
run_hook "$R5" "$T/stage5" MYSQLDUMP="no-such-mysqldump-xyz"
check "缺少 mysqldump: 退出码 3" '[ $RC -eq 3 ]'
check "给出中文提示与 apt 命令" 'grep -q "未找到 mysqldump" "$T/out.txt" && grep -q "sudo apt update && sudo apt install -y mysql-client" "$T/out.txt"'

# ===== 6. mysqldump 失败 / 导出不完整 =====
run_hook "$R5" "$T/stage6" MOCK_FAIL=1
check "mysqldump 失败: 非 0 且无 safe.sql / partial" '[ $RC -ne 0 ] && [ ! -e "$T/stage6/safe/safe.sql" ] && [ ! -e "$T/stage6/safe/safe.sql.partial" ]'
check "失败信息可见但不含密码" 'grep -q "Access denied" "$T/out.txt" && ! grep -q "envFilePass99" "$T/out.txt"'
check "失败后凭据文件也被删除" '[ ! -e "$(cat "$T/log/defaults.path")" ] && [ -z "$(ls -A "$T/tmp")" ]'
run_hook "$R5" "$T/stage7" MOCK_INCOMPLETE=1
check "导出不完整被丢弃" '[ $RC -ne 0 ] && [ ! -e "$T/stage7/safe/safe.sql" ]'

# ===== 7. 参数检查 =====
bash "$HOOK" >/dev/null 2>&1; check "缺少暂存目录参数 → 非 0" '[ $? -ne 0 ]'
bash "$HOOK" "$T/does-not-exist" >/dev/null 2>&1; check "暂存目录不存在 → 非 0" '[ $? -ne 0 ]'

# ===== 8. 安装脚本：幂等、755 =====
D="$T/hookdir/tg-bot-backup.d"
for i in 1 2; do
  BACKUP_HOOK_DIR="$D" SAFE_HOOK_ALLOW_NONROOT=1 bash "$INSTALL" >"$T/inst.txt" 2>&1; RC=$?
  check "安装第 $i 次成功" '[ $RC -eq 0 ]'
done
check "safe.sh 已安装且内容一致、权限 755" 'cmp -s "$HOOK" "$D/safe.sh" && [ "$(stat -c %a "$D/safe.sh")" = "755" ]'
check "目录里没有残留临时文件" '[ "$(ls -A "$D")" = "safe.sh" ]'
R8="$T/run8"; mkrun "$R8"
env PATH="$T/bin:$PATH" TMPDIR="$T/tmp" SAFE_RUN_DIR="$R8" MYSQLDUMP="$T/bin/mysqldump" MOCK_LOG="$T/log" SAFE_PIDS="" \
  BACKUP_HOOK_DIR="$D" SAFE_HOOK_ALLOW_NONROOT=1 bash "$INSTALL" --test >"$T/inst2.txt" 2>&1; RC=$?
check "--test 试跑成功并打印清单" '[ $RC -eq 0 ] && grep -q "MANIFEST" "$T/inst2.txt" && grep -q "钩子退出码: 0" "$T/inst2.txt"'
check "--test 输出不含密码" '! grep -qE "envFilePass99|jwt-secret-xyz" "$T/inst2.txt"'

echo
if [ "$FAILS" -eq 0 ]; then echo "全部通过"; exit 0; else echo "$FAILS 项失败"; exit 1; fi
