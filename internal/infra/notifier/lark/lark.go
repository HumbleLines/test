package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"trade-gateway/internal/infra/notifier"
)

type Lark struct {
	webhook string
	client  *http.Client
}

func New(webhook string, timeout time.Duration) notifier.Notifier {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Lark{
		webhook: webhook,
		client:  &http.Client{Timeout: timeout},
	}
}

func (l *Lark) Send(ctx context.Context, msg string) error {
	if l.webhook == "" {
		return nil
	}
	body, _ := json.Marshal(map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": msg},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := l.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
