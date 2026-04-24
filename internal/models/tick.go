package models

import (
	"sync"
	"time"
)

// Market 交易所类型
type Market string

const (
	SSE  Market = "SSE"  // 上交所
	SZSE Market = "SZSE" // 深交所
)

// Tick 原始行情数据
type Tick struct {
	Symbol       string    // 股票代码，如 "000001.SZ"
	Market       Market    // 交易所
	ExchangeTime time.Time // 交易所时间戳（核心时间）
	ArriveTime   time.Time // 到达时间
	Price        float64   // 成交价格
	Volume       int64     // 成交量
	SeqNo        int64     // 序列号（用于去重）
}

// KLine K线数据
type KLine struct {
	Symbol    string    `json:"symbol"`
	Market    Market    `json:"market"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
	IsClosed  bool      `json:"is_closed"`
}

// sync.Pool 对象池，减少 GC 压力

var TickPool = &sync.Pool{
	New: func() any {
		return &Tick{}
	},
}

var KLinePool = &sync.Pool{
	New: func() any {
		return &KLine{}
	},
}

// AcquireTick 从池中获取一个 Tick 对象
func AcquireTick() *Tick {
	return TickPool.Get().(*Tick)
}

// ReleaseTick 将 Tick 对象放回池中（重置后）
func ReleaseTick(t *Tick) {
	// 重置字段，避免脏数据
	t.Symbol = ""
	t.Market = ""
	t.ExchangeTime = time.Time{}
	t.ArriveTime = time.Time{}
	t.Price = 0
	t.Volume = 0
	t.SeqNo = 0
	TickPool.Put(t)
}

// AcquireKLine 从池中获取一个 KLine 对象
func AcquireKLine() *KLine {
	return KLinePool.Get().(*KLine)
}

// ReleaseKLine 将 KLine 对象放回池中（重置后）
func ReleaseKLine(k *KLine) {
	k.Symbol = ""
	k.Market = ""
	k.StartTime = time.Time{}
	k.EndTime = time.Time{}
	k.Open = 0
	k.High = 0
	k.Low = 0
	k.Close = 0
	k.Volume = 0
	k.IsClosed = false
	KLinePool.Put(k)
}
