# =========================
#   Multi-stage Builder
# =========================
FROM golang:1.24-alpine AS builder
WORKDIR /src

# 私有依赖 / go mod download 需要
RUN apk add --no-cache git ca-certificates && update-ca-certificates

# 先拷贝 go.mod/go.sum 以利用缓存
COPY go.mod go.sum ./

# 建议：关闭自动 toolchain 下载，避免 CI 网络问题（除非你明确需要）
ENV GOTOOLCHAIN=local
RUN go mod download

# 拷贝全部源码
COPY . .

# 构建目标：server | worker（默认 worker）
ARG TARGET=worker

# 可选：版本信息（CI 可注入）
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_TIME=unknown

# 输出目录
RUN mkdir -p /out

# 编译（根据 TARGET 选择入口）
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
      -o /out/main "${PKG}";

# 可选：带上默认 configs（整个目录 copy，更稳）
RUN if [ -d "./configs" ]; then cp -a ./configs /out/configs; fi


# =========================
#        Runtime
# =========================
FROM alpine:3.20
WORKDIR /app

# 时区 & HTTPS 证书
RUN apk add --no-cache tzdata ca-certificates && update-ca-certificates

# 复制二进制与配置
COPY --from=builder /out/main /app/main
COPY --from=builder /out/configs /app/configs

# 运行时环境
ENV TZ=UTC \
    GIN_MODE=release

# 准备可写目录（很多 SDK/日志/缓存会写）
RUN mkdir -p /app/runtime /app/log && \
    addgroup -S app && adduser -S -G app app && \
    chown -R app:app /app

USER app

# 端口：按需保留
EXPOSE 8080 8082 9503 7090

ENTRYPOINT ["/app/main"]
# 给一个默认 CMD，方便 K8s 覆盖参数
CMD []
