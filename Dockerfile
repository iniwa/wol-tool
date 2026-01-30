# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# CGO（SQLite）ビルドに必要なコンパイラをインストール
RUN apk add --no-cache build-base

WORKDIR /app

# 先にgo.mod/go.sumだけをコピーして依存関係を解決（キャッシュ効率化）
COPY go.mod ./
# go.sumがない場合に生成し、依存関係をダウンロード
RUN go mod tidy && go mod download

# ソースコードをコピー
COPY . .

# 静的リンクでビルド
# -ldflags="-s -w -extldflags '-static'" により、実行時に外部ライブラリを不要にする
RUN CGO_ENABLED=1 GOOS=linux go build -tags musl -ldflags="-s -w -extldflags '-static'" -o server .

# ステージ2: 実行環境
FROM alpine:latest

# 静的リンク済みなのでライブラリ追加は不要だが、デバッグ用に基本ツールはあっても良い
WORKDIR /app

# ビルド済みバイナリをコピー
COPY --from=builder /app/server ./server

# データ保存用ディレクトリ
RUN mkdir -p /app/data

# 実行
CMD ["/app/server"]