# playbook: コード検索（semble 優先）

コードを探すときの方針と、semble のセットアップ・使い方。常時遵守の検索ルールは [CLAUDE.md](../CLAUDE.md#コード検索semble-優先) を参照。

## 方針

1. **まず semble でセマンティック検索する**。自然言語（「請求書を入金済みにする状態遷移」）でもコード片でも投げられる。
2. semble で当たりを付けてから、必要な箇所だけ Read で精読する。
3. **Grep / Read の総当たりは禁止に近い**。総当たりは semble で対象を絞った後の確認用途に限る。
4. 影響範囲の広い調査・レビューは、サブエージェントに委譲して**要約だけ**を受け取る（[CLAUDE.md](../CLAUDE.md#サブエージェント委譲)）。

## semble とは

[MinishLab/semble](https://github.com/MinishLab/semble) — エージェント向けの高速・高精度コード検索。tree-sitter でコード単位にチャンク化し、Model2Vec 埋め込み（意味）と BM25（字句）を Reciprocal Rank Fusion で統合する。CPU のみで動き、API キー・GPU 不要。grep+read に比べ約98%少ないトークンで必要なスニペットを返す。

MCP サーバとして次の2ツールを公開する：

- **`search`** — 自然言語またはコードクエリでコードベースを検索。`repo` にローカルパスまたは `https://` の git URL を渡す。
- **`find_related`** — ファイルパスと行番号を渡すと、その箇所に意味的に近いチャンクを返す。

## セットアップ（未インストール時）

このリポジトリの [.mcp.json](../.mcp.json) に semble サーバを登録済み。`uvx` 経由で起動するため、ローカルに [uv](https://docs.astral.sh/uv/) があれば追加インストール不要で立ち上がる。

uv が無い、または明示的に入れたい場合：

```sh
# uv を入れる（未導入なら）
curl -LsSf https://astral.sh/uv/install.sh | sh

# semble を入れて対話インストーラで MCP 連携を有効化する
uv tool install semble
semble install
```

`.mcp.json` の登録内容（参考）：

```json
{
  "mcpServers": {
    "semble": {
      "command": "uvx",
      "args": ["--from", "semble[mcp]", "semble"]
    }
  }
}
```

> ローカルパスはファイル変更時に自動で再インデックスされる。docs/config も含めて索引したい場合は args に `"--content", "all"` を足す。
