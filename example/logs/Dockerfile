# phase 1, 构建阶段
# 使用 builder 别名
FROM golang:1.26-alpine3.23 as builder

WORKDIR /app/code

# 1. 【新增】安装 tzdata (时区数据) 和 ca-certificates (HTTPS证书，防止联网报错)
# 虽然 Alpine 基础镜像可能自带，但显式安装更保险
RUN apk add --no-cache tzdata ca-certificates

# 缓存依赖：只复制 go.mod 和 go.sum
COPY ./go.mod /app/code/go.mod
COPY ./go.sum /app/code/go.sum
# 设置代理并下载依赖。这层会在 go.mod/go.sum 不变时被缓存
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download

# 复制所有代码
COPY . /app/code
# 编译：禁用 CGO 以生成静态二进制文件
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o main ./example/logs


# phase 2, 运行阶段
FROM scratch

# 设置工作目录
WORKDIR /app

# 2. 【关键修复】从 builder 阶段复制时区数据到 scratch 镜像
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
# 3. 【建议】复制 CA 证书，防止将来访问 HTTPS (如微信API、云服务) 时报 x509 错误
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 4. 【可选】设置默认时区环境变量
ENV TZ=Asia/Shanghai

COPY --from=builder /app/code/main /app/main

# env --------- start
ENV BIND_ADDR=""
ENV ERR_FILE=""
ENV JAEGER_AGENT_HOST=""
ENV JAEGER_AGENT_PORT=""
ENV JAEGER_COLLECTOR_ENDPOINT=""
ENV JAEGER_SERVICE_NAME=""
ENV JWT_MAILBOX_KEY=""
ENV LOG_FILE=""
ENV MYSQL_DATABASE=""
ENV MYSQL_HOST=""
ENV MYSQL_PASSWORD=""
ENV MYSQL_PORT=""
ENV MYSQL_USER=""
ENV PROMETHEUS_LISTEN_ADDR=""
ENV REDIS_ADDR=""
ENV REDIS_PWD=""
ENV SERVICE_DOC=""
ENV SERVICE_ID=""
ENV SERVICE_VERSION=""
# env --------- end

EXPOSE 8080

# 设置容器启动时运行的命令
CMD ["./main"]
