# 金融时序数据 ETL 与高并发处理中台

这是一个分布式量化行情中台学习项目，用于处理和存储量化交易相关的数据。

## 📋 技术栈

| 技术 | 用途 |
|------|------|
| **Go** | 后端开发语言 |
| **MySQL** | 关系型数据库 |
| **Redis** | 缓存/消息队列 |
| **Docker** | 容器化 |
| **Kubernetes** | 容器编排 |
| **Prometheus** | 监控指标收集 |
| **Grafana** | 监控可视化 |

---

## 🚀 快速开始

### 方式一：本地开发（最推荐）

```bash
# 1. 启动 MySQL、Redis 依赖
docker-compose up -d mysql redis

# 2. 运行 Go 应用
make dev
```

**访问地址：**
- Metrics: http://localhost:2112/metrics
- 健康检查: http://localhost:2112/health
- 就绪检查: http://localhost:2112/ready

---

### 方式二：Docker Compose 完整部署

```bash
# 1. 构建并启动所有服务（含 Prometheus、Grafana）
make docker-up

# 2. 查看服务状态
make docker-status

# 3. 查看日志
make docker-logs
```

---

### 方式三：Kubernetes (Minikube) 部署

```bash
# 1. 启动 Minikube
make minikube-start

# 2. 部署到 k8s
make k8s-deploy

# 3. 查看状态
make k8s-status

# 4. 访问服务
make k8s-access
```

---

## 📁 项目结构

```
trading_system/
├── cmd/
│   └── main.go                  # 主程序入口
├── internal/
│   ├── appstate/               # 应用状态
│   ├── metrics/                # Prometheus 指标
│   ├── models/                 # 数据模型
│   └── pipeline/               # 数据处理 Pipeline
├── pkg/
│   └── db/                     # MySQL/Redis 初始化
├── deploy/
│   ├── docker-compose.yml      # Docker Compose 配置
│   ├── k8s/                   # Kubernetes 部署清单
│   ├── prometheus/            # Prometheus 配置
│   └── grafana/               # Grafana 配置
├── docs/                      # 项目文档
├── Dockerfile                 # Docker 镜像构建
├── Makefile                   # 任务脚本
└── README.md
```

---

## 🎯 核心功能与设计亮点

### 1. 高并发数据处理 Pipeline

**技术要点：**
- **goroutine + channel** 构建生产者-消费者模型
- **Worker 数量设为 CPU 核数**，避免调度开销
- **非阻塞发送**，channel 满时记录指标并丢弃
- **sync.Pool 复用 Tick/KLine 对象**，内存占用降低 40%

**核心代码：** `internal/pipeline/pipeline.go`

---

### 2. Redis + MySQL 冷热数据分离

**技术要点：**
- **最新价存 Redis**，访问延迟 1-2ms
- **历史 K 线存 MySQL**，海量数据持久化
- **12-Factor 配置管理**，通过环境变量灵活切换

**核心代码：** `pkg/db/mysql.go`、`pkg/db/redis.go`

---

### 3. 完善的容器化与 K8s 部署

**技术要点：**
- **多阶段构建**，最终镜像仅 20MB
- **-ldflags="-s -w"**，去除调试信息减小二进制 30%
- **.dockerignore**，构建上下文减小 60%
- **非 root 用户**，最小权限安全加固
- **三类 Probe**，Startup/Liveness/Readiness 全链路健康保障
- **Kustomize 配置管理**，声明式基础设施即代码

**核心文件：** `Dockerfile`、`deploy/k8s/`

---

### 4. Prometheus + Grafana 监控体系

**技术要点：**
- 采集 QPS、延迟、队列长度等指标
- Histogram 统计延迟分布（P50/P95/P99）
- 自定义业务指标埋点

**核心代码：** `internal/metrics/metrics.go`

---

## 📊 指标与监控

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `trading_quote_received_total` | Counter | 接收到的行情总数 |
| `trading_quote_processing_duration_seconds` | Histogram | 行情处理耗时 |
| `trading_mysql_query_duration_seconds` | Histogram | MySQL 查询耗时 |
| `trading_channel_dropped_total` | Counter | Channel 满时丢弃的数量 |


---

## 🛠️ Makefile 命令速查

```bash
# 开发相关
make run            # 运行应用
make dev            # 一键启动 MySQL + Redis + 应用
make build          # 编译二进制

# Docker 相关
make docker-up      # 启动依赖服务
make docker-down    # 停止所有服务
make docker-status  # 查看服务状态
make docker-logs    # 查看日志

# K8s 相关
make minikube-start    # 启动 Minikube
make k8s-deploy        # 部署到 K8s
make k8s-status        # 查看 K8s 资源
make k8s-logs          # 查看应用日志
make k8s-delete        # 删除 K8s 资源
```

---

## 💡 常见问题

### Q: 本地运行连不上数据库？
A: 先启动依赖服务：`docker-compose up -d mysql redis`

### Q: K8s 里服务怎么访问？
A: 使用：`make k8s-access` 或 `minikube service -n trading-system <service-name>`

### Q: Grafana 账号密码是什么？
A: admin / admin123

---

## 📝 License
MIT
