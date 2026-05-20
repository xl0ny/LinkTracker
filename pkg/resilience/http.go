package resilience

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/sony/gobreaker"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	defaultCBInterval  = 10 * time.Second
	failureRatePercent = 100

	backoffConstant    = "constant"
	backoffExponential = "exponential"
)

type retryableStatusError struct {
	code int
}

func (e *retryableStatusError) Error() string {
	return fmt.Sprintf("retryable status %d", e.code)
}

func NewHTTPClient(name string, cfg Config) *http.Client {
	base := http.DefaultTransport
	transport := newTransport(name, cfg, base)
	timeout := cfg.HTTP.Timeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func newTransport(name string, cfg Config, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	rt := http.RoundTripper(retryRoundTripper{base: base, cfg: cfg.Retry})
	if cfg.CircuitBreaker.WaitDurationInOpenState > 0 {
		rt = cbRoundTripper{base: rt, cb: newCircuitBreaker(name, cfg.CircuitBreaker)}
	}
	return rt
}

type retryRoundTripper struct {
	base http.RoundTripper
	cfg  RetryConfig
}

func (t retryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	attempts := t.cfg.MaxAttempts
	if attempts == 0 {
		attempts = 1
	}
	delay := t.cfg.Delay
	retryable := make(map[int]struct{}, len(t.cfg.RetryableStatuses))
	for _, code := range t.cfg.RetryableStatuses {
		retryable[code] = struct{}{}
	}

	opts := []retry.Option{
		retry.Context(req.Context()),
		retry.Attempts(attempts),
		retry.Delay(delay),
		retry.DelayType(delayTypeFor(t.cfg.Backoff)),
		retry.LastErrorOnly(true),
		retry.RetryIf(func(err error) bool {
			var statusErr *retryableStatusError
			if errors.As(err, &statusErr) {
				return true
			}
			return err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
		}),
	}
	if t.cfg.MaxDelay > 0 {
		opts = append(opts, retry.MaxDelay(t.cfg.MaxDelay))
	}

	var resp *http.Response
	err := retry.Do(
		func() error {
			if resp != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				resp = nil
			}
			r, roundErr := t.base.RoundTrip(req)
			if roundErr != nil {
				return fmt.Errorf("round trip: %w", roundErr)
			}
			if _, ok := retryable[r.StatusCode]; ok {
				_ = r.Body.Close()
				return &retryableStatusError{code: r.StatusCode}
			}
			resp = r
			return nil
		},
		opts...,
	)
	if err != nil {
		return nil, fmt.Errorf("retry round trip: %w", err)
	}
	return resp, nil
}

type cbRoundTripper struct {
	base http.RoundTripper
	cb   *gobreaker.CircuitBreaker
}

func (t cbRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	_, err := t.cb.Execute(func() (any, error) {
		r, roundErr := t.base.RoundTrip(req)
		if roundErr != nil {
			if r != nil && r.Body != nil {
				_, _ = io.Copy(io.Discard, r.Body)
				_ = r.Body.Close()
			}
			return nil, fmt.Errorf("round trip: %w", roundErr)
		}
		resp = r
		return struct{}{}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("circuit breaker: %w", err)
	}
	if resp == nil {
		return nil, errors.New("circuit breaker: missing http response")
	}
	return resp, nil
}

func newCircuitBreaker(name string, cfg CircuitBreakerConfig) *gobreaker.CircuitBreaker {
	window := cfg.SlidingWindowSize
	if window == 0 {
		window = 10
	}
	minCalls := cfg.MinimumRequiredCalls
	if minCalls == 0 {
		minCalls = window
	}
	threshold := cfg.FailureRateThreshold
	maxHalfOpen := cfg.PermittedCallsInHalfOpenState
	if maxHalfOpen == 0 {
		maxHalfOpen = 5
	}
	waitOpen := cfg.WaitDurationInOpenState
	if waitOpen <= 0 {
		waitOpen = time.Second
	}

	interval := time.Duration(window) * time.Second
	if interval <= 0 {
		interval = defaultCBInterval
	}

	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: maxHalfOpen,
		Interval:    interval,
		Timeout:     waitOpen,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if counts.Requests < minCalls {
				return false
			}
			rate := float64(counts.TotalFailures) / float64(counts.Requests) * failureRatePercent
			return rate >= threshold
		},
	})
}

func CircuitBreaker(name string, cfg Config) *gobreaker.CircuitBreaker {
	return newCircuitBreaker(name, cfg.CircuitBreaker)
}

func delayTypeFor(name string) retry.DelayTypeFunc {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case backoffExponential:
		return retry.BackOffDelay
	case "", backoffConstant:
		return retry.FixedDelay
	default:
		return retry.FixedDelay
	}
}
