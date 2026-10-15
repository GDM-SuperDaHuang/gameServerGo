#
# STAGE 1: prepare
#
# 依赖缓存层：只在 go.mod/go.sum 变化时失效
#
FROM golang:1.25-alpine AS prepare

WORKDIR /app

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOOS=linux

COPY go.mod .
COPY go.sum .
RUN go mod download

#
# STAGE 2: build
#
# 编译源码（构建上下文必须是仓库根目录，
# 如：docker build -f app/gate/Dockerfile .）
#
FROM prepare AS builder

ARG VERSION=dev
ARG TARGETARCH=amd64

COPY . .
# -s 去掉符号表，-w 去掉调试信息，-X 注入版本号（main.Version）
RUN GOARCH=${TARGETARCH} go build -ldflags="-s -w -X main.Version=${VERSION}" -o gate ./app/gate

#
# STAGE 3: run
#
# 只保留二进制文件，丢弃编译环境
#
FROM alpine:3.24

WORKDIR /app
# base 与 apk 源版本必须一致（都是 v3.24），否则 apk 解析失败
RUN echo "https://mirrors.aliyun.com/alpine/v3.24/main/" > /etc/apk/repositories \
    && echo "https://mirrors.aliyun.com/alpine/v3.24/community/" >> /etc/apk/repositories \
    # ca-certificates 必须保留：gate 要连 redis/mongo/etcd/微信/抖音，全是 TLS
    && apk add --no-cache ca-certificates tzdata \
    # UTC+0
    && cp /usr/share/zoneinfo/UTC /etc/localtime \
    && echo UTC > /etc/timezone \
    # UTC+8
    # && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    # && echo Asia/Shanghai > /etc/timezone \
    && apk del tzdata

COPY --from=builder /app/gate /app/gate

# 代码里读的是 ./logs/ 即 /app/logs（compose 挂载的也是 /app/logs），不是 /logs
RUN mkdir -p /app/logs

# WS 7001 / pprof 7051（见 config/gate.toml [gate-1]，仅文档作用）
EXPOSE 7001 7051

CMD ["/app/gate"]
