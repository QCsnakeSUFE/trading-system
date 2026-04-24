package pipeline

import (
	"log"
	"sync"
	"time"
	"trading_system/internal/metrics"
	"trading_system/internal/models"
)

// WindowKey 窗口唯一键
type WindowKey struct {
	Symbol string
	Minute int64 // Unix 秒，向下取整到分钟
}

// KLineWindow K线窗口
type KLineWindow struct {
	Key         WindowKey
	Market      models.Market // 必须从第一个tick取！
	StartTime   time.Time
	EndTime     time.Time
	Open        float64
	High        float64
	Low         float64
	Close       float64
	Volume      int64
	IsClosed    bool
	TickCount   int
	LastUpdated time.Time // 最后更新时间，用于清理
}

// SymbolState 每个symbol独立状态
type SymbolState struct {
	mu           sync.RWMutex
	watermark    time.Time
	prevKLine    *models.KLine
	lastActivity time.Time
}

// WindowManager 时间窗口管理器（生产级）
type WindowManager struct {
	mu              sync.RWMutex
	windows         map[WindowKey]*KLineWindow
	symbolStates    map[string]*SymbolState // 每个symbol独立！
	allowedLateness time.Duration
	outChan         chan *models.KLine

	// 禁止值拷贝（Go 面试高频）
	noCopy sync.Mutex
}

// NewWindowManager 创建窗口管理器
func NewWindowManager(outChan chan *models.KLine, allowedLateness time.Duration) *WindowManager {
	return &WindowManager{
		windows:         make(map[WindowKey]*KLineWindow),
		symbolStates:    make(map[string]*SymbolState),
		allowedLateness: allowedLateness,
		outChan:         outChan,
	}
}

// AddTick 添加 Tick 到窗口（优化版！）
func (wm *WindowManager) AddTick(tick *models.Tick) {
	// ------------------- 第一步：防御性校验 -------------------
	if tick == nil {
		log.Printf("[WindowManager] Warning: nil tick received, discarded")
		metrics.WindowManagerErrorTotal.Inc()
		return
	}

	if tick.Symbol == "" {
		log.Printf("[WindowManager] Warning: tick with empty symbol received, discarded")
		metrics.WindowManagerErrorTotal.Inc()
		return
	}

	// ------------------- 第二步：按symbol隔离处理 -------------------
	wm.mu.RLock()
	symbolState, exists := wm.symbolStates[tick.Symbol]
	wm.mu.RUnlock()

	if !exists {
		// 新建 symbol 状态
		newState := &SymbolState{
			watermark:    tick.ExchangeTime,
			lastActivity: time.Now(),
		}
		wm.mu.Lock()
		if _, ok := wm.symbolStates[tick.Symbol]; !ok {
			wm.symbolStates[tick.Symbol] = newState
			symbolState = newState
		} else {
			symbolState = wm.symbolStates[tick.Symbol]
		}
		wm.mu.Unlock()
	} else {
		symbolState = wm.symbolStates[tick.Symbol]
	}

	// ------------------- 第三步：更新该 symbol 的 watermark -------------------
	if symbolState == nil {
		log.Printf("[WindowManager] Warning: symbolState is nil for symbol %s", tick.Symbol)
		metrics.WindowManagerErrorTotal.Inc()
		return
	}

	symbolState.mu.Lock()
	if tick.ExchangeTime.After(symbolState.watermark) {
		symbolState.watermark = tick.ExchangeTime
	}
	symbolState.lastActivity = time.Now()
	symbolState.mu.Unlock()

	// ------------------- 第四步：找到或创建窗口（读写锁拆分） -------------------
	minuteUnix := tick.ExchangeTime.Truncate(time.Minute).Unix()
	key := WindowKey{Symbol: tick.Symbol, Minute: minuteUnix}

	// 读锁查窗口
	wm.mu.RLock()
	window, exists := wm.windows[key]
	wm.mu.RUnlock()

	if !exists {
		// 没找到，创建（加写锁）
		wm.mu.Lock()
		// 二次检查（防止并发竞态）
		if window, exists = wm.windows[key]; !exists {
			window = wm.createWindow(tick, key)
			wm.windows[key] = window
		}
		wm.mu.Unlock()
	}

	// ------------------- 第五步：更新窗口（窗口内锁） -------------------
	if !window.IsClosed {
		wm.updateKLineWindow(window, tick)
	} else {
		// 已闭合窗口：记录日志 + 统计
		log.Printf("[WindowManager] Warning: tick arrived after window closed. symbol=%s time=%d closed=%v",
			tick.Symbol, tick.ExchangeTime.Unix(), window.EndTime)
		metrics.WindowManagerLateTickTotal.Inc()
		// 生产环境：这里要加异步修正历史K线的逻辑
	}

	// ------------------- 第六步：检查闭合（单独处理，锁内不发送） -------------------
	wm.checkAndCloseSymbol(tick.Symbol)
}

// createWindow 创建新窗口
func (wm *WindowManager) createWindow(tick *models.Tick, key WindowKey) *KLineWindow {
	start := time.Unix(key.Minute, 0)
	end := start.Add(time.Minute)

	return &KLineWindow{
		Key:         key,
		Market:      tick.Market, // 必须从第一个tick取！
		StartTime:   start,
		EndTime:     end,
		Open:        tick.Price,
		High:        tick.Price,
		Low:         tick.Price,
		Close:       tick.Price,
		Volume:      tick.Volume,
		TickCount:   1,
		LastUpdated: time.Now(),
	}
}

// updateKLineWindow 更新窗口 OHLC
func (wm *WindowManager) updateKLineWindow(window *KLineWindow, tick *models.Tick) {
	// K线窗口本身也应该加锁，但为了简化（假设一个窗口同一时间只有一个协程访问）
	window.Close = tick.Price
	if tick.Price > window.High {
		window.High = tick.Price
	}
	if tick.Price < window.Low {
		window.Low = tick.Price
	}
	window.Volume += tick.Volume
	window.TickCount++
	window.LastUpdated = time.Now()
}

// checkAndCloseSymbol 检查单个 symbol 的窗口闭合（生产级！）
func (wm *WindowManager) checkAndCloseSymbol(symbol string) {
	wm.mu.RLock()
	symbolState, exists := wm.symbolStates[symbol]
	wm.mu.RUnlock()
	if !exists {
		return
	}

	symbolState.mu.RLock()
	watermarkLine := symbolState.watermark.Add(-wm.allowedLateness)
	symbolState.mu.RUnlock()

	// ------------------- 收集需要闭合的窗口（读锁） -------------------
	windowsToClose := make([]*KLineWindow, 0)
	keysToRemove := make([]WindowKey, 0)

	wm.mu.RLock()
	for key, window := range wm.windows {
		if key.Symbol == symbol && !window.IsClosed && window.EndTime.Before(watermarkLine) {
			windowsToClose = append(windowsToClose, window)
			keysToRemove = append(keysToRemove, key)
		}
	}
	wm.mu.RUnlock()

	if len(windowsToClose) == 0 {
		return
	}

	// ------------------- 先构建K线，再修改窗口（锁拆分） -------------------
	klinesToSend := make([]*models.KLine, 0)

	for _, window := range windowsToClose {
		// 填充K线（读锁已放，不阻塞）
		var kline *models.KLine
		if window.TickCount > 0 {
			kline = wm.windowToKLine(window)
		} else {
			// 没数据，用前一根填充
			kline = wm.fillMissing(window.Key)
		}

		klinesToSend = append(klinesToSend, kline)
	}

	// ------------------- 真正修改状态（加写锁） -------------------
	wm.mu.Lock()
	for _, key := range keysToRemove {
		if window, ok := wm.windows[key]; ok {
			window.IsClosed = true
		}
	}
	for _, kline := range klinesToSend {
		symbolState.mu.Lock()
		symbolState.prevKLine = kline
		symbolState.mu.Unlock()
	}
	// 清理窗口
	for _, key := range keysToRemove {
		delete(wm.windows, key)
	}
	wm.mu.Unlock()

	// ------------------- 发送K线（锁外！绝对不锁内阻塞！） -------------------
	for _, kline := range klinesToSend {
		select {
		case wm.outChan <- kline:
			metrics.WindowManagerKlineClosedTotal.Inc()
		default:
			// 通道满了，不要阻塞，记录日志并丢弃
			log.Printf("[WindowManager] Warning: outChan full, dropping kline for symbol=%s", kline.Symbol)
			metrics.WindowManagerDropKlineTotal.Inc()
		}
	}
}

// windowToKLine 窗口转 K 线（无锁）
func (wm *WindowManager) windowToKLine(window *KLineWindow) *models.KLine {
	// 从对象池获取 KLine
	kline := models.AcquireKLine()
	kline.Symbol = window.Key.Symbol
	kline.Market = window.Market
	kline.StartTime = window.StartTime
	kline.EndTime = window.EndTime
	kline.Open = window.Open
	kline.High = window.High
	kline.Low = window.Low
	kline.Close = window.Close
	kline.Volume = window.Volume
	kline.IsClosed = true
	return kline
}

// fillMissing 填充缺失数据（无锁）
func (wm *WindowManager) fillMissing(key WindowKey) *models.KLine {
	var prev *models.KLine
	wm.mu.RLock()
	if symbolState, ok := wm.symbolStates[key.Symbol]; ok {
		symbolState.mu.RLock()
		prev = symbolState.prevKLine
		symbolState.mu.RUnlock()
	}
	wm.mu.RUnlock()

	if prev == nil {
		// 没有前一根，返回一个默认值
		kline := models.AcquireKLine()
		kline.Symbol = key.Symbol
		kline.Market = models.SZSE
		kline.StartTime = time.Unix(key.Minute, 0)
		kline.EndTime = time.Unix(key.Minute, 0).Add(time.Minute)
		kline.Open = 0
		kline.High = 0
		kline.Low = 0
		kline.Close = 0
		kline.Volume = 0
		kline.IsClosed = true
		return kline
	}

	// 沿用前一根的 close
	kline := models.AcquireKLine()
	kline.Symbol = key.Symbol
	kline.Market = prev.Market
	kline.StartTime = time.Unix(key.Minute, 0)
	kline.EndTime = time.Unix(key.Minute, 0).Add(time.Minute)
	kline.Open = prev.Close
	kline.High = prev.Close
	kline.Low = prev.Close
	kline.Close = prev.Close
	kline.Volume = 0
	kline.IsClosed = true
	return kline
}

// Lock 禁止值拷贝（Go 面试高频）
func (wm *WindowManager) Lock() {}

// Unlock 禁止值拷贝（Go 面试高频）
func (wm *WindowManager) Unlock() {}

// CleanupStaleStates 清理长期不活跃的合约（防止OOM）
func (wm *WindowManager) CleanupStaleStates(ttl time.Duration) int {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for symbol, state := range wm.symbolStates {
		state.mu.RLock()
		inactive := now.Sub(state.lastActivity) > ttl
		state.mu.RUnlock()

		if inactive {
			delete(wm.symbolStates, symbol)
			// 同时清理该 symbol 的所有窗口
			for key := range wm.windows {
				if key.Symbol == symbol {
					delete(wm.windows, key)
				}
			}
			cleaned++
			log.Printf("[WindowManager] Cleaned up stale symbol: %s", symbol)
		}
	}
	return cleaned
}
