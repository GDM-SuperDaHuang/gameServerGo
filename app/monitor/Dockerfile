#
# STAGE 1: prepare
#
# 获取基础环境，不包含任何业务逻辑
#
# FROM 192.168.1.4:8003/mirror/golang:1.24-alpine AS prepare
FROM golang:1.24-alpine AS prepare

WORKDIR /app

ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

ADD go.mod .
ADD go.sum .
RUN go mod download && go mod tidy && go mod verify && go mod vendor


#
# STAGE 2: build
#
# 编译源码
#
FROM prepare AS builder

COPY . .

RUN go build -ldflags="-s -w" -o dockerMonitor ./service/app/monitor
#
# STAGE 3: RUN
#
# 丢弃以上的编译环境，只使用编译出的二进制文件
#
# FROM 192.168.1.4:8003/mirror/alpine:latest
FROM alpine:latest

WORKDIR /app
# 将时区设置为 UTC+0
RUN echo "https://mirrors.aliyun.com/alpine/v3.20/main/" > /etc/apk/repositories \
    && echo "https://mirrors.aliyun.com/alpine/v3.20/community/" >> /etc/apk/repositories \
    && apk add --no-cache tzdata \
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
