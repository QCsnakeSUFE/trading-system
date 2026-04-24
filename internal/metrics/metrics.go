package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	QuoteReceivedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "trading_quote_received_total",
			Help: "Total number of quotes received",
		},
		[]string{"symbol"},
	)

	QuoteProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "trading_quote_processed_total",
			Help: "Total number of quotes processed",
		},
		[]string{"symbol"},
	)

	QuoteProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "trading_quote_processing_duration_seconds",
			Help:    "Duration of quote processing in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"symbol"},
	)

	MySQLQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "trading_mysql_query_duration_seconds",
			Help:    "Duration of MySQL queries in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		},
		[]string{"operation"},
	)

	RedisCommandDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "trading_redis_command_duration_seconds",
			Help:    "Duration of Redis commands in seconds",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05},
		},
		[]string{"command"},
	)

	ActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "trading_active_connections",
			Help: "Number of active connections",
		},
	)

	CurrentPrice = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "trading_current_price",
			Help: "Current price per symbol",
		},
		[]string{"symbol"},
	)

	// ================== 去重模块指标（新增）==================
	DedupeTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_dedupe_total",
			Help: "Total number of ticks processed by deduper",
		},
	)

	DedupeDuplicateTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_dedupe_duplicate_total",
			Help: "Total number of duplicate ticks found",
		},
	)

	DedupeConflictTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_dedupe_conflict_total",
			Help: "Total number of tick conflicts found (price/volume differs)",
		},
	)

	DedupeEvictedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_dedupe_evicted_total",
			Help: "Total number of tick entries evicted from LRU cache",
		},
	)

	// ================== WindowManager 指标（新增）==================
	WindowManagerKlineClosedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_window_kline_closed_total",
			Help: "Total number of K-lines successfully closed and sent",
		},
	)

	WindowManagerLateTickTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_window_late_tick_total",
			Help: "Total number of ticks that arrived after window closed",
		},
	)

	WindowManagerDropKlineTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_window_drop_kline_total",
			Help: "Total number of K-lines dropped because outChan full",
		},
	)

	WindowManagerErrorTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_window_error_total",
			Help: "Total number of errors in window manager",
		},
	)

	// ================== DataPipeline 指标（新增）==================
	PipelineTickReceivedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_pipeline_tick_received_total",
			Help: "Total number of ticks received by pipeline",
		},
	)

	PipelineInChanGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "trading_pipeline_in_chan_gauge",
			Help: "Current size of InChan",
		},
	)

	ChannelDroppedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "trading_channel_dropped_total",
			Help: "Total number of ticks dropped because channel was full",
		},
	)
)
