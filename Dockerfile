# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# CGOを有効にするために必要なツールをインストール
RUN apk add --no-cache build-base

# ラズパイ4 (64bit) 向けの設定
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm64

WORKDIR /app

# ファイルのコピー
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# アプリケーションのビルド
RUN go build -ldflags="-s -w" -o /server .

# ステージ2: 実行環境
FROM alpine:latest
# SQLiteの動作に必要なライブラリをインストール
RUN apk add --no-cache ca-certificates libc6-compat

WORKDIR /app
COPY --from=builder /server .

# データディレクトリ作成
RUN mkdir -p /app/data
WORKDIR /app/data

# サーバーを実行
CMD ["/app/server"]