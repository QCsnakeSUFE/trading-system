# Minikube 部署指南

## 📋 前置要求

1. **安装 Docker Desktop**
2. **安装 Minikube**
   - Mac: `brew install minikube`
   - 或访问 https://minikube.sigs.k8s.io/docs/start/

---

## 🚀 快速开始（3步部署）

### 第一步：启动 Minikube
```bash
make minikube-start
```
这会启动一个单节点的 k8s 集群，配置为 2核 CPU 和 2GB 内存。

### 第二步：部署应用
```bash
make k8s-deploy
```
这会：
1. 构建 Docker 镜像到 minikube 环境
2. 使用 Kustomize 部署所有组件

### 第三步：访问服务
```bash
make k8s-access
```
这会自动打开：
- 应用 Metrics
- Prometheus
- Grafana (用户名: admin, 密码: admin123)

---

## 📊 其他常用命令

### 查看状态
```bash
# 查看 Minikube 状态
make minikube-status

# 查看 K8s 部署状态
make k8s-status
```

### 查看日志
```bash
make k8s-logs
```

### 打开 Dashboard
```bash
make minikube-dashboard
```

### 删除部署
```bash
make k8s-delete
```

### 停止 Minikube
```bash
make minikube-stop
```

---

## 🔍 手动验证部署

检查 Pod 是否正常运行：
```bash
kubectl get pods -n trading-system
```

你应该看到类似这样的输出：
```
NAME                           READY   STATUS    RESTARTS   AGE
grafana-xxx                    1/1     Running   0          2m
mysql-0                        1/1     Running   0          2m
prometheus-xxx                 1/1     Running   0          2m
redis-xxx                      1/1     Running   0          2m
trading-app-xxx                1/1     Running   0          2m
```

---

## 🛠️ 故障排查

### 问题 1：Pod 一直处于 Pending 状态
```bash
# 查看 Pod 详情
kubectl describe pod -n trading-system <pod-name>
```
通常是资源不足，可以尝试：
```bash
make minikube-delete
make minikube-start
```

### 问题 2：应用镜像拉取失败
确保你使用了 `minikube docker-env` 构建镜像：
```bash
make k8s-deploy
```

### 问题 3：服务无法访问
```bash
# 查看服务
kubectl get svc -n trading-system

# 手动打开服务
minikube service -n trading-system prometheus
minikube service -n trading-system grafana
```

---

## 📦 部署的组件

| 组件 | 说明 | 访问方式 |
|------|------|---------|
| MySQL | 数据库 | ClusterIP 内部访问 |
| Redis | 缓存 | ClusterIP 内部访问 |
| Trading App | 行情引擎 | NodePort / minikube service |
| Prometheus | 监控 | NodePort / minikube service |
| Grafana | 可视化 | NodePort / minikube service |

---

## 💡 提示

1. **首次启动可能需要几分钟**来拉取镜像
2. 如果遇到网络问题，先配置 Docker Desktop 镜像加速器（见 DOCKER_SETUP.md）
3. 可以通过 `minikube dashboard` 可视化查看集群状态

祝使用愉快！🎉
