# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# CGO (SQLite) に必要なビルドツールをインストール
RUN apk add --no-cache build-base

ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm64

WORKDIR /app

# ソースコードをすべてコピー
COPY . .

# 依存関係を整理し、go.sumを強制的に生成・更新してからビルド
RUN go mod tidy && \
    go build -ldflags="-s -w" -o /server .

# ステージ2: 実行環境
FROM alpine:latest
RUN apk add --no-cache ca-certificates libc6-compat

WORKDIR /app
COPY --from=builder /server .

# データディレクトリ作成
RUN mkdir -p /app/data
WORKDIR /app/data

# サーバーを実行
CMD ["/app/server"]