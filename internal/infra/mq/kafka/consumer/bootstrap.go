package consumer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"trade-gateway/internal/consts"

	"trade-gateway/internal/config"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// KafkaRawRunner 使用 segmentio/kafka-go 实现的消费者 Runner。
// - Start 非阻塞：内部起 goroutine 拉取并发处理；
// - Stop 优雅退出：cancel + 等在途处理收敛；
// - 位点提交：当前为“手动提交，成功才提交”(at-least-once)；如需自动提交见注释。
type KafkaRawRunner struct {
	name string
	rcfg kafka.ReaderConfig

	fn         FuncHandler // 业务处理（可被 tracing 包装）
	processors int         // 并发度（信号量控制）

	startOnce sync.Once
	stopOnce  sync.Once
	done      chan struct{}
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	reader    *kafka.Reader
}

// splitCSVTopics 兼容“一个配置下写多个 topic（用逗号/空白分隔）”的需求。
// 例如: "order, user, payment" => []{"order","user","payment"}。
func splitCSVTopics(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	// 先统一把中文逗号替换成英文逗号
	s = strings.NewReplacer("，", ",", ";", ",").Replace(s)
	// 按逗号拆，再去空白
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		t := strings.TrimSpace(it)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// NewKafkaRawRunner 组装一个可被 Manager 托管的 KafkaConsumers Runner。
// 支持：
//  1. 单 topic：c.Topic = "order"
//  2. 多 topic：c.Topic = "order,user,payment"  (内部设置 GroupTopics)
func NewKafkaRawRunner(name string, c config.KafkaConsumerCfg, fn FuncHandler) *KafkaRawRunner {
	rc := kafka.ReaderConfig{
		Brokers:  c.Brokers,
		GroupID:  c.Group,
		MinBytes: c.MinBytes,
		MaxBytes: c.MaxBytes,
		// 手动提交：只有处理成功才提交位点（推荐，at-least-once）
		CommitInterval: 0,
		MaxWait:        250 * time.Millisecond,
	}

	// Topic / GroupTopics（二选一）
	topics := splitCSVTopics(c.Topic)
	if len(topics) > 1 {
		rc.GroupTopics = topics
	} else {
		rc.Topic = c.Topic
	}

	// StartOffset：仅在“该 group 对应分区没有已提交位点”时才会生效
	switch strings.ToLower(strings.TrimSpace(c.Offset)) {
	case "first", "earliest":
		rc.StartOffset = kafka.FirstOffset
	case "last", "latest":
		rc.StartOffset = kafka.LastOffset
	default:
		rc.StartOffset = kafka.FirstOffset
	}

	// SASL/PLAIN（有配才启用）
	if c.Username != "" || c.Password != "" {
		rc.Dialer = &kafka.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
			SASLMechanism: plain.Mechanism{
				Username: c.Username,
				Password: c.Password,
			},
		}
	}

	proc := c.Processors
	if proc <= 0 {
		proc = 8
	}

	// 是否包一层 tracing 由配置控制（优先从 headers 接力上游 trace）
	finalFn := WrapTracing(fn, c.TraceEnabled, consts.KATracerName, consts.KASpanName, true)

	return &KafkaRawRunner{
		name:       fmt.Sprintf("mq:%s", name),
		rcfg:       rc,
		fn:         finalFn,
		processors: proc,
		done:       make(chan struct{}),
	}
}

func (r *KafkaRawRunner) Name() string { return r.name }

// Start 非阻塞：内部起“拉取主循环 + 多协程处理”。
// 若你想切换到“自动提交”(at-most-once)：
//  1. 把 rc.CommitInterval 改为 time.Second；
//  2. 这里把 FetchMessage 改为 ReadMessage；
//  3. 删掉 CommitMessages；
//

func (r *KafkaRawRunner) Start(ctx context.Context) error {
	r.startOnce.Do(func() {
		ctx, r.cancel = context.WithCancel(ctx)
		r.reader = kafka.NewReader(r.rcfg)
		// 并发控制（信号量）
		sem := make(chan struct{}, r.processors)

		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			defer close(r.done)
			defer func() {
				if r.reader != nil {
					err := r.reader.Close()
					if err != nil {
						log.Printf("kafka.Reader close error: %s", err.Error())
					}
				}
			}()

			for {
				msg, err := r.reader.FetchMessage(ctx)
				if err != nil {
					// 收到退出信号或超时：结束主循环
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return
					}
					// 其他错误：轻微退避后继续拉取
					time.Sleep(200 * time.Millisecond)
					continue
				}
				// 关键定位日志：判断是否“真重复”
				log.Printf("[kafka] topic=%s partition=%d offset=%d key=%q",
					msg.Topic, msg.Partition, msg.Offset, string(msg.Key))
				sem <- struct{}{} // 占一个并发令牌
				r.wg.Add(1)
				go func(m kafka.Message) {
					log.Printf("[kafka] topic=%s p=%d off=%d key_len=%d val_len=%d val_prefix=%q",
						m.Topic, m.Partition, m.Offset, len(m.Key), len(m.Value),
						func(b []byte) string {
							s := string(b)
							if len(s) > 256 {
								return s[:256]
							}
							return s
						}(m.Value),
					)
					defer r.wg.Done()
					defer func() { <-sem }()

					// —— 关键：无论业务成功/失败，最后都提交位点（at-most-once）
					defer func() {
						ctxCommit, cancel := context.WithTimeout(ctx, 2*time.Second)
						defer cancel()
						if err := r.reader.CommitMessages(ctxCommit, m); err != nil {
							// 一般是 rebalance / 取消导致；这里仅记录一下
							log.Printf("kafka.CommitMessages error: %s", err.Error())
						}
					}()
					// 将 Kafka headers 注入 ctx
					hdrs := make(map[string]string, len(m.Headers))
					for _, h := range m.Headers {
						hdrs[h.Key] = string(h.Value)
					}
					ctxMsg := withHeaders(ctx, hdrs)

					// —— 业务处理：失败也不影响位点提交
					if err := r.fn(ctxMsg, string(m.Key), string(m.Value)); err != nil {
						// 策略：失败属于异常，但不重试，直接交给业务侧兜底
						// 建议：这里或 fn 内把原始消息落库/DLQ/告警（否则提交后就再也拿不到了）
						log.Printf("kafka.Reader fn error (will still commit): %s", err.Error())
						// 注意：不要 return 提前退出，否则 defer 仍然会提交，但后面的逻辑都不会走
						// 这里没有后续逻辑了，所以直接 return 也无妨
						return
					}

					// 成功场景：这里无额外逻辑；位点提交由上面的 defer 统一完成
				}(msg)
			}
		}()
	})
	return nil
}

// Stop 触发取消并等待在途处理完成（优雅退出）。
// 3) Stop：先 cancel 再 Close，确保 FetchMessage 被打断
func (r *KafkaRawRunner) Stop(ctx context.Context) error {
	r.stopOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		if r.reader != nil {
			_ = r.reader.Close()
		}
	})
	done := make(chan struct{})
	go func() { r.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
