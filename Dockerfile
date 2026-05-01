# ========== 构建阶段 ==========
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/xboard ./cmd/server/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/xboard-scheduler ./cmd/scheduler/

# ========== 运行阶段 ==========
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=builder /build/xboard /app/xboard
COPY --from=builder /build/xboard-scheduler /app/xboard-scheduler
COPY config.example.yaml /app/config.yaml
COPY web/ /app/web/
COPY plugins-core/ /app/plugins-core/
COPY migrations/ /app/migrations/

VOLUME ["/app/storage", "/app/plugins", "/app/config.yaml"]

EXPOSE 8080

ENTRYPOINT ["/app/xboard"]
CMD ["--config", "/app/config.yaml"]
