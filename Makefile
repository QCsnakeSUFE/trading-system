.PHONY: run build docker-build docker-up docker-down clean dev
.PHONY: minikube-start minikube-stop minikube-status minikube-dashboard
.PHONY: k8s-deploy k8s-delete k8s-status k8s-logs k8s-access

# 运行项目
run:
	go run cmd/main.go

# 静默模式运行
run-silent:
	go run cmd/main.go --silent

# 构建项目
build:
	go build -o trading_system cmd/main.go

# 构建 Docker 镜像
docker-build:
	docker build -t trading-system:latest .

# 启动所有服务（使用官方镜像）
docker-up:
	docker-compose up -d mysql redis prometheus grafana

# 停止所有服务
docker-down:
	docker-compose down
	-docker-compose -f docker-compose.cn.yml down 2>/dev/null

# 查看服务状态
docker-status:
	docker-compose ps
	@echo ""
	@echo "国内镜像服务状态："
	@docker-compose -f docker-compose.cn.yml ps 2>/dev/null || true

# 查看日志
docker-logs:
	docker-compose logs -f

# 清理编译文件
clean:
	rm -f trading_system

# === Minikube 相关命令 ===

# 启动 minikube
minikube-start:
	minikube start --driver=docker --cpus=2 --memory=2g

# 停止 minikube
minikube-stop:
	minikube stop

# 删除 minikube
minikube-delete:
	minikube delete

# 查看 minikube 状态
minikube-status:
	minikube status

# 打开 minikube dashboard
minikube-dashboard:
	minikube dashboard

# 构建 Docker 镜像到 minikube 环境
minikube-docker-build:
	@eval $$(minikube docker-env) && docker build -t trading-system:latest .

# 部署到 minikube
k8s-deploy: minikube-docker-build
	kubectl apply -k deploy/k8s

# 删除 k8s 部署
k8s-delete:
	kubectl delete -k deploy/k8s

# 查看 k8s 状态
k8s-status:
	kubectl get all -n trading-system

# 查看 k8s 日志
k8s-logs:
	kubectl logs -f -n trading-system -l app=trading-app

# 访问服务
k8s-access:
	@echo "正在打开服务..."
	@minikube service -n trading-system trading-app
	@minikube service -n trading-system prometheus
	@minikube service -n trading-system grafana

# 一键启用依赖服务并运行 Go 应用
dev:
	docker-compose up -d mysql redis
	echo "依赖服务成功启动"
	make run
	echo "Go 应用成功启动"