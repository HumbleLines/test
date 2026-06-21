package format

import (
	"strings"
	"time"
)

// ParseDur 把 "30m" 这类字符串转为 time.Duration（非法返回 0）
func ParseDur(s string) time.Duration {
	d, _ := time.ParseDuration(strings.TrimSpace(s))
	return d
}

// DurOrDefault 默认值
func DurOrDefault(v, def time.Duration) time.Duration {
	if v == 0 {
		return def
	}
	return v
}
func IntOrDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
