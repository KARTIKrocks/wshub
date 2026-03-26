package wshub

import (
	"context"
	"sync"
	"sync/atomic"
)

// broadcastTask is a unit of work dispatched to the worker pool.
// Each task represents a batch of clients that need to receive a sendItem.
type broadcastTask struct {
	hub     *Hub
	clients []*Client
	item    sendItem
	ctx     context.Context // nil for non-context sends
	result  *contextResult  // nil for non-context sends
	wg      *sync.WaitGroup // caller's WaitGroup — Done() called after batch completes
}

// contextResult collects the first context error across concurrent batches.
type contextResult struct {
	mu  sync.Mutex
	err error
}

// workerPool is a fixed set of long-lived goroutines that process broadcast
// tasks. It replaces the per-broadcast goroutine spawning in parallelSend,
// eliminating goroutine creation/teardown overhead on every broadcast call.
type workerPool struct {
	tasks     chan broadcastTask
	wg        sync.WaitGroup // tracks worker goroutines for clean shutdown
	stopped   atomic.Bool
	closeOnce sync.Once
}

// newWorkerPool creates and starts a pool of numWorkers goroutines.
// Workers pull tasks from a shared channel and process them until the
// channel is closed via shutdown().
func newWorkerPool(numWorkers int) *workerPool {
	p := &workerPool{
		tasks: make(chan broadcastTask, numWorkers*4),
	}
	p.wg.Add(numWorkers)
	for range numWorkers {
		go p.runWorker()
	}
	return p
}

// runWorker is the main loop for each worker goroutine.
// It processes tasks until the tasks channel is closed.
func (p *workerPool) runWorker() {
	defer p.wg.Done()
	for task := range p.tasks {
		task.execute()
	}
}

// shutdown closes the tasks channel and waits for all workers to drain
// remaining tasks and exit. Safe to call multiple times.
func (p *workerPool) shutdown() {
	p.stopped.Store(true)
	p.closeOnce.Do(func() {
		close(p.tasks)
	})
	p.wg.Wait()
}

// submit sends a task to the pool. Returns false if the pool is shut down,
// allowing callers to fall back to inline execution.
func (p *workerPool) submit(task broadcastTask) (ok bool) {
	if p.stopped.Load() {
		return false
	}
	// Guard against the TOCTOU window between the stopped check and the
	// channel send — shutdown could close the channel in between.
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	p.tasks <- task
	return true
}

// execute processes a single broadcast task — sending the item to every
// client in the batch. It supports two paths: a fast path without context
// checking, and a context-aware path that stops early on cancellation.
func (t broadcastTask) execute() {
	defer t.wg.Done()
	if t.ctx != nil {
		for _, client := range t.clients {
			if !t.hub.trySendWithContext(t.ctx, client, t.item) {
				t.result.mu.Lock()
				if t.result.err == nil {
					t.result.err = t.ctx.Err()
				}
				t.result.mu.Unlock()
				return
			}
		}
	} else {
		for _, client := range t.clients {
			t.hub.trySend(client, t.item)
		}
	}
}
