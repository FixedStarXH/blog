# 博客系统 Blog-System（多用户 CMS）

> 基于 **Go + Gin + GORM + MySQL** 的前后端分离多用户博客系统。
> 支持注册投稿、审核发布、多角色权限的后台管理。
> 瑞士国际主义视觉风格，面向简历的完整工程化实践。

![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)
![Gin](https://img.shields.io/badge/Gin-v1.12-7F52FF?style=flat)
![GORM](https://img.shields.io/badge/GORM-v1.31-8C1A6A?style=flat)
![License](https://img.shields.io/badge/License-MIT-green)

---

## ✨ 功能特性

### 前台
- 首页：头条精选、文章目录表、专栏推荐、分类索引、站点统计、每日一言
- 文章列表：分类 / 标签 / 关键词 / 作者筛选、分页、按最新 / 热门排序
- 文章详情：正文渲染、阅读时长、自动目录（TOC）、上一篇 / 下一篇、相关推荐
- 评论：游客 / 登录用户均可评论、楼中楼回复、后台审核后展示
- 标签墙、时间归档、作者页、专栏页、RSS 订阅、Sitemap
- **特色**：浏览量 IP 防刷、文章点赞（IP/用户去重）、私密文章（密码解锁）、JWT 双 token 自动续期
- **AI 问答助手**：基于文章内容的智能问答（RAG），SSE 流式回复、多轮对话、一键复制、会话历史本地持久化

### 后台（RBAC 多角色）
- **角色体系**：普通用户(1) / 编辑(2) / 管理员(3)，游客(0) 不落库
- **投稿审核工作流**：草稿 → 待审核 → 发布 / 驳回（附原因，可修改重提）
- 仪表盘数据统计；文章管理（发布 / 草稿 / 置顶 / 定时 / 软删除 / 回收站）
- 用户管理（角色分配 / 禁用）、分类 / 标签 / 专栏管理
- 评论审核（通过 / 驳回 / 批量）、图片上传图库、站点设置、友情链接

### AI 智能问答（RAG）

- **索引**：文章正文切块（~500 字 + 50 字重叠）→ Embedding 向量化 → 存 `article_chunks` 表
- **检索**：问题向量化 → 与全部块做余弦相似度 → 取 top-5 最相关片段
- **生成**：命中片段拼进 Prompt → 大模型 SSE 流式回答（可溯源到具体文章）
- **降级链**：服务商无 Embedding 接口（如 DeepSeek 官方）时自动降级"全文模式"，直接把文章全文交给模型回答
- **多轮对话**：history 上下文（限条数）插在问题之前，保证追问连贯；会话历史 localStorage 持久化（上限 50 条）
- **成本控制**：每日问答次数限额（Redis 按日计数，超限当天友好提示；Redis 不可用时自动放行）

## 🏗️ 技术架构

```
Controller(HTTP)  →  Service(业务)  →  DAO(数据)  →  GORM → MySQL
                        ↘            ↘
                      Model(实体+行为)   cache(Redis 缓存层)
```

- **分层架构**：Controller / Service / DAO / Model 四层，依赖方向自上而下单向
- **AI 问答（RAG）**：切块 → Embedding → 余弦检索 → Prompt → SSE 流式回答，无 Embedding 时自动降级全文模式
- **依赖注入**：构造函数注入，Service 依赖注入清晰
- **JWT 双 token + RBAC**：access(2h) 业务鉴权 + refresh(7d) 换新，Redis 白名单轮换吊销，前端 401 自动刷新重放
- **Redis 缓存分层**：列表/详情/归档缓存 + 浏览量增量双写（30s 定时刷库 + 分布式锁）
- **软删除**：全部模型基于 `gorm.Model`，天然支持回收站
- **统一响应**：`{ code, message, data }` 前后端契约清晰

## 📂 目录结构

```
blog-system/
├── main.go              入口
├── config/              配置（YAML + Viper + 环境变量覆盖）
├── model/               数据模型 + 常量 + 建表 + 种子数据
├── dao/                 数据层（GORM 实现）
├── service/             业务层
├── controller/          HTTP 层
├── middleware/          中间件（鉴权 / 角色 / 限流 / 日志）
├── cache/               Redis 缓存层（列表 / 详情 / 浏览量 / refresh 白名单）
├── utils/               工具（响应 / JWT / 密码 / 上传）
├── cmd/                 独立工具（CSDN 批量导入 / 来源回填 / 标签修复）
├── router/              路由注册
├── web/                 SPA 前端（Vite）
├── uploads/             上传图片
├── docs/                需求 / 接口 / 数据库 / 规范 / 审核 文档
├── Dockerfile           多阶段构建（node 前端 + go 后端 + alpine 运行）
├── docker-compose.yml   一键部署（mysql + redis + app）
└── README.md
```

## 🚀 快速启动

### 方式一：Docker 一键部署（推荐）

```bash
# 需已安装 Docker
docker compose up -d --build
# 访问 http://localhost:8080（自动建表 + 种子数据 + 启动 Redis/MySQL）
```

> MySQL/Redis 容器不暴露宿主机端口，只在内网互访，与本地环境零冲突；
> 配置通过环境变量注入（`BLOG_MYSQL_HOST`、`BLOG_JWT_SECRET` 等，见 docker-compose.yml）。

### 方式二：本地运行

#### 环境要求
- Go 1.26+
- MySQL 8.0+
- Redis 7+（缓存 + 浏览量双写 + refresh token 白名单，**必需**）

#### 步骤

```bash
# 1. 克隆项目
git clone <repo-url>
cd blog-system

# 2. 配置数据库与 Redis
# 编辑 config.yaml，填入你的 MySQL / Redis 连接信息

# 3. 启动（自动建表 + 写入测试数据）
go run .

# 4. 访问
# 后端 API: http://localhost:8080/api/...
# 前台页面: http://localhost:8080/
```

#### 测试账号

| 账号 | 密码 | 角色 |
|------|------|------|
| admin | 123456 | 管理员(3) |
| editor | 123456 | 编辑(2) |
| user1 | 123456 | 普通用户(1) |

> 启动时自动创建测试数据（三角色 + 分类 + 标签 + 专栏 + 文章，含 1 篇待审核），详情见 `model/init.go`。

## 📚 文档

> 项目文档位于本地 `docs/` 目录（仅本地使用，不随仓库分发）：

| 文档 | 说明 |
|------|------|
| `docs/需求文档.md` | 功能全景需求（多用户 CMS） |
| `docs/接口设计.md` | 接口契约（公共 + 用户 + 投稿 + 后台） |
| `docs/数据库设计.md` | 12 张表结构设计 |
| `docs/项目设计规范.md` | 工程规范（分层 / 命名 / 安全 / Git） |
| `docs/开发流程.md` | 8 阶段开发路线（含 curl 验证） |
| `docs/项目审核报告.md` | 简历级质量评估 |

## 🧪 测试与质量

```bash
go vet ./...        # 静态检查
go test ./...       # 单元测试（40 用例）
gofmt -l .          # 格式检查
```

> 测试覆盖：JWT 签发与鉴权、接口限流、角色权限、RAG 切块 / 关键词 / 降级链、浏览量 IP 防刷、点赞防重、评论审核流转。测试用纯 Go sqlite 内存库，无外部依赖、自包含。

## 🔧 依赖管理

- 使用 Go Modules，依赖见 `go.mod`
- 核心依赖：Gin（Web）、GORM（ORM）、Viper（配置）、golang-jwt（JWT）、x/crypto（BCrypt）、go-redis（Redis）

## 🛡️ 安全设计

- **JWT 双 token**：access(2h) 业务鉴权 + refresh(7d) 换新，`token_type` 隔离类型，Redis 白名单轮换吊销，前端 401 自动刷新重放；登出吊销 refresh
- **接口限流**：登录/刷新/注册按 IP 限流，防暴力破解与 token 遍历
- 密码 BCrypt 哈希存储
- RBAC 鉴权（后台接口 `role>=2`，用户管理 `role=3`）
- 浏览量 IP 防刷（明细表 + 悲观锁 + Redis 增量）；点赞幂等防负
- SQL 全参数化（GORM），杜绝注入
- 评论内容转义，防 XSS；评论审核后展示
- 上传白名单（jpg/png/gif/webp）+ 随机重命名
- 敏感配置不入库（gitignore），生产走环境变量注入；AI Key 同样只走环境变量
- AI 问答每日限额（Redis 按日计数，超限友好提示，防 token 消耗失控）
- 全文模式内容截断上限 + 关键词匹配 top-3，控制单次问答 token 成本

## 📄 License

MIT
