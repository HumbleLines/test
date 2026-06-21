package xxl

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// BeatLogger 把“执行器注册成功 / registry success”类日志限流到 interval 一次。
type BeatLogger struct {
	interval time.Duration
	mu       sync.Mutex
	last     time.Time
}

func NewBeatLogger(interval time.Duration) *BeatLogger {
	return &BeatLogger{interval: interval}
}

func (l *BeatLogger) Info(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	// 关键字按库默认输出做两种匹配（中/英）
	if strings.Contains(msg, "执行器注册成功") || strings.Contains(strings.ToLower(msg), "registry") {
		l.mu.Lock()
		defer l.mu.Unlock()
		if time.Since(l.last) < l.interval {
			return // 丢弃本次重复心跳日志
		}
		l.last = time.Now()
	}
	log.Println(msg)
}

func (l *BeatLogger) Error(format string, a ...interface{}) {
	log.Println(fmt.Sprintf(format, a...))
}
