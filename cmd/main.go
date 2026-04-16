package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading_system/internal/aggregator"
	"trading_system/internal/api"
	"trading_system/internal/metrics"
	"trading_system/internal/models"
	"trading_system/pkg/db"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("分布式量化行情中台 (V2.0) 正在启动...")
	fmt.Println("包含: SQL, Redis, Docker, K8s, Prometheus")
	fmt.Println("========================================")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n收到关闭信号，正在优雅关闭...")
		cancel()
	}()

	database := db.InitDB()
	rdb := db.InitRedis()

	klineWorker := aggregator.NewKLineAggregator(database)

	go startHTTPServer(database, rdb)

	go simulateMarketData(ctx, database, rdb, klineWorker)

	<-ctx.Done()
	fmt.Println("行情引擎已安全关闭，内存释放完毕。")
}

func startHTTPServer(database *gorm.DB, rdb *redis.Client) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	handler := api.NewHandler(database, rdb)

	r.GET("/health", handler.HealthCheck)
	r.GET("/ready", handler.ReadyCheck)
	r.GET("/metrics", api.PrometheusHandler())

	apiGroup := r.Group("/api/v1")
	{
		apiGroup.GET("/quotes", handler.GetQuotes)
		apiGroup.GET("/klines", handler.GetKLines)
		apiGroup.POST("/quotes", handler.CreateQuote)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("HTTP 服务器启动在端口 %s\n", port)
	if err := r.Run(":" + port); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP 服务器启动失败: %v", err)
	}
}

func simulateMarketData(ctx context.Context, database *gorm.DB, rdb *redis.Client, klineWorker *aggregator.KLineAggregator) {
	fmt.Println("开始模拟实时行情数据流...")
	priceSim := models.NewPriceSimulator(150.0)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fakePrice := priceSim.NextPrice()
			quote := models.MarketQuote{
				Symbol:    "AAPL",
				Price:     fakePrice,
				Timestamp: time.Now(),
			}

			start := time.Now()
			result := database.Create(&quote)
			metrics.MySQLQueryDuration.WithLabelValues("insert").Observe(time.Since(start).Seconds())

			if result.Error != nil {
				log.Printf("MySQL 写入失败：%v", result.Error)
			} else {
				metrics.QuoteReceivedTotal.WithLabelValues("AAPL").Inc()
			}

			redisStart := time.Now()
			err := rdb.Set(ctx, "LATEST_PRICE_AAPL", fakePrice, 0).Err()
			metrics.RedisCommandDuration.WithLabelValues("set").Observe(time.Since(redisStart).Seconds())

			if err != nil {
				log.Printf("Redis 更新失败：%v", err)
			}

			processStart := time.Now()
			klineWorker.ProcessTick(quote)
			metrics.QuoteProcessingDuration.WithLabelValues("AAPL").Observe(time.Since(processStart).Seconds())
			metrics.QuoteProcessedTotal.WithLabelValues("AAPL").Inc()
			metrics.CurrentPrice.WithLabelValues("AAPL").Set(fakePrice)

			fmt.Printf("[%s] 价格: %.2f\n", time.Now().Format("15:04:05.000"), fakePrice)
		}
	}
}
