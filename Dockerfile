# Build Stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o main .

# Final Stage
FROM scratch
WORKDIR /app
COPY --from=builder /app/main .
# 修正箇所: templatesフォルダではなく、ルートにあるindex.htmlをコピーします
COPY --from=builder /app/index.html .

EXPOSE 8090
CMD ["/app/main"]