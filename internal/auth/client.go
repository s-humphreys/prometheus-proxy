package auth

import (
	"context"
	"errors"

	"github.com/s-humphreys/prometheus-proxy/internal/logger"
)

var errEmptyToken = errors.New("empty authentication token")

type ClientHeader struct {
	Key   string
	Value string
}

type Client interface {
	InitClient() error
	SetLogger(logger *logger.Logger)
	AcquireToken(ctx context.Context) (string, error)
	GetHeaders(ctx context.Context) ([]ClientHeader, error)
}
