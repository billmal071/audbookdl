package downloader

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}

	// attempt 0 should give ~1s (750ms-1250ms with 25% jitter)
	d := CalculateBackoff(0, cfg)
	if d < 750*time.Millisecond || d > 1250*time.Millisecond {
		t.Errorf("attempt 0: expected 750ms-1250ms, got %v", d)
	}

	// high attempt should not exceed MaxDelay + 25%
	maxAllowed := time.Duration(float64(cfg.MaxDelay) * 1.25)
	for attempt := 10; attempt <= 20; attempt++ {
		d := CalculateBackoff(attempt, cfg)
		if d > maxAllowed {
			t.Errorf("attempt %d: expected <= %v, got %v", attempt, maxAllowed, d)
		}
	}
}

func TestRetryOperation_Success(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	}

	calls := 0
	err := RetryOperation(context.Background(), cfg, func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryOperation_EventualSuccess(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	}

	calls := 0
	err := RetryOperation(context.Background(), cfg, func() error {
		calls++
		if calls < 3 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOperation_AllFail(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	}

	sentinel := errors.New("always fails")
	calls := 0
	err := RetryOperation(context.Background(), cfg, func() error {
		calls++
		return sentinel
	})

	if err == nil {
		t.Error("expected an error, got nil")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOperation_ContextCancelled(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := RetryOperation(ctx, cfg, func() error {
		return errors.New("should not matter")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
