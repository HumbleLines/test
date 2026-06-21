package publisher

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
)

// TextMapCarrier，把注入结果落在 map 上
type mapCarrier map[string]string

func (m mapCarrier) Get(key string) string { return m[key] }
func (m mapCarrier) Set(key, val string)   { m[key] = val }
func (m mapCarrier) Keys() []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// Publish 带 trace 注入的通用发布
func Publish(ctx context.Context, w *kafka.Writer, key string, value string, extra map[string]string) error {
	// 注入 trace 到 map
	carr := mapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carr)

	// 合并用户自定义 headers
	for k, v := range extra {
		carr[k] = v
	}

	//  转成 kafka headers
	headers := make([]kafka.Header, 0, len(carr))
	for k, v := range carr {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}

	// 写消息（默认写入 writer.Topic；也可在 Message 里覆写 Topic 字段）
	return w.WriteMessages(ctx, kafka.Message{
		Key:     []byte(key),
		Value:   []byte(value),
		Headers: headers,
	})
}
