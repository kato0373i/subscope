package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// errorBody は API のエラー応答形式 { "error": { "code", "message" } }。
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON は JSON 応答を書き出す。
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("[httpapi] レスポンス書き込みに失敗: %v", err)
	}
}

// writeError は規約形式のエラー応答を書き出す。
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// withCORS はフロント開発オリジンからのアクセスを許可する。
// 認証情報を伴わない読み取り/コマンド前提のため Allow-Origin: * とする（将来 #53 で絞る）。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withRecover はハンドラの panic を 500 に変換し、サーバを落とさない。
func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[httpapi] panic: %v", rec)
				writeError(w, http.StatusInternalServerError, "internal", "内部エラーが発生しました")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
