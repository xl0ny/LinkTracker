package resilience

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/require"
)

var breakerSuccess = struct{}{}

func testConfig() Config {
	return Config{
		HTTP: HTTPConfig{Timeout: 100 * time.Millisecond},
		Retry: RetryConfig{
			MaxAttempts:       3,
			Delay:             50 * time.Millisecond,
			RetryableStatuses: []int{500, 502, 503, 504},
		},
		CircuitBreaker: CircuitBreakerConfig{
			SlidingWindowSize:             10,
			MinimumRequiredCalls:          5,
			FailureRateThreshold:          100,
			PermittedCallsInHalfOpenState: 3,
			WaitDurationInOpenState:       100 * time.Millisecond,
		},
	}
}

func TestHTTP_timeout(t *testing.T) {
	delay := 300 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.HTTP.Timeout = 50 * time.Millisecond
	client := NewHTTPClient("timeout-test", cfg)

	start := time.Now()
	_, err := client.Get(srv.URL)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, delay)
}

func TestHTTP_retryOn5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.HTTP.Timeout = 2 * time.Second
	client := NewHTTPClient("retry-test", cfg)

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
	require.Equal(t, int32(3), calls.Load())
}

func TestHTTP_noRetryOn4xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	client := NewHTTPClient("no-retry-test", cfg)

	resp, err := client.Get(srv.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	_ = resp.Body.Close()
	require.Equal(t, int32(1), calls.Load())
}

func TestHTTP_retryConstantBackoff(t *testing.T) {
	var calls atomic.Int32
	var prev time.Time
	var gaps []time.Duration
	backoff := 80 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now()
		if calls.Add(1) > 1 && !prev.IsZero() {
			gaps = append(gaps, now.Sub(prev))
		}
		prev = now
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.HTTP.Timeout = 2 * time.Second
	cfg.Retry.Delay = backoff
	cfg.Retry.MaxAttempts = 3
	client := NewHTTPClient("backoff-test", cfg)

	_, err := client.Get(srv.URL)
	require.Error(t, err)
	require.Equal(t, int32(3), calls.Load())
	for _, gap := range gaps {
		require.GreaterOrEqual(t, gap, backoff-time.Millisecond*15)
	}
}

func TestHTTP_retryExponentialBackoff(t *testing.T) {
	var calls atomic.Int32
	var prev time.Time
	var gaps []time.Duration
	base := 60 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now()
		if calls.Add(1) > 1 && !prev.IsZero() {
			gaps = append(gaps, now.Sub(prev))
		}
		prev = now
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.HTTP.Timeout = 2 * time.Second
	cfg.Retry.Delay = base
	cfg.Retry.MaxAttempts = 4
	cfg.Retry.Backoff = "exponential"
	cfg.Retry.MaxDelay = 0
	client := NewHTTPClient("exp-backoff-test", cfg)

	_, err := client.Get(srv.URL)
	require.Error(t, err)
	require.Equal(t, int32(4), calls.Load())
	require.GreaterOrEqual(t, len(gaps), 2)
	require.Greater(t, gaps[1], gaps[0])
	require.GreaterOrEqual(t, gaps[1], base*2-15*time.Millisecond)
}

func TestHTTP_exponentialBackoffMaxDelay(t *testing.T) {
	var calls atomic.Int32
	var prev time.Time
	var gaps []time.Duration
	base := 40 * time.Millisecond
	maxDelay := 50 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		now := time.Now()
		if calls.Add(1) > 1 && !prev.IsZero() {
			gaps = append(gaps, now.Sub(prev))
		}
		prev = now
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.HTTP.Timeout = 2 * time.Second
	cfg.Retry.Delay = base
	cfg.Retry.MaxDelay = maxDelay
	cfg.Retry.MaxAttempts = 4
	cfg.Retry.Backoff = "exponential"
	client := NewHTTPClient("exp-cap-test", cfg)

	_, err := client.Get(srv.URL)
	require.Error(t, err)
	for _, gap := range gaps {
		require.LessOrEqual(t, gap, maxDelay+30*time.Millisecond)
	}
}

func TestRateLimit_disabledWhenNegative(t *testing.T) {
	mw := RateLimitMiddleware(RateLimitConfig{RPS: -1, Burst: -1})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := &http.Client{Timeout: time.Second}
	for range 5 {
		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		_ = resp.Body.Close()
	}
}

func TestRateLimit_exceedsLimit(t *testing.T) {
	cfg := RateLimitConfig{RPS: 2, Burst: 2}
	mw := RateLimitMiddleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := &http.Client{Timeout: time.Second}
	var ok, limited int
	for range 5 {
		resp, err := client.Get(srv.URL)
		require.NoError(t, err)
		switch resp.StatusCode {
		case http.StatusOK:
			ok++
		case http.StatusTooManyRequests:
			limited++
		}
		_ = resp.Body.Close()
	}
	require.GreaterOrEqual(t, ok, 2)
	require.GreaterOrEqual(t, limited, 1)
}

func TestCircuitBreaker_openState(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	cfg := testConfig()
	cfg.HTTP.Timeout = time.Second
	cfg.Retry.MaxAttempts = 1
	cfg.CircuitBreaker.MinimumRequiredCalls = 3
	cfg.CircuitBreaker.SlidingWindowSize = 10
	cfg.CircuitBreaker.FailureRateThreshold = 100
	cfg.CircuitBreaker.WaitDurationInOpenState = time.Second

	client := NewHTTPClient("cb-open", cfg)

	for range 5 {
		_, _ = client.Get(srv.URL)
	}
	before := calls.Load()

	start := time.Now()
	_, err := client.Get(srv.URL)
	elapsed := time.Since(start)

	require.Error(t, err)
	require.Less(t, elapsed, 100*time.Millisecond)
	require.Equal(t, before, calls.Load())
}

func tripBreaker(t *testing.T, cb *gobreaker.CircuitBreaker, failures int) {
	t.Helper()
	fail := func() (any, error) { return breakerSuccess, errors.New("fail") }
	for range failures {
		_, _ = cb.Execute(fail)
	}
	require.Equal(t, gobreaker.StateOpen, cb.State())
}

func TestCircuitBreaker_halfOpenToClosed(t *testing.T) {
	cfg := testConfig()
	cfg.CircuitBreaker.MinimumRequiredCalls = 2
	cfg.CircuitBreaker.PermittedCallsInHalfOpenState = 2
	cfg.CircuitBreaker.WaitDurationInOpenState = 50 * time.Millisecond
	cfg.CircuitBreaker.FailureRateThreshold = 100

	cb := CircuitBreaker("half-closed", cfg)
	tripBreaker(t, cb, 3)
	time.Sleep(cfg.CircuitBreaker.WaitDurationInOpenState + 20*time.Millisecond)

	ok := func() (any, error) { return breakerSuccess, nil }
	for range int(cfg.CircuitBreaker.PermittedCallsInHalfOpenState) {
		_, err := cb.Execute(ok)
		require.NoError(t, err)
	}
	require.Equal(t, gobreaker.StateClosed, cb.State())
}

func TestCircuitBreaker_halfOpenToOpen(t *testing.T) {
	cfg := testConfig()
	cfg.CircuitBreaker.MinimumRequiredCalls = 2
	cfg.CircuitBreaker.PermittedCallsInHalfOpenState = 2
	cfg.CircuitBreaker.WaitDurationInOpenState = 50 * time.Millisecond
	cfg.CircuitBreaker.FailureRateThreshold = 100

	cb := CircuitBreaker("half-open", cfg)
	tripBreaker(t, cb, 3)
	time.Sleep(cfg.CircuitBreaker.WaitDurationInOpenState + 20*time.Millisecond)

	fail := func() (any, error) { return breakerSuccess, errors.New("fail") }
	for range int(cfg.CircuitBreaker.PermittedCallsInHalfOpenState) {
		_, _ = cb.Execute(fail)
	}
	require.Equal(t, gobreaker.StateOpen, cb.State())
}

func TestCircuitBreaker_openFailsFast(t *testing.T) {
	cfg := testConfig()
	cfg.CircuitBreaker.MinimumRequiredCalls = 2

	cb := CircuitBreaker("fast", cfg)
	tripBreaker(t, cb, 3)

	var ran atomic.Bool
	start := time.Now()
	_, err := cb.Execute(func() (any, error) {
		ran.Store(true)
		time.Sleep(500 * time.Millisecond)
		return breakerSuccess, nil
	})
	elapsed := time.Since(start)

	require.Error(t, err)
	require.False(t, ran.Load())
	require.Less(t, elapsed, 50*time.Millisecond)
}
