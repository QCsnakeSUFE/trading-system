package api

import (
	"net/http"
	"trading_system/internal/metrics"
	"trading_system/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Handler struct {
	db  *gorm.DB
	rdb *redis.Client
}

func NewHandler(db *gorm.DB, rdb *redis.Client) *Handler {
	return &Handler{db: db, rdb: rdb}
}

func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "trading_system",
	})
}

func (h *Handler) ReadyCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

func (h *Handler) GetQuotes(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		symbol = "AAPL"
	}

	var quotes []models.MarketQuote
	result := h.db.Where("symbol = ?", symbol).Order("timestamp DESC").Limit(100).Find(&quotes)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol": symbol,
		"count":  len(quotes),
		"data":   quotes,
	})
}

func (h *Handler) GetKLines(c *gin.Context) {
	symbol := c.Query("symbol")
	if symbol == "" {
		symbol = "AAPL"
	}

	var klines []models.MinuteKLine
	result := h.db.Where("symbol = ?", symbol).Order("minute_time DESC").Limit(100).Find(&klines)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol": symbol,
		"count":  len(klines),
		"data":   klines,
	})
}

func (h *Handler) CreateQuote(c *gin.Context) {
	var quote models.MarketQuote
	if err := c.ShouldBindJSON(&quote); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := h.db.Create(&quote)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	metrics.QuoteReceivedTotal.WithLabelValues(quote.Symbol).Inc()
	metrics.QuoteProcessedTotal.WithLabelValues(quote.Symbol).Inc()
	metrics.CurrentPrice.WithLabelValues(quote.Symbol).Set(quote.Price)

	c.JSON(http.StatusCreated, quote)
}
