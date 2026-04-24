package pipeline

import (
	"container/list"
	"log"
	"sync"
	"time"
	"trading_system/internal/metrics"
	"trading_system/internal/models"
)

// DedupeTickKey 去重唯一键：交易系统标准唯一标识
type DedupeTickKey struct {
	Symbol           string
	Market           models.Market
	SeqNo            int64
	ExchTimeUnixNano int64
}

// DedupeEntry 缓存条目
type DedupeEntry struct {
	key        DedupeTickKey
	tick       *models.Tick
	createTime int64
	isConflict bool
}

// Deduper 去重器（LRU淘汰）
type Deduper struct {
	mu       sync.RWMutex // 读写锁：读多写少场景性能更高
	capacity int          // LRU容量
	cache    map[DedupeTickKey]*list.Element
	lruList  *list.List

	metrics struct {
		sync.Mutex
		total    int64 // 总处理次数
		dup      int64 // 重复次数
		conflict int64 // 冲突次数
		evicted  int64 // LRU淘汰次数
	}
}

// NewDeduper 创建去重器：自动容错非法容量
func NewDeduper(capacity int) *Deduper {
	if capacity <= 0 {
		capacity = 100000 // 默认10万，防止OOM
	}
	return &Deduper{
		capacity: capacity,
		cache:    make(map[DedupeTickKey]*list.Element),
		lruList:  list.New(),
	}
}

// CheckAndResolve 核心接口
// 返回：isDuplicate 是否重复（不代表丢弃）, finalTick 最终使用的tick
func (d *Deduper) CheckAndResolve(tick *models.Tick) (bool, *models.Tick) {

	if tick == nil {
		log.Printf("[去重] 错误：输入tick为空")
		return true, nil
	}

	key := DedupeTickKey{
		Symbol:           tick.Symbol,
		Market:           tick.Market,
		SeqNo:            tick.SeqNo,
		ExchTimeUnixNano: tick.ExchangeTime.UnixNano(),
	}

	d.mu.RLock()
	ele, exists := d.cache[key]
	d.mu.RUnlock()

	d.metrics.Lock()
	d.metrics.total++
	d.metrics.Unlock()

	metrics.DedupeTotal.Inc()

	// 已存在：重复数据
	if exists {
		d.mu.Lock()
		// 二次确认（防止锁间隙并发问题）
		ele, exists = d.cache[key]
		if !exists {
			d.mu.Unlock()
			return d.CheckAndResolve(tick)
		}

		d.lruList.MoveToFront(ele)
		entry := ele.Value.(*DedupeEntry)

		hasConflict := false
		if entry.tick.Price != tick.Price || entry.tick.Volume != tick.Volume {
			hasConflict = true
		}

		if hasConflict {
			d.metrics.Lock()
			d.metrics.conflict++
			d.metrics.Unlock()

			metrics.DedupeConflictTotal.Inc()

			log.Printf(
				"[去重冲突] market=%s symbol=%s seq=%d time=%d | 旧价=%.2f 旧量=%d | 新价=%.2f 新量=%d | 策略：新数据覆盖",
				tick.Market, tick.Symbol, tick.SeqNo, tick.ExchangeTime.UnixNano(),
				entry.tick.Price, entry.tick.Volume,
				tick.Price, tick.Volume,
			)
			entry.isConflict = true
		}

		// 策略：后到优先级更高，覆盖旧数据
		entry.tick = tick
		d.mu.Unlock()

		d.metrics.Lock()
		d.metrics.dup++
		d.metrics.Unlock()

		metrics.DedupeDuplicateTotal.Inc()

		// 正确语义：是重复，但不丢弃，返回最新tick
		return true, tick
	}

	// 不存在：新增数据
	d.mu.Lock()
	// 二次检查
	if ele, exists = d.cache[key]; exists {
		d.lruList.MoveToFront(ele)
		d.mu.Unlock()
		return true, tick
	}

	// 插入LRU
	entry := &DedupeEntry{
		key:        key,
		tick:       tick,
		createTime: time.Now().UnixNano(),
		isConflict: false,
	}
	listEle := d.lruList.PushFront(entry)
	d.cache[key] = listEle

	// LRU 淘汰旧数据
	if d.lruList.Len() > d.capacity {
		back := d.lruList.Back()
		if back != nil {
			oldEntry := back.Value.(*DedupeEntry)
			delete(d.cache, oldEntry.key)
			d.lruList.Remove(back)

			d.metrics.Lock()
			d.metrics.evicted++
			d.metrics.Unlock()

			metrics.DedupeEvictedTotal.Inc()
		}
	}

	d.mu.Unlock()
	return false, tick
}

// 获取监控指标
func (d *Deduper) Stats() (total, dup, conflict, evicted int64) {
	d.metrics.Lock()
	defer d.metrics.Unlock()
	return d.metrics.total, d.metrics.dup, d.metrics.conflict, d.metrics.evicted
}

// 禁止值拷贝（含锁结构体必须禁止）
func (d *Deduper) Lock()            {}
func (d *Deduper) Unlock()          {}
func (d *Deduper) GoString() string { return "pipeline.Deduper(no-copy)" }
