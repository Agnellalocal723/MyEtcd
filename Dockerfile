# 多阶段构建Dockerfile
# 第一阶段：构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 复制go mod文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o myetcd ./cmd/server

# 第二阶段：运行阶段
FROM alpine:latest

# 安装ca-certificates以支持HTTPS请求
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN addgroup -g 1001 -S myetcd && \
    adduser -u 1001 -S myetcd -G myetcd

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/myetcd .

# 创建数据目录
RUN mkdir -p /app/data /app/logs /app/config && \
    chown -R myetcd:myetcd /app

# 复制默认配置文件
COPY config/config.json /app/config/

# 切换到非root用户
USER myetcd

# 暴露端口
EXPOSE 2379 2380 8080

# 设置环境变量
ENV MYETCD_DATA_DIR=/app/data
ENV MYETCD_WAL_DIR=/app/data/wal
ENV MYETCD_SNAPSHOT_DIR=/app/data/snapshot
ENV MYETCD_LOG_DIR=/app/logs
ENV MYETCD_CONFIG_FILE=/app/config/config.json

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:2379/health || exit 1

# 启动命令
CMD ["./myetcd", "-config-file=/app/config/config.json", "-data-dir=/app/data", "-port=:2379"]