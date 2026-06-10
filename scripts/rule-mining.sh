#!/usr/bin/env bash
# Claude Code セッションログから「ルール化すべきパターン」を抽出する。
# 本体は同ディレクトリの rule-mining.py（Python 3 標準ライブラリのみ）。
#
# 使い方:
#   ./scripts/rule-mining.sh                    # 全プロジェクト・直近30日 → rule-mining-output.md
#   ./scripts/rule-mining.sh --days 7           # 直近7日
#   ./scripts/rule-mining.sh --project subscope # プロジェクト絞り込み
set -euo pipefail
exec python3 "$(dirname "$0")/rule-mining.py" "$@"
