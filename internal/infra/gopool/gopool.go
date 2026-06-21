package gopool

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"trade-gateway/internal/infra/notifier"
)

type Pool interface {
	Go(fn func())
	Wait()
}

type Option struct {
	Name          string
	MaxGoroutines int
	Notifier      notifier.Notifier
}

type pool struct {
	name     string
	limitCh  chan struct{}
	wg       sync.WaitGroup
	blocked  atomic.Bool
	notifier notifier.Notifier
}

func New(opt Option) Pool {
	if opt.MaxGoroutines <= 0 {
		opt.MaxGoroutines = 1
	}
	return &pool{
		name:     opt.Name,
		limitCh:  make(chan struct{}, opt.MaxGoroutines),
		notifier: opt.Notifier,
	}
}

func (p *pool) Go(fn func()) {
	blockedPrinted := false
	select {
	case p.limitCh <- struct{}{}:
	default:
		if p.blocked.CompareAndSwap(false, true) {
			fmt.Printf("[gopool %s] full, waiting...\n", p.name)
			blockedPrinted = true
		}
		p.limitCh <- struct{}{}
		fmt.Printf("[gopool %s] resumed slot\n", p.name)
	}
	if blockedPrinted {
		p.blocked.Store(false)
	}

	p.wg.Add(1)
	go func() {
		defer func() {
			<-p.limitCh
			p.wg.Done()
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				// 最小日志 + 可选通知
				fmt.Printf("[gopool %s] panic: %v\n%s\n", p.name, r, stack)
				if p.notifier != nil {
					_ = p.notifier.Send(context.Background(),
						fmt.Sprintf("[gopool %s] panic: %v\n\n```\n%s\n```", p.name, r, stack))
				}
			}
		}()
		fn()
	}()
}

func (p *pool) Wait() { p.wg.Wait() }
