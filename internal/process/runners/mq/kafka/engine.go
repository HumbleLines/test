// internal/process/runners/mq/kafka/engine_handler.go
package mq

import (
	"context"
	"trade-gateway/internal/bootstrap/worker"

	"go.uber.org/zap"
)

func Test(app *worker.App) func(ctx context.Context, key, val string) error {

	var slog *zap.SugaredLogger
	if app == nil || app.Logger == nil {
		slog = zap.NewNop().Sugar()
	} else {
		slog = app.Logger.Sugar()
	}

	return func(ctx context.Context, key, val string) error {
		log := slog
		log.Infow("test: incoming raw", "key", key, "val", val)
		return nil
	}
}
