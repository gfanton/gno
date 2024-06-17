package services

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
)

type IncrementType int

const (
	IncrementTypeUnknown IncrementType = iota
	IncrementTypeGlobal
	IncrementTypeSession
)

var ErrUnknownIncrementType error = errors.New("unknown increment type")

func NewCount(log *slog.Logger) Count {
	return Count{
		Log: log,
	}
}

type Count struct {
	Log *slog.Logger
}

var count int64

func (cs Count) Increment(ctx context.Context) (counts int64, err error) {
	val := atomic.AddInt64(&count, 1)
	return val, nil
}

func (cs Count) Get(ctx context.Context) (counts int64, err error) {
	val := atomic.LoadInt64(&count)
	return val, nil
}
