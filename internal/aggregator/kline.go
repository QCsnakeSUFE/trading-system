package aggregator

import (
	"fmt"
	"time"
	"trading_system/internal/models"

	"gorm.io/gorm"
)

type KLineAggregator struct {
	db         *gorm.DB
	currentBar *models.MinuteKLine
}

func NewKLineAggregator(db *gorm.DB) *KLineAggregator {
	return &KLineAggregator{db: db}
}

// 接收一个 Tick，判断并合成 K 线
func (a *KLineAggregator) ProcessTick(tick models.MarketQuote) {
	tickMinute := tick.Timestamp.Truncate(time.Minute)

	if a.currentBar == nil || !a.currentBar.MinuteTime.Equal(tickMinute) {
		if a.currentBar != nil {
			a.db.Create(a.currentBar)
			fmt.Printf("[Aggregator] 分钟线入库：%s | C: %.2f\n", a.currentBar.MinuteTime.Format("15:04"),
				a.currentBar.Close)
		}

		a.currentBar = &models.MinuteKLine{
			Symbol:     tick.Symbol,
			Open:       tick.Price,
			High:       tick.Price,
			Low:        tick.Price,
			Close:      tick.Price,
			MinuteTime: tickMinute,
		}
	} else {
		if tick.Price > a.currentBar.High {
			a.currentBar.High = tick.Price
		}
		if tick.Price < a.currentBar.Low {
			a.currentBar.Low = tick.Price
		}
		a.currentBar.Close = tick.Price // 持续更新收盘价， tick 停止更新时，Close 就是收盘价
	}
}
