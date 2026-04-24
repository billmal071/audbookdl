package downloader

import (
	"context"
	"math/rand"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{MaxAttempts: 5, BaseDelay: 1 * time.Second, MaxDelay: 30 * time.Second, Multiplier: 2.0}
}

func CalculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	delay := float64(cfg.BaseDelay)
	for i := 0; i < attempt; i++ {
		delay *= cfg.Multiplier
	}
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	delay += jitter
	return time.Duration(delay)
}

func RetryOperation(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt < cfg.MaxAttempts-1 {
			backoff := CalculateBackoff(attempt, cfg)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return lastErr
}
