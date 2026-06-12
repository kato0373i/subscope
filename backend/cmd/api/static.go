package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// withStaticFallback は API ハンドラの前段に静的配信を足す合成ハンドラを返す。
// /api/ と /healthz は API ハンドラへ委譲し、それ以外はビルド済みフロント（dir）を配信する。
// 実ファイルが無いパスは index.html にフォールバックする（SPA ルーティング対応）。
// これにより 1 プロセス・同一オリジンで UI と API の両方を配信できる（Docker 想定）。
func withStaticFallback(api http.Handler, dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz" {
			api.ServeHTTP(w, r)
			return
		}
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}
