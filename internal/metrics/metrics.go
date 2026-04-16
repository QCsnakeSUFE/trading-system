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
)
