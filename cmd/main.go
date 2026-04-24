package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading_system/internal/appstate"
	"trading_system/internal/metrics"
	"trading_system/internal/models"
	"trading_system/internal/pipeline"
	"trading_system/pkg/db"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AppConfig struct {
	Mode        string
	GenInterval time.Duration
	WorkerCount int
	NoGenerate  bool
	Silent      bool
}

func parseFlags() *AppConfig {
	cfg := &AppConfig{}

	flag.StringVar(&cfg.Mode, "mode", "normal", "运行模式: normal|stress|custom")
	flag.DurationVar(&cfg.GenInterval, "interval", 10*time.Millisecond, "数据生成间隔")
	flag.IntVar(&cfg.WorkerCount, "workers", 4, "Pipeline worker数量")
	flag.BoolVar(&cfg.NoGenerate, "no-generate", false, "不生成模拟数据")
	flag.BoolVar(&cfg.Silent, "silent", false, "静默模式，减少日志输出")

	flag.Parse()
	return cfg
}

func printBanner(cfg *AppConfig) {
	fmt.Println("========================================")
	fmt.Println("  分布式量化行情中台 (V4.0) 正在启动...")
	fmt.Println("========================================")
	fmt.Printf("  运行模式:     %s\n", cfg.Mode)
	fmt.Printf("  Worker数量:   %d\n", cfg.WorkerCount)
	fmt.Printf("  生成间隔:     %v\n", cfg.GenInterval)
	if cfg.NoGenerate {
		fmt.Println("  数据生成:     已禁用")
	}
	if cfg.Silent {
		fmt.Println("  日志:         静默模式")
	}
	fmt.Println("========================================")
	fmt.Println()
}

func main() {
	cfg := parseFlags()
	printBanner(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n收到关闭信号，正在优雅关闭...")
		cancel()
	}()

	// 初始化数据库和 Redis
	database := db.InitDB()
	rdb := db.InitRedis()

	// 启动 HTTP 服务器（包含 metrics 和健康检查）
	go func() {
		// Metrics 端点
		http.Handle("/metrics", promhttp.Handler())

		// 健康检查端点：应用是否存活
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		})

		// 就绪检查端点：依赖服务是否可用
		http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
			// 检查 MySQL 连接
			sqlDB, err := database.DB()
			if err != nil {
				http.Error(w, "database not ready", http.StatusServiceUnavailable)
				return
			}
			if err := sqlDB.Ping(); err != nil {
				http.Error(w, "database ping failed", http.StatusServiceUnavailable)
				return
			}

			// 检查 Redis 连接
			if err := rdb.Ping(ctx).Err(); err != nil {
				http.Error(w, "redis not ready", http.StatusServiceUnavailable)
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ready"}`))
		})

		fmt.Println("HTTP 服务器已启动，监听 :2112")
		fmt.Println("  - /metrics: Prometheus 指标")
		fmt.Println("  - /health: 存活检查")
		fmt.Println("  - /ready: 就绪检查")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			log.Printf("HTTP 服务器启动失败: %v", err)
		}
	}()

	// 插入测试数据
	insertTestKLineData(database)

	// 启动 Pipeline
	dataPipeline := pipeline.NewDataPipeline(ctx)
	dataPipeline.Start(cfg.WorkerCount)

	// 启动消费者
	go consumeKLines(ctx, database, rdb, dataPipeline.OutChan, cfg.Silent)

	// 启动数据生成
	if !cfg.NoGenerate {
		tickGen := setupTickGenerator(cfg)
		appstate.SetGlobalTickGenerator(tickGen)
		go generateTickData(ctx, dataPipeline.InChan, tickGen, cfg)
	}

	<-ctx.Done()
	fmt.Println("行情引擎已安全关闭，内存释放完毕。")
}

func setupTickGenerator(cfg *AppConfig) *models.TickGenerator {
	var tickGen *models.TickGenerator

	switch cfg.Mode {
	case "stress":
		tickGen = models.NewTickGeneratorWithConfig(models.StressTestConfig())
		tickGen.SetMode(models.ModeStress)
	case "custom":
		customCfg := models.DefaultTickGeneratorConfig()
		customCfg.GenInterval = cfg.GenInterval
		tickGen = models.NewTickGeneratorWithConfig(customCfg)
		tickGen.SetMode(models.ModeCustom)
	default:
		tickGen = models.NewTickGenerator()
		tickGen.SetMode(models.ModeNormal)
	}

	return tickGen
}

func generateTickData(ctx context.Context, inChan chan<- *models.Tick, tickGen *models.TickGenerator, cfg *AppConfig) {
	fmt.Println("开始生成行情数据流...")

	genInterval := tickGen.GetConfig().GenInterval
	if cfg.Mode == "custom" {
		genInterval = cfg.GenInterval
	}

	ticker := time.NewTicker(genInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			close(inChan)
			return
		case <-ticker.C:
			if tickGen.IsPaused() {
				continue
			}

			ticks := tickGen.GenerateWithDuplicateConflict()
			for _, tick := range ticks {
				if tick != nil {
					// 非阻塞发送，防止 channel 满了阻塞
					select {
					case inChan <- tick:
						metrics.QuoteReceivedTotal.WithLabelValues(tick.Symbol).Inc()
						metrics.CurrentPrice.WithLabelValues(tick.Symbol).Set(tick.Price)
					default:
						metrics.ChannelDroppedTotal.Inc()
					}
				}
			}

			if !cfg.Silent && len(ticks) == 1 {
				tick := ticks[0]
				fmt.Printf("[%s] %-10s Price: %.2f  Vol: %d\n",
					time.Now().Format("15:04:05.000"),
					tick.Symbol, tick.Price, tick.Volume)
			}
		}
	}
}

func consumeKLines(ctx context.Context, database *gorm.DB, rdb *redis.Client, outChan <-chan *models.KLine, silent bool) {
	klineCount := 0
	for kline := range outChan {
		klineCount++

		// 写入 MySQL
		start := time.Now()
		result := database.Create(&models.MarketQuote{
			Symbol:    kline.Symbol,
			Price:     kline.Close,
			Timestamp: kline.StartTime,
		})
		metrics.MySQLQueryDuration.WithLabelValues("insert").Observe(time.Since(start).Seconds())

		if result.Error != nil {
			log.Printf("MySQL写入Quote失败: %v", result.Error)
		}

		klineResult := database.Create(&models.MinuteKLine{
			Symbol:     kline.Symbol,
			Open:       kline.Open,
			High:       kline.High,
			Low:        kline.Low,
			Close:      kline.Close,
			Volume:     kline.Volume,
			MinuteTime: kline.StartTime,
		})

		if klineResult.Error != nil {
			log.Printf("MySQL写入MinuteKLine失败: %v", klineResult.Error)
		}

		// 更新 Redis
		redisStart := time.Now()
		err := rdb.Set(ctx, "LATEST_PRICE_"+kline.Symbol, kline.Close, 0).Err()
		metrics.RedisCommandDuration.WithLabelValues("set").Observe(time.Since(redisStart).Seconds())

		if err != nil {
			log.Printf("Redis更新K线失败: %v", err)
		}

		if !silent && klineCount%10 == 0 {
			fmt.Printf("-> K线 %d: %-10s [%.2f, %.2f, %.2f, %.2f] Vol: %d\n",
				klineCount, kline.Symbol, kline.Open, kline.High, kline.Low, kline.Close, kline.Volume)
		}

		// 把 KLine 放回对象池！
		models.ReleaseKLine(kline)
	}
}

func insertTestKLineData(database *gorm.DB) {
	now := time.Now()
	symbols := []string{
		"000001.SZ", "600000.SH", "000002.SZ", "600036.SH", "000858.SZ",
		"600519.SH", "002415.SZ", "601318.SH", "000333.SZ", "600028.SH",
	}

	basePrices := map[string]float64{
		"000001.SZ": 10.0,
		"600000.SH": 8.5,
		"000002.SZ": 12.0,
		"600036.SH": 35.0,
		"000858.SZ": 150.0,
		"600519.SH": 1800.0,
		"002415.SZ": 60.0,
		"601318.SH": 45.0,
		"000333.SZ": 55.0,
		"600028.SH": 6.5,
	}

	totalCount := 0

	for _, symbol := range symbols {
		basePrice := basePrices[symbol]
		testData := []models.MinuteKLine{}

		for i := 0; i < 30; i++ {
			minuteTime := now.Add(-time.Duration(30-i) * time.Minute)
			open := basePrice + float64(i)*basePrice*0.001 - basePrice*0.015
			close := open + (rand.Float64()-0.5)*basePrice*0.004
			high := math.Max(open, close) + rand.Float64()*basePrice*0.002
			low := math.Min(open, close) - rand.Float64()*basePrice*0.002
			volume := int64(rand.Intn(10000) + 5000)

			testData = append(testData, models.MinuteKLine{
				Symbol:     symbol,
				Open:       open,
				High:       high,
				Low:        low,
				Close:      close,
				Volume:     volume,
				MinuteTime: minuteTime,
			})
		}

		for _, kline := range testData {
			result := database.Create(&kline)
			if result.Error != nil {
				log.Printf("插入测试K线失败: %v", result.Error)
			}
		}
		totalCount += len(testData)
	}

	fmt.Printf("已为 %d 个品种插入 %d 条测试K线数据\n", len(symbols), totalCount)
}
