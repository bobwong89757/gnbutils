package log

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap/zapcore"
)

type asyncWriter struct {
	writer    zapcore.WriteSyncer
	ch        chan []byte
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closeOnce sync.Once
	syncCh    chan struct{} // 用于 Sync 操作
	mu        sync.Mutex
	pending   int64 // 待处理的数据计数
	closed    int32 // 原子标记，表示是否已关闭
}

func newAsyncWriter(ws zapcore.WriteSyncer) zapcore.WriteSyncer {
	if ws == nil {
		// 如果 writer 为 nil，返回一个安全的空实现，避免 panic
		return zapcore.AddSync(&nullWriter{})
	}

	ctx, cancel := context.WithCancel(context.Background())
	aw := &asyncWriter{
		writer: ws,
		ch:     make(chan []byte, 10000),
		ctx:    ctx,
		cancel: cancel,
		syncCh: make(chan struct{}, 1),
		closed: 0,
	}

	aw.wg.Add(1)
	go aw.run()
	return aw
}

// nullWriter 是一个安全的空 writer，用于处理 nil writer 的情况
type nullWriter struct{}

func (n *nullWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (a *asyncWriter) Write(p []byte) (int, error) {
	// 检查是否已关闭
	if atomic.LoadInt32(&a.closed) == 1 {
		// 如果已经关闭，直接写入（同步模式），避免向已关闭的 channel 写入导致 panic
		if a.writer != nil {
			return a.writer.Write(p)
		}
		return len(p), nil
	}

	// 复制数据，避免外部修改
	cp := make([]byte, len(p))
	copy(cp, p)

	// 尝试将数据放入 channel
	select {
	case a.ch <- cp:
		// 成功放入 channel，增加 pending 计数
		a.mu.Lock()
		a.pending++
		a.mu.Unlock()
		return len(p), nil
	case <-a.ctx.Done():
		// 如果已经关闭，直接写入（同步模式）
		if a.writer != nil {
			return a.writer.Write(p)
		}
		return len(p), nil
	default:
		// channel 满了，阻塞等待（避免丢失数据）
		select {
		case a.ch <- cp:
			// 成功放入 channel，增加 pending 计数
			a.mu.Lock()
			a.pending++
			a.mu.Unlock()
			return len(p), nil
		case <-a.ctx.Done():
			// 如果已经关闭，直接写入（同步模式）
			if a.writer != nil {
				return a.writer.Write(p)
			}
			return len(p), nil
		}
	}
}

func (a *asyncWriter) Sync() error {
	// 如果已关闭，直接同步底层 writer
	if atomic.LoadInt32(&a.closed) == 1 {
		if a.writer != nil {
			return a.writer.Sync()
		}
		return nil
	}

	// 发送同步信号
	select {
	case a.syncCh <- struct{}{}:
	default:
		// syncCh 已满，说明有同步操作正在进行，等待一下
		time.Sleep(10 * time.Millisecond)
	}

	// 等待所有待处理的数据写入完成
	// 使用超时保护，避免无限等待
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	done := false
	for !done {
		a.mu.Lock()
		pending := a.pending
		a.mu.Unlock()

		if pending == 0 {
			// 所有数据都已处理完成
			done = true
			break
		}

		select {
		case <-timeout.C:
			// 超时，不再等待
			done = true
		case <-ticker.C:
			// 继续检查
		case <-a.ctx.Done():
			// 如果已关闭，不再等待
			done = true
		}
	}

	// 调用底层 writer 的 Sync
	if a.writer != nil {
		return a.writer.Sync()
	}
	return nil
}

func (a *asyncWriter) run() {
	defer a.wg.Done()
	for {
		select {
		case p, ok := <-a.ch:
			if !ok {
				// channel 已关闭，退出
				return
			}
			// 写入数据，忽略错误（日志写入错误通常不应该影响业务逻辑）
			// 如果 writer 为 nil，这里会 panic，但正常情况下不应该发生
			if a.writer != nil {
				_, _ = a.writer.Write(p)
			}
			// 减少待处理计数
			a.mu.Lock()
			if a.pending > 0 {
				a.pending--
			}
			a.mu.Unlock()
		case <-a.syncCh:
			// 同步信号，继续处理（Sync 会通过检查 pending 来等待）
			continue
		case <-a.ctx.Done():
			// 处理剩余的数据
			for {
				select {
				case p, ok := <-a.ch:
					if !ok {
						// channel 已关闭
						return
					}
					if a.writer != nil {
						_, _ = a.writer.Write(p)
					}
					a.mu.Lock()
					if a.pending > 0 {
						a.pending--
					}
					a.mu.Unlock()
				default:
					return
				}
			}
		}
	}
}

// Close 优雅关闭异步写入器，等待所有数据写入完成
func (a *asyncWriter) Close() error {
	var err error
	a.closeOnce.Do(func() {
		// 设置关闭标记，防止新的写入操作
		atomic.StoreInt32(&a.closed, 1)
		// 取消 context，停止接收新数据
		a.cancel()
		// 关闭 channel（这会触发 run() 中的 channel 关闭检查）
		close(a.ch)
		// 等待 goroutine 完成
		a.wg.Wait()
		// 同步底层 writer（如果 writer 为 nil，这里会 panic，但正常情况下不应该发生）
		if a.writer != nil {
			err = a.writer.Sync()
		}
	})
	return err
}
