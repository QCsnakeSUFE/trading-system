package pipeline

import (
	"context"
	"sync"
	"time"
	"trading_system/internal/metrics"
	"trading_system/internal/models"
)

// DataPipeline 完整的数据处理管道
type DataPipeline struct {
	deduper   *Deduper
	windowMgr *WindowManager
	InChan    chan *models.Tick
	OutChan   chan *models.KLine

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 禁止拷贝
	noCopy sync.Mutex
}

// NewDataPipeline 创建 Pipeline
func NewDataPipeline(ctx context.Context) *DataPipeline {
	ctx, cancel := context.WithCancel(ctx)
	outChan := make(chan *models.KLine, 1000)
	return &DataPipeline{
		deduper:   NewDeduper(100000),                       // LRU 10万
		windowMgr: NewWindowManager(outChan, 3*time.Second), // 允许3秒延迟
		InChan:    make(chan *models.Tick, 10000),
		OutChan:   outChan,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 启动多协程并发处理（面试核心加分点！）
func (p *DataPipeline) Start(workerCount int) {
	for i := 0; i < workerCount; i++ {
		p.wg.Add(1)
		go p.workerLoop(i)
	}
}

// workerLoop 高并发工作协程
func (p *DataPipeline) workerLoop(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case tick, ok := <-p.InChan:
			if !ok {
				return
			}

			// 监控：队列大小
			metrics.PipelineInChanGauge.Set(float64(len(p.InChan)))
			metrics.PipelineTickReceivedTotal.Inc()

			// 1. 去重（无论是否重复，都使用 finalTick）
			isDup, finalTick := p.deduper.CheckAndResolve(tick)
			if isDup {
				// 是重复，但是继续处理（finalTick是最新值）
			}

			// 2. 必传！去重只是覆盖，不是丢弃！
			if finalTick != nil {
				p.windowMgr.AddTick(finalTick)
			}

			// 3. 把 Tick 放回对象池！
			models.ReleaseTick(tick)
		}
	}
}

// Stop 优雅关闭
func (p *DataPipeline) Stop() {
	p.cancel()
	p.wg.Wait()
	close(p.InChan)
	close(p.OutChan)
}

// Lock 禁止拷贝
func (p *DataPipeline) Lock() {}

// Unlock 禁止拷贝
func (p *DataPipeline) Unlock() {}
