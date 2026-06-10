/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** バックエンド REST API のベース URL。未設定なら MockApi を使う。 */
  readonly VITE_API_BASE_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
