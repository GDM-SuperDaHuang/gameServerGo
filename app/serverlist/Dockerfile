FROM alpine:3.20

WORKDIR /app

# golang:1.24-apline 需要手动安装然后删除
# 将时区设置为 UTC+0
RUN echo "https://mirrors.aliyun.com/alpine/v3.20/main/" > /etc/apk/repositories \
    && echo "https://mirrors.aliyun.com/alpine/v3.20/community/" >> /etc/apk/repositories \
    && apk add --no-cache tzdata \
    # UTC+0
    && cp /usr/share/zoneinfo/UTC /etc/localtime \
    && echo UTC > /etc/timezone \
    # UTC+8
    # && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && apk del tzdata

COPY bin/serverlist /app/serverlist

RUN mkdir -p  /logs

CMD ["/app/serverlist"]
