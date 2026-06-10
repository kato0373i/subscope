#!/usr/bin/env python3
"""Claude Code セッションログから「ルール化すべきパターン」を抽出する rule-mining スクリプト。

~/.claude/projects/ 配下の JSONL セッションログを読み、以下を検出する:
  1. Claude が同じ種類のミス（ツールエラー）を繰り返している箇所
  2. ユーザーが修正・訂正を繰り返している箇所
  3. ユーザーが「〜してください」と何度も指示している内容

結果は CLAUDE.md に追記しやすい Markdown 形式で rule-mining-output.md に書き出す。

使い方:
  python3 scripts/rule-mining.py                 # 全プロジェクト・直近30日
  python3 scripts/rule-mining.py --days 7        # 直近7日分のみ
  python3 scripts/rule-mining.py --project subscope   # プロジェクト名（ディレクトリ名の部分一致）で絞り込み
  python3 scripts/rule-mining.py -o out.md       # 出力先を変更

依存: Python 3.9+ 標準ライブラリのみ。
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from difflib import SequenceMatcher
from pathlib import Path

CLAUDE_PROJECTS_DIR = Path.home() / ".claude" / "projects"

# ---------------------------------------------------------------------------
# 検出パターン
# ---------------------------------------------------------------------------

# ユーザーの修正・訂正シグナル（メッセージ冒頭〜中盤に現れる訂正表現）
CORRECTION_PATTERNS = [
    (r"違う|ちがう|違います", "「違う」と否定"),
    (r"ではなく|じゃなく", "「〜ではなく」と言い直し"),
    (r"やり直し|やりなおし", "やり直しを要求"),
    (r"修正して|直して|なおして", "修正を要求"),
    (r"戻して|元に戻", "差し戻しを要求"),
    (r"間違|まちが|誤り|誤って", "誤りを指摘"),
    (r"しないで|やめて|不要です|いらない", "やったことの取り消し・禁止"),
    (r"そうじゃな|そういうことじゃ", "意図の取り違えを指摘"),
    (r"何度も言|前にも言|さっきも言", "繰り返しの指摘（強いシグナル）"),
]

# 指示文の末尾パターン（「〜してください」系）
INSTRUCTION_RE = re.compile(
    r"[^。\n!！?？]{4,80}?(?:してください|して下さい|してね|お願いします|お願いいたします)"
)

# 指示テキストの正規化時に落とすノイズ語
INSTRUCTION_NOISE_RE = re.compile(r"[\s、。・「」『』()（）\[\]　]+")

# ユーザーメッセージとして扱わない（システム由来の）内容
SYSTEM_CONTENT_MARKERS = (
    "<system-reminder>",
    "<command-name>",
    "<local-command",
    "<task-notification>",
    "Caveat:",
)

# エラーメッセージの正規化: パス・数値・ID などの可変部分を潰してシグネチャ化する
ERROR_NORMALIZERS = [
    (re.compile(r"/[\w./~-]+"), "<path>"),
    (re.compile(r"\b[0-9a-f]{7,40}\b"), "<hash>"),
    (re.compile(r"\b\d+\b"), "<n>"),
    (re.compile(r"['\"`][^'\"`]{1,60}['\"`]"), "<str>"),
    (re.compile(r"\s+"), " "),
]


# ---------------------------------------------------------------------------
# データ構造
# ---------------------------------------------------------------------------


@dataclass
class Evidence:
    project: str
    session: str
    timestamp: str
    text: str


@dataclass
class Finding:
    key: str
    label: str
    evidences: list[Evidence] = field(default_factory=list)

    @property
    def count(self) -> int:
        return len(self.evidences)

    @property
    def sessions(self) -> set[str]:
        return {e.session for e in self.evidences}


# ---------------------------------------------------------------------------
# ログ読み込み
# ---------------------------------------------------------------------------


def iter_log_files(projects_dir: Path, project_filter: str | None):
    if not projects_dir.is_dir():
        return
    for proj_dir in sorted(projects_dir.iterdir()):
        if not proj_dir.is_dir():
            continue
        if project_filter and project_filter not in proj_dir.name:
            continue
        # サブエージェントのログ（subagents/ 配下）は対象外。トップレベルの .jsonl のみ。
        for f in sorted(proj_dir.glob("*.jsonl")):
            yield proj_dir.name, f


def parse_timestamp(s: str | None) -> datetime | None:
    if not s:
        return None
    try:
        return datetime.fromisoformat(s.replace("Z", "+00:00"))
    except ValueError:
        return None


def extract_text(content) -> str:
    """message.content（str または block 配列）からテキスト部分を取り出す。"""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for block in content:
            if isinstance(block, dict) and block.get("type") == "text":
                parts.append(block.get("text", ""))
        return "\n".join(parts)
    return ""


def is_system_content(text: str) -> bool:
    head = text.lstrip()[:80]
    return any(m in head for m in SYSTEM_CONTENT_MARKERS)


def normalize_error(msg: str) -> str:
    sig = msg.strip()[:300]
    for pattern, repl in ERROR_NORMALIZERS:
        sig = pattern.sub(repl, sig)
    return sig.strip()[:160]


def normalize_instruction(text: str) -> str:
    t = INSTRUCTION_NOISE_RE.sub("", text)
    t = re.sub(r"(してください|して下さい|してね|お願いします|お願いいたします)$", "", t)
    return t


# ---------------------------------------------------------------------------
# 解析本体
# ---------------------------------------------------------------------------


def analyze(projects_dir: Path, since: datetime, project_filter: str | None):
    error_findings: dict[str, Finding] = {}
    correction_findings: dict[str, Finding] = {}
    instructions: list[tuple[str, Evidence]] = []  # (正規化文, 証拠)
    sessions_seen: set[str] = set()
    messages_seen = 0
    # フォーク/再開されたセッションは履歴を別ファイルに複製するため、uuid で重複排除する
    seen_uuids: set[str] = set()

    for project, path in iter_log_files(projects_dir, project_filter):
        session = path.stem
        try:
            lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
        except OSError as e:
            print(f"warn: {path} を読めません: {e}", file=sys.stderr)
            continue

        for line in lines:
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            if entry.get("isSidechain"):
                continue
            # compact 時の要約はユーザー発言ではないので除外
            if entry.get("isCompactSummary") or entry.get("isVisibleInTranscriptOnly"):
                continue
            uuid = entry.get("uuid")
            if uuid:
                if uuid in seen_uuids:
                    continue
                seen_uuids.add(uuid)

            ts = parse_timestamp(entry.get("timestamp"))
            if ts and ts < since:
                continue
            ts_str = ts.astimezone().strftime("%Y-%m-%d") if ts else "?"

            if entry.get("type") != "user":
                continue
            content = entry.get("message", {}).get("content")

            # --- 1. ツールエラー（Claude のミス） ---
            if isinstance(content, list):
                for block in content:
                    if (
                        isinstance(block, dict)
                        and block.get("type") == "tool_result"
                        and block.get("is_error")
                    ):
                        raw = block.get("content")
                        if isinstance(raw, list):
                            raw = extract_text(raw)
                        if not isinstance(raw, str) or not raw.strip():
                            continue
                        sig = normalize_error(raw)
                        finding = error_findings.setdefault(
                            sig, Finding(key=sig, label=sig)
                        )
                        finding.evidences.append(
                            Evidence(project, session, ts_str, raw.strip()[:200])
                        )

            # --- ユーザーが実際に打ったテキスト ---
            text = extract_text(content)
            if not text.strip() or is_system_content(text):
                continue
            sessions_seen.add(session)
            messages_seen += 1

            # --- 2. 修正・訂正シグナル ---
            for pattern, label in CORRECTION_PATTERNS:
                if re.search(pattern, text):
                    finding = correction_findings.setdefault(
                        label, Finding(key=pattern, label=label)
                    )
                    finding.evidences.append(
                        Evidence(project, session, ts_str, text.strip()[:200])
                    )
                    break  # 1メッセージにつき1カウント

            # --- 3. 「〜してください」系の指示 ---
            for m in INSTRUCTION_RE.finditer(text):
                sentence = m.group(0).strip()
                norm = normalize_instruction(sentence)
                if len(norm) < 4:
                    continue
                instructions.append(
                    (norm, Evidence(project, session, ts_str, sentence[:200]))
                )

    instruction_findings = cluster_instructions(instructions)
    return error_findings, correction_findings, instruction_findings, sessions_seen, messages_seen


def cluster_instructions(
    items: list[tuple[str, Evidence]], threshold: float = 0.62
) -> list[Finding]:
    """正規化済み指示文を類似度でゆるくクラスタリングする。"""
    clusters: list[tuple[str, Finding]] = []  # (代表文, Finding)
    for norm, ev in items:
        placed = False
        for rep, finding in clusters:
            if SequenceMatcher(None, rep, norm).ratio() >= threshold:
                finding.evidences.append(ev)
                placed = True
                break
        if not placed:
            clusters.append((norm, Finding(key=norm, label=ev.text)))
            clusters[-1][1].evidences.append(ev)
    return [f for _, f in clusters]


# ---------------------------------------------------------------------------
# Markdown 出力
# ---------------------------------------------------------------------------


def render_evidence(f: Finding, limit: int = 3) -> str:
    lines = []
    for ev in f.evidences[:limit]:
        quote = ev.text.replace("\n", " ")
        lines.append(f"  - `{ev.timestamp}` [{ev.project}] {quote}")
    if f.count > limit:
        lines.append(f"  - …ほか {f.count - limit} 件")
    return "\n".join(lines)


def render_markdown(
    error_findings: dict[str, Finding],
    correction_findings: dict[str, Finding],
    instruction_findings: list[Finding],
    sessions_seen: set[str],
    messages_seen: int,
    since: datetime,
    min_count: int,
) -> str:
    now = datetime.now().strftime("%Y-%m-%d %H:%M")
    out = [
        "# Rule Mining 結果",
        "",
        f"- 生成日時: {now}",
        f"- 対象期間: {since.astimezone().strftime('%Y-%m-%d')} 以降",
        f"- 解析対象: {len(sessions_seen)} セッション / ユーザーメッセージ {messages_seen} 件",
        f"- 検出しきい値: 同一パターン {min_count} 回以上",
        "",
        "以下はルール候補です。妥当なものを CLAUDE.md にコピーして追記してください。",
        "",
    ]

    # --- 1. 繰り返しエラー ---
    out.append("## 1. Claude が繰り返しているミス（ツールエラー）")
    out.append("")
    errs = sorted(
        (f for f in error_findings.values() if f.count >= min_count),
        key=lambda f: -f.count,
    )
    if errs:
        for f in errs:
            out.append(f"### {f.count} 回 / {len(f.sessions)} セッション")
            out.append("")
            out.append(f"エラーシグネチャ: `{f.label}`")
            out.append("")
            out.append(render_evidence(f))
            out.append("")
            out.append("```markdown")
            out.append("<!-- CLAUDE.md 追記候補（原因に応じて書き換えてください） -->")
            out.append(f"- （このエラーの再発防止策をここに書く: {f.label[:60]}）")
            out.append("```")
            out.append("")
    else:
        out.append(f"{min_count} 回以上繰り返されたエラーパターンはありませんでした。")
        out.append("")

    # --- 2. 修正・訂正 ---
    out.append("## 2. ユーザーが修正・訂正を繰り返している箇所")
    out.append("")
    corrs = sorted(
        (f for f in correction_findings.values() if f.count >= min_count),
        key=lambda f: -f.count,
    )
    if corrs:
        for f in corrs:
            out.append(f"### {f.label}（{f.count} 回 / {len(f.sessions)} セッション）")
            out.append("")
            out.append(render_evidence(f, limit=5))
            out.append("")
            out.append("```markdown")
            out.append("<!-- CLAUDE.md 追記候補（訂正内容を一般化して書いてください） -->")
            out.append(f"- （頻出する訂正「{f.label}」の原因となる行動を禁止/指示するルール）")
            out.append("```")
            out.append("")
    else:
        out.append(f"{min_count} 回以上繰り返された修正・訂正パターンはありませんでした。")
        out.append("")

    # --- 3. 繰り返し指示 ---
    out.append("## 3. 繰り返されている指示（「〜してください」系）")
    out.append("")
    insts = sorted(
        (f for f in instruction_findings if f.count >= min_count),
        key=lambda f: -f.count,
    )
    if insts:
        for f in insts:
            rep = f.label.replace("\n", " ")
            out.append(f"### 「{rep}」（{f.count} 回 / {len(f.sessions)} セッション）")
            out.append("")
            out.append(render_evidence(f, limit=5))
            out.append("")
            out.append("```markdown")
            out.append("<!-- CLAUDE.md 追記候補: 毎回指示しなくて済むよう常設ルール化 -->")
            out.append(f"- {rep}")
            out.append("```")
            out.append("")
    else:
        out.append(f"{min_count} 回以上繰り返された指示はありませんでした。")
        out.append("")

    return "\n".join(out)


# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--days", type=int, default=30, help="直近N日分を解析（デフォルト30）")
    ap.add_argument(
        "--project",
        default=None,
        help="プロジェクトディレクトリ名の部分一致で絞り込み（例: subscope）",
    )
    ap.add_argument(
        "--min-count",
        type=int,
        default=2,
        help="ルール候補とみなす最低出現回数（デフォルト2）",
    )
    ap.add_argument(
        "-o",
        "--output",
        default="rule-mining-output.md",
        help="出力先 Markdown ファイル（デフォルト rule-mining-output.md）",
    )
    ap.add_argument(
        "--logs-dir",
        default=str(CLAUDE_PROJECTS_DIR),
        help="ログディレクトリ（デフォルト ~/.claude/projects）",
    )
    args = ap.parse_args()

    since = datetime.now(timezone.utc) - timedelta(days=args.days)
    errors, corrections, instructions, sessions, messages = analyze(
        Path(args.logs_dir), since, args.project
    )

    md = render_markdown(
        errors, corrections, instructions, sessions, messages, since, args.min_count
    )
    out_path = Path(args.output)
    out_path.write_text(md, encoding="utf-8")

    n_err = sum(1 for f in errors.values() if f.count >= args.min_count)
    n_corr = sum(1 for f in corrections.values() if f.count >= args.min_count)
    n_inst = sum(1 for f in instructions if f.count >= args.min_count)
    print(
        f"解析完了: {len(sessions)} セッション / {messages} メッセージ\n"
        f"ルール候補: エラー反復 {n_err} 件, 修正・訂正 {n_corr} 件, 繰り返し指示 {n_inst} 件\n"
        f"出力: {out_path}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
