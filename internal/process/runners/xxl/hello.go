package xxl

import (
	"context"
	"time"
	"trade-gateway/internal/bootstrap/worker"
)

// DemoHello 任务逻辑
func DemoHello(app *worker.App) func(ctx context.Context, p string) (string, error) {
	return func(ctx context.Context, p string) (string, error) {
		app.Logger.SugarWithContext(ctx).Infow("xxl demo_hello", "param", p)
		app.Services.Health.DBOK(ctx)
		app.Services.RedisTrace.Incr(ctx, "xxl_demo_hello", 222*time.Second)
		// === 发送一条 KafkaConsumers 消息 ===
		//if w, ok := app.Deps.KafkaProducer("default"); ok && w != nil {
		//	// Writer 在构造时若已固定 Topic，这里 topic 传 "" 即可；否则填具体 topic
		//	err := kafka.Publish(ctx, w, "", `{"hello":"hello"}`, map[string]string{
		//		"param": p,
		//		"at":    time.Now().Format(time.RFC3339),
		//		"from":  "xxl.demo_hello",
		//	})
		//	if err != nil {
		//		app.Logger.SugarWithContext(ctx).Errorw("kafka publish failed",
		//			"key", "xxl_demo_hello", "err", err)
		//	} else {
		//		app.Logger.SugarWithContext(ctx).Infow("kafka publish success")
		//	}
		//} else {
		//	app.Logger.SugarWithContext(ctx).Warnw("no kafka writer",
		//		"name", "default")
		//}

		return "pong", nil
	}
}
