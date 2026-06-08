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

```
frontend/
├── index.html
├── src/
│   ├── main.tsx          # エントリポイント
│   ├── App.tsx           # 画面（契約一覧・請求/回収状況）
│   ├── format.ts         # 表示整形（金額・状態ラベル）
│   └── api/
│       ├── types.ts      # バックエンドのドメインに対応する DTO 型
│       └── client.ts     # SubscopeApi 抽象 + MockApi 実装
├── eslint.config.js
├── tsconfig*.json
└── vite.config.ts
```

## バックエンド API との接続方針（#20 decision）

バックエンドは現状 HTTP API を持たず、`backend/cmd/api` のデモ実行のみ。
そこでフロントは **`SubscopeApi` インターフェース越し**にデータを取得する設計とし、
既定実装は **`MockApi`（決定的なサンプルデータ）** とする。

バックエンドに REST/HTTP 層が追加された段階で、`HttpApi` 実装を `src/api/client.ts`
に追加して `api` の差し替えだけで切り替えられる。UI コンポーネントは API 抽象にのみ
依存しており、データ源の実装には依存しない。

DTO 型（`src/api/types.ts`）はバックエンドの `shared.Money` と各 ID・状態機械に
1:1 で対応させ、API 境界の契約をフロント側にも明示している。
