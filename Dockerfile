# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# SQLiteのビルドに必要なツールをインストール
RUN apk add --no-cache build-base

WORKDIR /app

# 依存関係ファイルをコピー
COPY go.mod ./

# 依存関係の解決（ソースコードコピーの前に実行して効率化）
RUN go mod tidy && go mod download

# ソースコードをコピー
COPY . .

# 【修正】静的リンクオプションを削除し、標準ビルドに変更
# これによりビルド時間が大幅に短縮され、ハングアップを防ぎます
RUN go build -ldflags="-s -w" -o server .

# ステージ2: 実行環境
FROM alpine:latest

WORKDIR /app

# タイムゾーン情報などを追加（必須ではないですがあると便利です）
RUN apk add --no-cache ca-certificates tzdata

# ビルド成果物をコピー
COPY --from=builder /app/server ./server

# データディレクトリ作成
RUN mkdir -p /app/data

# 実行
CMD ["/app/server"]