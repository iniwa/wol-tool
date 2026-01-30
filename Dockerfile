# ステージ1: ビルド環境
FROM golang:1.23-alpine AS builder

# CGOを有効にし、ラズパイ4 (64bit) 向けにクロスコンパイル設定
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=arm64

WORKDIR /app

# プロジェクトのすべてのファイル (main.go, index.html, go.mod など) をコピー
COPY . .

# go.mod に基づいて依存関係をダウンロードし、go.sum を生成/検証する
# その後、アプリケーションをビルドする
RUN go mod tidy && \
    go build -ldflags="-s -w" -o /server .

# ステージ2: 実行環境 (軽量)
FROM alpine:latest
WORKDIR /app

# ビルドされたバイナリをコピー
COPY --from=builder /server .

# データベースファイルを保存するデータディレクトリを作成
# このディレクトリを永続ボリュームとしてマウントすることを想定
RUN mkdir -p /app/data
WORKDIR /app/data

# サーバーを実行 (実行ファイルは /app/server にある)
CMD ["/app/server"]
