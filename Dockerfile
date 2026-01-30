# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# CGO_ENABLED=1 (SQLite) に必要なビルドツールをインストール
RUN apk add --no-cache build-base

# 環境変数の設定
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm64

WORKDIR /app

# まず依存関係ファイルだけをコピーしてキャッシュを有効化
COPY go.mod ./
# go.sum がリポジトリにない場合を考慮しつつダウンロード
RUN go mod download || go mod tidy

# ソースコード全体をコピー
COPY . .

# アプリケーションをビルド
RUN go build -ldflags="-s -w" -o /server .

# ステージ2: 実行環境
FROM alpine:latest
# SQLiteの動作に必要なライブラリをインストール
RUN apk add --no-cache ca-certificates libc6-compat

WORKDIR /app
# ビルド済みバイナリをコピー
COPY --from=builder /server .

# データディレクトリ作成
RUN mkdir -p /app/data
WORKDIR /app/data

# サーバーを実行
CMD ["/app/server"]