# =========================
#   Multi-stage Builder
# =========================
FROM golang:1.24-alpine AS builder
WORKDIR /src

# 国内/慢网络必备加速（只影响构建阶段）
ENV GOTOOLCHAIN=auto \
    GOPROXY=https://goproxy.cn,https://goproxy.io,direct \
    GOSUMDB=sum.golang.google.cn \
    GODEBUG="http2client=0"

RUN set -eux; \
  for i in 1 2 3 4 5; do \
    go mod download && break; \
    echo "go mod download failed, retry $i..." >&2; \
    sleep $((i*i)); \
  done
# 为 go mod download 准备 git + ca-cert
RUN apk add --no-cache git ca-certificates && update-ca-certificates

# 先拷贝 go.mod/go.sum 以利用缓存
COPY go.mod go.sum ./

# 下载依赖（第一次慢，后面走缓存会快很多）
RUN go mod download

# 拷贝全部源码
COPY . .

# 构建目标：server | worker（默认 server）
ARG TARGET=server
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN set -eux; \
    case "$TARGET" in \
      server) PKG="./cmd/server" ;; \
      worker) PKG="./cmd/worker" ;; \
      *) echo "Unknown TARGET=$TARGET (expect server|worker)" && exit 1 ;; \
    esac; \
    go build -trimpath -ldflags "-s -w \
      -X main.version=${VERSION} \
      -X main.commit=${COMMIT_SHA} \
      -X main.buildTime=${BUILD_TIME}" \
      -o /out/main "${PKG}"; \
    mkdir -p /out/configs && cp -r ./configs/* /out/configs/ 2>/dev/null || true

# =========================
#        Runtime
# =========================
FROM alpine:3.20
WORKDIR /app

# 安装 tzdata + ca-cert + curl + redis-cli
RUN apk add --no-cache \
    tzdata \
    ca-certificates \
    curl \
    redis && \
    update-ca-certificates
RUN apk add --no-cache websocat || echo "websocat not available, skip"

COPY --from=builder /out/main     /app/main
COPY --from=builder /out/configs  /app/configs

ENV TZ=UTC \
    GIN_MODE=release

EXPOSE 38080

USER 65532:65532

ENTRYPOINT ["/app/main"]
