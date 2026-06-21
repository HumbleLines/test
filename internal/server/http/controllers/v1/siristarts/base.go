package siristarts

import (
	serviceset "trade-gateway/internal/bootstrap/wiring"
)

type Handlers struct {
	S *serviceset.Set
}

func NewHandlers(s *serviceset.Set) *Handlers { return &Handlers{S: s} }
