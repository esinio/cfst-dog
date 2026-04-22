# --- 第一阶段：编译阶段 ---
FROM golang:alpine AS builder

# 安装必要的构建依赖
RUN apk add --no-cache git gcc musl-dev

# 设置工作目录
WORKDIR /build

# 克隆源代码 (使用 --depth 1 减少克隆体积)
RUN git clone --depth 1 https://github.com/XIU2/CloudflareSpeedTest.git .

ENV GOPROXY=https://goproxy.cn,direct

# 编译程序
# CGO_ENABLED=0 确保静态链接，提高在 alpine 上的兼容性
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o cfst main.go


# --- 第二阶段：运行阶段 ---
FROM alpine:latest

# 安装 cron 服务的依赖（Alpine 自带 busybox-extras 包含 crond）
RUN apk add --no-cache ca-certificates tzdata

# 设置工作目录
WORKDIR /usr/local/bin/cfst/

# 从编译器镜像中拷贝编译好的二进制文件和必要的 IP 库
COPY --from=builder /build/cfst .
COPY --from=builder /build/ip.txt .
COPY --from=builder /build/ipv6.txt .

# 赋予执行权限
RUN chmod +x /usr/local/bin/cfst/cfst && mkdir /data

# 配置定时任务 (每两小时运行一次)
# 注意：这里将输出重定向到 stdout 以便在 docker logs 中查看
# 格式：分 时 日 月 周 命令
RUN echo "0 */2 * * * cd /usr/local/bin/cfst && ./cfst -tl 200 -dn 20 -o /data/result.csv >> /var/log/cfst.log 2>&1" > /etc/crontabs/root

# 启动 cron 服务并保持前台运行
# -f: 前台运行
# -l 2: 日志级别
CMD ["crond", "-f", "-l", "2"]
