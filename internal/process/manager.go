package process

import (
	"context"

	ilog "trade-gateway/internal/infra/log"
)

type Manager struct {
	runners []Runner
	log     *ilog.Logger
}

func NewManager(log *ilog.Logger) *Manager { return &Manager{log: log} }

// Register 按顺序登记 Runner（后登记的在 StopAll 中会先关闭——LIFO 语义）
func (m *Manager) Register(r Runner) { m.runners = append(m.runners, r) }

// StartAll 并发启动，非阻塞返回
// 说明：这里不需要关心顺序，因为“启动顺序”通常不要求严格先后；
// 真有依赖，建议由各 Runner 自己 Start 时做重试/等待依赖可用。
func (m *Manager) StartAll(ctx context.Context) error {
	for _, r := range m.runners {
		r := r
		if m.log != nil {
			m.log.Sugar().Infow("[process] start", "runner", r.Name())
		}
		go func() { _ = r.Start(ctx) }()
	}
	return nil
}

// StopAll 按注册的逆序（LIFO）依次停止，确保“最后注册的（通常是 Kafka）先停”。
// 说明：采用“顺序、阻塞”的关停策略，可保证严格顺序（避免 Kafka 与 Engine 并行停止导致竞态）。
func (m *Manager) StopAll(ctx context.Context) error {
	for i := len(m.runners) - 1; i >= 0; i-- {
		r := m.runners[i]
		if m.log != nil {
			m.log.Sugar().Infow("[process] stopping", "runner", r.Name())
		}
		if err := r.Stop(ctx); err != nil {
			// 若某个 Runner 超时/失败，这里记录一下，继续关后续 Runner；
			// 你也可以根据需要，遇到错误就直接 return。
			if m.log != nil {
				m.log.Sugar().Warnw("[process] stop error", "runner", r.Name(), "err", err)
			}
		} else if m.log != nil {
			m.log.Sugar().Infow("[process] stopped", "runner", r.Name())
		}
	}
	return nil
}
