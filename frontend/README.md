# subscope frontend

サブスクリプション業務を可視化する管理画面（React + TypeScript + Vite）。

## セットアップ

```sh
npm ci
npm run dev      # 開発サーバ
npm run build    # 型チェック（tsc -b）+ 本番ビルド
npm run lint     # ESLint
```

## 構成

```text
frontend/
├── index.html
├── src/
│   ├── main.tsx          # エントリポイント
│   ├── App.tsx           # 画面（契約一覧・請求/回収状況）
│   ├── format.ts         # 表示整形（金額・状態ラベル）
│   └── api/
│       ├── types.ts      # バックエンドのドメインに対応する DTO 型
│       └── client.ts     # SubscopeApi 抽象 + MockApi / HttpApi 実装
├── eslint.config.js
├── tsconfig*.json
└── vite.config.ts
```

## バックエンド API との接続方針（#20 / #35）

フロントは **`SubscopeApi` インターフェース越し**にデータを取得する設計で、
UI コンポーネントは API 抽象にのみ依存し、データ源の実装には依存しない。

実装は2つ：
- **`MockApi`** — 決定的なサンプルデータ（既定。バックエンド未起動でも動く）。
- **`HttpApi`** — バックエンドの REST API（`internal/platform/httpapi`, #35）に接続。

`api` の選択は環境変数で行う：`VITE_API_BASE_URL` が設定されていれば `HttpApi`、
未設定なら `MockApi`。例：

```sh
VITE_API_BASE_URL=http://localhost:8080 npm run dev
```

DTO 型（`src/api/types.ts`）はバックエンドの `shared.Money` と各 ID・状態機械に
1:1 で対応させ、API 境界の契約をフロント側にも明示している。
