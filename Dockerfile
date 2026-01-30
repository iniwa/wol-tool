# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# SQLiteのビルド(CGO)に必要なツールをインストール
RUN apk add --no-cache build-base

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm64

WORKDIR /app

# 全ファイルをコピー（main.go, index.html, go.mod等）
COPY . .

# 依存関係を整理してgo.sumを修復し、ビルドを実行
RUN go mod tidy && \
    go build -ldflags="-s -w" -o server .

# ステージ2: 実行環境
FROM alpine:latest
# SQLiteとGoバイナリの実行に必要なライブラリを追加
RUN apk add --no-cache ca-certificates libc6-compat

WORKDIR /app
# ビルド済みバイナリを適切な場所にコピー
COPY --from=builder /app/server ./server

# データディレクトリ作成
RUN mkdir -p /app/data
WORKDIR /app/data

# サーバーを実行
CMD ["/app/server"]