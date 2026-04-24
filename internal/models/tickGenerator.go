package models

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// GeneratorMode 生成模式
type GeneratorMode int

const (
	ModeNormal GeneratorMode = iota // 正常模式
	ModeStress                      // 压力测试模式
	ModeCustom                      // 自定义模式
)

// TickGeneratorConfig 生成器配置
type TickGeneratorConfig struct {
	Symbols         []string      // 股票列表
	BasePrice       float64       // 基础价格
	PriceRange      float64       // 价格波动范围
	Mu              float64       // 漂移率（年化）
	Sigma           float64       // 波动率（年化）
	GenInterval     time.Duration // 生成间隔
	DuplicateRate   float64       // 重复概率 0-1
	ConflictRate    float64       // 字段冲突概率 0-1
	LateTickRate    float64       // 延迟tick概率 0-1
	MaxLateDuration time.Duration // 最大延迟时间
	VolatilityMode  bool          // 是否开启高波动模式
	BurstMode       bool          // 是否开启爆发模式
}

// DefaultTickGeneratorConfig 默认配置
func DefaultTickGeneratorConfig() *TickGeneratorConfig {
	return &TickGeneratorConfig{
		Symbols: []string{
			"000001.SZ", "600000.SH", "000002.SZ", "600036.SH", "000858.SZ",
			"600519.SH", "002415.SZ", "601318.SH", "000333.SZ", "600028.SH",
		},
		BasePrice:       10.0,
		PriceRange:      400.0,
		Mu:              0.03,
		Sigma:           0.1,
		GenInterval:     10 * time.Millisecond,
		DuplicateRate:   0.2,
		ConflictRate:    0.15,
		LateTickRate:    0.05,
		MaxLateDuration: 5 * time.Second,
		VolatilityMode:  false,
		BurstMode:       false,
	}
}

// StressTestConfig 压力测试配置
func StressTestConfig() *TickGeneratorConfig {
	cfg := DefaultTickGeneratorConfig()
	cfg.GenInterval = 1 * time.Millisecond
	cfg.DuplicateRate = 0.5
	cfg.ConflictRate = 0.3
	cfg.LateTickRate = 0.1
	return cfg
}

// TickGenerator 多交易所行情生成器（增强版）
type TickGenerator struct {
	cfg         *TickGeneratorConfig
	rng         *rand.Rand // 直接使用rand.Int()全局函数，在高并发下会锁竞争，性能暴跌
	prices      map[string]float64
	seqCounters map[string]int64
	mu          sync.RWMutex
	mode        GeneratorMode
	paused      bool
	burstCount  int
}

// NewTickGenerator 创建生成器
func NewTickGenerator() *TickGenerator {
	return NewTickGeneratorWithConfig(DefaultTickGeneratorConfig())
}

// NewTickGeneratorWithConfig 使用配置创建生成器
func NewTickGeneratorWithConfig(cfg *TickGeneratorConfig) *TickGenerator {
	prices := make(map[string]float64)
	seqCounters := make(map[string]int64)

	for _, s := range cfg.Symbols {
		prices[s] = cfg.BasePrice + rand.Float64()*cfg.PriceRange
		seqCounters[s] = 0
	}

	return &TickGenerator{
		cfg:         cfg,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		prices:      prices,
		seqCounters: seqCounters,
		mode:        ModeNormal,
		paused:      false,
	}
}

// SetMode 设置生成模式
func (tg *TickGenerator) SetMode(mode GeneratorMode) {
	var newCfg *TickGeneratorConfig
	if mode == ModeStress {
		newCfg = StressTestConfig()
	}
	tg.mu.Lock()
	tg.mode = mode
	if newCfg != nil {
		tg.cfg = newCfg
	}
	tg.mu.Unlock()
}

// GetMode 获取当前模式
func (tg *TickGenerator) GetMode() GeneratorMode {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	return tg.mode
}

// Pause 暂停生成
func (tg *TickGenerator) Pause() {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.paused = true
}

// Resume 恢复生成
func (tg *TickGenerator) Resume() {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.paused = false
}

// IsPaused 检查是否暂停
func (tg *TickGenerator) IsPaused() bool {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	return tg.paused
}

// SetConfig 更新配置
func (tg *TickGenerator) SetConfig(cfg *TickGeneratorConfig) {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.cfg = cfg
}

// GetConfig 获取当前配置
func (tg *TickGenerator) GetConfig() *TickGeneratorConfig {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	return tg.cfg
}

// GetSymbols 获取所有股票代码
func (tg *TickGenerator) GetSymbols() []string {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	return append([]string{}, tg.cfg.Symbols...)
}

// GetCurrentPrice 获取指定股票当前价格
func (tg *TickGenerator) GetCurrentPrice(symbol string) (float64, bool) {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	price, ok := tg.prices[symbol]
	return price, ok
}

// TriggerBurst 触发爆发模式
func (tg *TickGenerator) TriggerBurst(count int) {
	tg.mu.Lock()
	defer tg.mu.Unlock()
	tg.burstCount = count
}

// GetSymbolMarket 获取股票对应的交易所
func (tg *TickGenerator) GetSymbolMarket(symbol string) Market {
	if len(symbol) < 3 {
		return SSE
	}
	if symbol[len(symbol)-2:] == "SZ" {
		return SZSE
	}
	return SSE
}

// GenerateNext 生成下一条 Tick（含噪声）
func (tg *TickGenerator) GenerateNext() *Tick {
	tg.mu.RLock()
	if tg.paused {
		tg.mu.RUnlock()
		return nil
	}

	if tg.burstCount > 0 {
		tg.mu.RUnlock()
		tg.mu.Lock()
		tg.burstCount--
		tg.mu.Unlock()
	} else {
		tg.mu.RUnlock()
	}

	tg.mu.RLock()
	idx := tg.rng.Intn(len(tg.cfg.Symbols))
	symbol := tg.cfg.Symbols[idx]
	market := tg.GetSymbolMarket(symbol)
	prevPrice := tg.prices[symbol]
	cfg := tg.cfg
	tg.mu.RUnlock()

	mu := cfg.Mu / (252 * 24 * 3600)
	sigma := cfg.Sigma / math.Sqrt(252*24*3600)

	if cfg.VolatilityMode {
		sigma *= 3
	}

	drift := (mu - 0.5*sigma*sigma) * 1.0
	diffusion := sigma * tg.rng.NormFloat64()
	newPrice := prevPrice * math.Exp(drift+diffusion)

	tg.mu.Lock()
	tg.prices[symbol] = newPrice
	tg.seqCounters[symbol]++
	seqNo := tg.seqCounters[symbol]
	tg.mu.Unlock()

	exchangeTime := time.Now()

	// 从对象池获取 Tick
	tick := AcquireTick()
	tick.Symbol = symbol
	tick.Market = market
	tick.ExchangeTime = exchangeTime
	tick.ArriveTime = exchangeTime
	tick.Price = newPrice
	tick.Volume = 100 + tg.rng.Int63n(10000)
	tick.SeqNo = seqNo

	return tick
}

// GenerateNextWithSymbol 生成指定股票的 Tick
func (tg *TickGenerator) GenerateNextWithSymbol(symbol string) *Tick {
	tg.mu.RLock()
	if tg.paused {
		tg.mu.RUnlock()
		return nil
	}

	prevPrice, ok := tg.prices[symbol]
	if !ok {
		tg.mu.RUnlock()
		return nil
	}
	market := tg.GetSymbolMarket(symbol)
	cfg := tg.cfg
	tg.mu.RUnlock()

	mu := cfg.Mu / (252 * 24 * 3600)
	sigma := cfg.Sigma / math.Sqrt(252*24*3600)

	drift := (mu - 0.5*sigma*sigma) * 1.0
	diffusion := sigma * tg.rng.NormFloat64()
	newPrice := prevPrice * math.Exp(drift+diffusion)

	tg.mu.Lock()
	tg.prices[symbol] = newPrice
	tg.seqCounters[symbol]++
	seqNo := tg.seqCounters[symbol]
	tg.mu.Unlock()

	exchangeTime := time.Now()

	// 从对象池获取 Tick
	tick := AcquireTick()
	tick.Symbol = symbol
	tick.Market = market
	tick.ExchangeTime = exchangeTime
	tick.ArriveTime = exchangeTime
	tick.Price = newPrice
	tick.Volume = 100 + tg.rng.Int63n(10000)
	tick.SeqNo = seqNo

	return tick
}

// GenerateWithDuplicateConflict 生成带重复、带字段冲突的 tick（用于演示）
func (tg *TickGenerator) GenerateWithDuplicateConflict() []*Tick {
	tg.mu.RLock()
	cfg := tg.cfg
	tg.mu.RUnlock()

	var result []*Tick

	tick1 := tg.GenerateNext()
	if tick1 == nil {
		return result
	}
	result = append(result, tick1)

	if tg.rng.Float64() < cfg.DuplicateRate {
		dup := *tick1
		dup.ArriveTime = dup.ArriveTime.Add(30 * time.Millisecond)
		dup.Price = dup.Price * (0.999 + tg.rng.Float64()*0.002)
		dup.Volume = dup.Volume + tg.rng.Int63n(100) - 50
		result = append(result, &dup)
	}

	if tg.rng.Float64() < cfg.LateTickRate {
		lateTick := *tick1
		delay := time.Duration(tg.rng.Int63n(int64(cfg.MaxLateDuration)))
		lateTick.ArriveTime = lateTick.ArriveTime.Add(delay)
		lateTick.SeqNo = tick1.SeqNo + 1
		result = append(result, &lateTick)
	}

	return result
}

// ApplyNoise 给 Tick 加噪声（延迟/乱序/重复）
func (tg *TickGenerator) ApplyNoise(ticks []*Tick) []*Tick {
	tg.mu.RLock()
	cfg := tg.cfg
	tg.mu.RUnlock()

	var result []*Tick

	for _, t := range ticks {
		result = append(result, t)

		if tg.rng.Float64() < 0.3 {
			delay := time.Duration(tg.rng.Intn(100)) * time.Millisecond
			t.ArriveTime = t.ArriveTime.Add(delay)
		}

		if tg.rng.Float64() < cfg.LateTickRate {
			delay := time.Duration(tg.rng.Int63n(int64(cfg.MaxLateDuration)))
			t.ArriveTime = t.ArriveTime.Add(delay)
		}

		if tg.rng.Float64() < cfg.DuplicateRate {
			dup := *t
			dup.ArriveTime = dup.ArriveTime.Add(50 * time.Millisecond)
			result = append(result, &dup)
		}
	}

	return result
}

// GenerateBatch 生成一批 Tick
func (tg *TickGenerator) GenerateBatch(count int) []*Tick {
	var ticks []*Tick
	for i := 0; i < count; i++ {
		tick := tg.GenerateNext()
		if tick != nil {
			ticks = append(ticks, tick)
		}
	}
	return tg.ApplyNoise(ticks)
}
