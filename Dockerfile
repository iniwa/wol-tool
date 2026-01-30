# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# ビルドに必要なツールをインストール
RUN apk add --no-cache build-base

WORKDIR /app

# go.modなどをコピー
COPY go.mod ./

# go.sumがなくても自動生成して依存関係を解決
RUN go mod tidy && go mod download

# ソースコードをコピー
COPY . .

# 静的リンクでビルド
# -extldflags "-static" でCライブラリ依存をなくす
RUN CGO_ENABLED=1 GOOS=linux go build -tags musl -ldflags="-s -w -extldflags '-static'" -o server .

# ステージ2: 実行環境
FROM alpine:latest

WORKDIR /app

# ビルド成果物をコピー
COPY --from=builder /app/server ./server

# データディレクトリ
RUN mkdir -p /app/data

# 実行
CMD ["/app/server"]