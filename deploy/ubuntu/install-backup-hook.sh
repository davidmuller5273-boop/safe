#!/usr/bin/env bash
# 安装 SafeW 机器人备份钩子到 /etc/tg-bot-backup.d/safe.sh（root，755）。可重复执行。
#   cd ~/safe && git pull origin main && sudo bash deploy/ubuntu/install-backup-hook.sh
#   sudo bash deploy/ubuntu/install-backup-hook.sh --test   # 安装后试跑一次（导出到临时目录，随后删除）
# 测试用环境变量：BACKUP_HOOK_DIR（默认 /etc/tg-bot-backup.d）、SAFE_HOOK_ALLOW_NONROOT=1。
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC="$HERE/safe-backup-hook.sh"
DEST_DIR="${BACKUP_HOOK_DIR:-/etc/tg-bot-backup.d}"
DEST="$DEST_DIR/safe.sh"
RUN_TEST=0
[ "${1:-}" = "--test" ] && RUN_TEST=1

if [ "$(id -u)" -ne 0 ] && [ "${SAFE_HOOK_ALLOW_NONROOT:-}" != "1" ]; then
  echo "请用 root 运行：sudo bash deploy/ubuntu/install-backup-hook.sh" >&2
  exit 1
fi
[ -f "$SRC" ] || { echo "找不到 $SRC，请在 ~/safe 里先 git pull origin main" >&2; exit 1; }
bash -n "$SRC" || { echo "钩子脚本语法检查失败，已中止" >&2; exit 1; }

if [ ! -d "$DEST_DIR" ]; then
  echo "提示：$DEST_DIR 还不存在（tg-directory-bot 的备份脚本可能尚未安装），先创建；安装备份脚本后会自动执行本钩子。"
  mkdir -p "$DEST_DIR"
  chmod 755 "$DEST_DIR"
fi

tmp="$(mktemp "$DEST_DIR/.safe.sh.XXXXXX")"
trap 'rm -f -- "$tmp"' EXIT
cat -- "$SRC" >"$tmp"
chmod 755 "$tmp"
if [ "$(id -u)" -eq 0 ]; then chown root:root "$tmp"; fi
mv -f -- "$tmp" "$DEST"
trap - EXIT
echo "已安装: $DEST（$(stat -c '%a %U:%G' "$DEST")）"

if ! command -v mysqldump >/dev/null 2>&1; then
  echo "警告：未找到 mysqldump，备份时会失败。请先安装：sudo apt update && sudo apt install -y mysql-client   # 或 mariadb-client"
fi

if [ "$RUN_TEST" -eq 1 ]; then
  stage="$(mktemp -d)"
  trap 'rm -rf -- "$stage"' EXIT
  echo "试跑钩子（输出到临时目录 $stage，结束后删除）…"
  rc=0
  bash "$DEST" "$stage" || rc=$?
  [ -f "$stage/safe/MANIFEST.txt" ] && { echo "---- MANIFEST ----"; cat "$stage/safe/MANIFEST.txt"; }
  echo "钩子退出码: $rc"
  exit "$rc"
fi
echo "下一步：sudo tgbot-backup 手动备份一次；恢复方法见 docs/Ubuntu部署.md「自动加密备份」→「恢复」。"
