#
# STAGE 1: prepare
#
# 获取基础环境，不包含任何业务逻辑
#
# FROM 192.168.1.4:8003/mirror/golang:1.24-alpine AS prepare
# go.mod 要求 go >= 1.25.0，基础镜像必须同步升级，否则报 GOTOOLCHAIN=local 错误
FROM golang:1.25-alpine AS prepare

WORKDIR /app

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

ADD go.mod .
ADD go.sum .
RUN go mod download


#
# STAGE 2: build
#
# 编译源码
#
FROM prepare AS builder

COPY . .

RUN go build -ldflags="-s -w" -o dockerMonitor ./app/monitor
#
# STAGE 3: RUN
#
# 丢弃以上的编译环境，只使用编译出的二进制文件
#
# FROM 192.168.1.4:8003/mirror/alpine:latest
FROM alpine:3.24

WORKDIR /app
# 将时区设置为 UTC+0
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

COPY --from=builder /app/dockerMonitor  /app/dockerMonitor

RUN mkdir -p  /app/logs

CMD ["/app/dockerMonitor"]
