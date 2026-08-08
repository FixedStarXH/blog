# ---------- 阶段 1：构建前端（Vite → dist/） ----------
FROM node:20-alpine AS web-build
WORKDIR /app/web
# 利用层缓存：先只拷依赖清单，npm ci 命中缓存就不用重装
COPY web/package.json web/package-lock.json ./
RUN npm config set registry https://registry.npmmirror.com && npm ci
# 再拷源码构建，输出到 ../dist（vite.config.js 的 outDir）
COPY web/ ./
RUN npm run build

# ---------- 阶段 2：构建后端（Go 静态二进制） ----------
FROM golang:1.26-alpine AS go-build
WORKDIR /app
# 先只拷 go.mod/go.sum 下载依赖（层缓存）
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
# 拷全部源码构建；CGO_ENABLED=0 产出纯静态二进制，alpine 可直接运行
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o blog-system .

# ---------- 阶段 3：精简运行镜像 ----------
FROM alpine:3.20
# tzdata + TZ：DSN 里 loc=Local 依赖系统时区，否则时间全是 UTC
RUN apk add --no-cache tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
# 只带运行必需物：后端二进制 + 前端构建产物
COPY --from=go-build /app/blog-system .
COPY --from=web-build /app/dist ./dist
# 运行期挂载卷（uploads 存图片、logs 存日志，从宿主机挂进来持久化）
RUN mkdir -p uploads logs
EXPOSE 8080
# 配置全部由环境变量注入（docker-compose.yml 的 environment），无 config.yaml
CMD ["./blog-system"]
