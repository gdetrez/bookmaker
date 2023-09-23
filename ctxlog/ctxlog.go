package ctxlog

import (
	"context"
	"log"
)

type ctxKey int

const (
	ctxKeyLogger = iota
)

func WithLogger(ctx context.Context, l *log.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger, l)
}

func LoggerFromContext(ctx context.Context) *log.Logger {
	v, ok := ctx.Value(ctxKeyLogger).(*log.Logger)
	if v != nil && ok {
		return v
	}
	return log.Default()
}
