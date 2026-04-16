# 分布式量化行情中台 - 云原生运维项目

这是一个完整的云原生运维项目，涵盖 **SQL、Redis、Docker、Kubernetes、Prometheus** 等技术栈，适合作为 **SRE/K8s/AIops** 岗位的面试项目。

## 📋 技术栈

| 技术 | 用途 |
|------|------|
| **Go** | 后端开发语言 |
| **MySQL** | 关系型数据库 |
| **Redis** | 缓存/消息队列 |
| **Gin** | HTTP Web 框架 |
| **Docker** | 容器化 |
| **Kubernetes** | 容器编排 |
| **Prometheus** | 监控指标收集 |
| **Grafana** | 监控可视化 |

---

## 🚀 快速开始

### 方式一：Docker Compose 部署（推荐本地开发）

```bash
# 1. 进入项目目录
cd trading_system

# 2. 构建并启动所有服务
docker-compose up -d --build

# 3. 查看服务状态
docker-compose ps

# 4. 查看日志
docker-compose logs -f app
```

**访问地址：**
- 应用: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin123)

---

## 📁 项目结构

```
trading_system/
├── cmd/
│   └── main.go              # 主程序入口
├── internal/
│   ├── api/
│   │   └── handlers.go    # HTTP API 处理器
│   ├── aggregator/
│   │   └── kline.go     # K线聚合器
│   ├── metrics/
│   │   └── metrics.go   # Prometheus 指标定义
│   └── models/
│       └── market.go    # 数据模型
├── pkg/
│   └── db/
│       ├── mysql.go     # MySQL 初始化
│       └── redis.go     # Redis 初始化
├── deploy/
│   ├── docker-compose.yml    # Docker Compose 配置
│   ├── Dockerfile          # Docker 镜像构建
│   ├── k8s/               # Kubernetes 部署清单
│   ├── prometheus/        # Prometheus 配置
│   └── grafana/           # Grafana 配置
├── go.mod
├── go.sum
└── README.md
```

---

## 🔧 核心功能详解

### 1. SQL (MySQL) - 关系型数据库

**文件位置：** `pkg/db/mysql.go`

**为什么这样写？**

```go
func InitDB() *gorm.DB {
    // 1. 从环境变量读取 DSN，支持灵活配置
    dsn := os.Getenv("DB_DSN")
    
    // 2. 重试机制：数据库启动需要时间，启动时重试 5 次
    for i := 0; i < 5; i++ {
        db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
        if err == nil {
            break
        }
        time.Sleep(2 * time.Second)
    }
    
    // 3. 自动迁移：自动创建表结构
    err = db.AutoMigrate(&models.MarketQuote{}, &models.MinuteKLine{})
}
```

**设计要点：**
- **环境变量配置**：不硬编码密码，便于在不同环境部署
- **健康检查重试**：解决容器启动顺序问题，MySQL 启动需要时间
- **自动迁移**：开发阶段自动管理表结构，生产环境建议手动迁移

---

### 2. Redis - 缓存层

**文件位置：** `pkg/db/redis.go`

```go
func InitRedis() *redis.Client {
    rdb := redis.NewClient(&redis.Options{
        Addr: redisAddr,
    })
    
    // 启动时 Ping 一下，检查连接
    if err := rdb.Ping(ctx).Err(); err != nil {
        log.Printf("Redis 暂时连接不上：%v", err)
    }
    return rdb
}
```

**为什么这样写？**
- 即使 Redis 暂时不可用也不阻塞程序启动，服务降级处理
- 缓存热点数据（最新价），减轻 MySQL 压力

---

### 3. Docker 容器化

**文件位置：** `Dockerfile`

```dockerfile
# 多阶段构建：减小最终镜像体积
FROM golang:1.21-alpine AS builder
# ... 构建阶段 ...

FROM alpine:latest
# 只复制编译好的二进制文件
COPY --from=builder /app/trading-system .
```

**为什么这样写？**

| 优势 | 说明 |
|------|------|
| **多阶段构建** | 最终镜像只有 ~20MB，而 golang 镜像有 ~300MB+ |
| **Alpine 基础镜像** | 安全、轻量、含必要的证书和时区数据 |
| **非 root 用户** | 更安全（示例中可进一步优化） |

---

### 4. Kubernetes 部署

**关键文件：** `deploy/k8s/app.yaml`

#### 4.1 Deployment - 部署策略

```yaml
strategy:
  rollingUpdate:
    maxSurge: 1        # 最多同时启动 1 个新 Pod
    maxUnavailable: 0  # 0 个 Pod 不可用
  type: RollingUpdate
```

**为什么？**
- **滚动更新**：保证服务零停机
- `maxUnavailable: 0` 确保始终有足够副本处理流量

#### 4.2 健康检查探针

```yaml
livenessProbe:           # 存活探针：检测容器是否需要重启
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:        # 就绪探针：检测 Pod 是否可以接收流量
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5

startupProbe:          # 启动探针：给应用足够的启动时间
  httpGet:
    path: /health
    port: 8080
  failureThreshold: 30
```

**为什么三种探针？**
- **Startup**：解决慢启动应用，避免被 liveness 误杀
- **Readiness**：只有就绪的 Pod 才会接收 Service 流量
- **Liveness**：容器僵死时自动重启

#### 4.3 资源限制

```yaml
resources:
  requests:    # 申请资源：保证 Pod 能调度到有足够资源的节点
    cpu: 100m
    memory: 128Mi
  limits:      # 限制资源：防止 Pod 占用过多资源
    cpu: 500m
    memory: 256Mi
```

---

### 5. Prometheus 监控

**文件位置：** `internal/metrics/metrics.go`

```go
var (
    QuoteReceivedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "trading_quote_received_total",
            Help: "Total number of quotes received",
        },
        []string{"symbol"},
    )
    
    QuoteProcessingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "trading_quote_processing_duration_seconds",
            Help: "Duration of quote processing in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"symbol"},
    )
)
```

**指标类型说明：**

| 类型 | 用途 | 示例 |
|------|------|------|
| **Counter** | 只增不减的计数器 | 请求总数、错误数 |
| **Gauge** | 可上可下的仪表盘 | 当前连接数、内存使用率 |
| **Histogram** | 直方图，统计分布 | 请求耗时分布 |
| **Summary** | 摘要，分位数统计 | P50/P95/P99 延迟 |

**在代码中使用：**

```go
start := time.Now()
result := database.Create(&quote)
metrics.MySQLQueryDuration.WithLabelValues("insert").Observe(time.Since(start).Seconds())
```

---

### 6. HTTP API & 健康检查

**文件位置：** `internal/api/handlers.go`

```go
// 健康检查：用于 Liveness Probe
func (h *Handler) HealthCheck(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "healthy",
    })
}

// 就绪检查：用于 Readiness Probe
func (h *Handler) ReadyCheck(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{
        "status": "ready",
    })
}
```

**API 端点：**

| 路径 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/ready` | GET | 就绪检查 |
| `/metrics` | GET | Prometheus 指标 |
| `/api/v1/quotes` | GET | 获取行情数据 |
| `/api/v1/klines` | GET | 获取 K线数据 |

---

## 📊 面试必答要点

### SRE 相关问题

**Q1: 为什么要用 Kubernetes？**

- **容器编排**：自动化部署、扩缩容、自愈
- **服务发现**：Pod IP 动态变化，通过 Service 访问
- **负载均衡**：自动分配流量到多个 Pod
- **滚动更新**：零停机发布新版本
- **声明式 API**：描述期望状态，K8s 自动收敛

**Q2: 健康检查探针的区别？**

- **Startup**：给应用足够启动时间，避免被误杀
- **Readiness**：流量只发给就绪的 Pod
- **Liveness**：Pod 僵死时自动重启

**Q3: 资源 requests vs limits？**

- **requests**：Pod 调度的依据，保证能分配到有足够资源的节点
- **limits**：防止 Pod 占用过多资源，影响其他 Pod

**Q4: Prometheus 为什么用 Pull 模式？**

- Pull 模式更容易调试：目标挂了一眼就能看到
- 服务发现：自动发现目标
- 过载保护：Prometheus 控制抓取频率

---

## 🎯 总结

这个项目展示了：

1. ✅ **SQL (MySQL)** - 数据持久化，带重试和自动迁移
2. ✅ **Redis** - 缓存层，热点数据存储
3. ✅ **Docker** - 多阶段构建，镜像优化
4. ✅ **Kubernetes** - Deployment、Service、ConfigMap、Secret、健康检查、资源限制
5. ✅ **Prometheus** - 四种指标类型，业务埋点
6. ✅ **Grafana** - 监控可视化

祝你面试顺利！🎉
