# syntax=docker/dockerfile:1

# ============================================================
# subscope — フロント + バックエンドを 1 イメージにまとめる
# マルチステージビルド。最終イメージは Go バイナリ 1 つで
# REST API（internal/platform/httpapi）と静的フロントを同一オリジン配信する。
# 外部依存・認証情報は含まない。
# ============================================================

# --- Stage 1: フロント（React + Vite）をビルド ---
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
# 依存だけ先に入れてキャッシュを効かせる
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# 同一オリジン配信のため API ベース URL を空文字にする（相対パス /api/... で HttpApi を使う）。
ENV VITE_API_BASE_URL=""
RUN npm run build

# --- Stage 2: バックエンド（Go）をビルド ---
# 外部依存は無く（標準ライブラリ + internal のみ）go.sum は存在しない。
FROM golang:1.23-alpine AS backend
WORKDIR /src/backend
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

# --- Stage 3: 実行イメージ（最小・非 root）---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /out/api /app/api
COPY --from=frontend /app/frontend/dist /app/web

# SUBSCOPE_ADDR=待受アドレス, STATIC_DIR=フロント配信ディレクトリ（cmd/api が解釈）。
ENV SUBSCOPE_ADDR=":8080"
ENV STATIC_DIR="/app/web"
EXPOSE 8080

# 非 root で実行する（distroless:nonroot の既定ユーザーを明示し、CI でも検証可能にする）。
USER nonroot:nonroot

ENTRYPOINT ["/app/api"]
