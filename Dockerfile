# 第一阶段：构建
FROM golang:1.22-alpine AS builder

# 设置工作目录
WORKDIR /app

# 设置 Go 模块代理（可选，根据需要配置）
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

# 只复制 go.mod 和 go.sum，利用 Docker 缓存层
# 这样代码变化时不需要重新下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译优化：
# - CGO_ENABLED=0: 静态编译，不依赖系统 C 库
# - GOOS=linux: 目标操作系统
# - -ldflags="-s -w": 去掉调试符号和 DWARF 信息，减小二进制约 30%
# - -trimpath: 去除文件系统路径，提高构建可重现性
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o trading-system ./cmd/main.go

# 第二阶段：运行时
FROM alpine:3.19

# 安装必要的工具
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN addgroup -g 1001 -S appuser && \
    adduser -u 1001 -S appuser -G appuser

WORKDIR /app

# 复制编译好的二进制文件
COPY --from=builder /app/trading-system .

# 修改权限
RUN chown -R appuser:appuser /app && \
    chmod +x ./trading-system

# 切换到非 root 用户
USER appuser

EXPOSE 2112

CMD ["./trading-system"]
