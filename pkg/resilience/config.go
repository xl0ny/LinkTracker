package resilience

import "time"

type Config struct {
	HTTP           HTTPConfig           `yaml:"http"`
	Retry          RetryConfig          `yaml:"retry"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	RateLimit      RateLimitConfig      `yaml:"rate_limit"`
}

type HTTPConfig struct {
	Timeout time.Duration `yaml:"timeout"`
}

type RetryConfig struct {
	MaxAttempts       uint          `yaml:"max_attempts"`
	Delay             time.Duration `yaml:"delay"`
	MaxDelay          time.Duration `yaml:"max_delay"`
	Backoff           string        `yaml:"backoff"`
	RetryableStatuses []int         `yaml:"retryable_statuses"`
}

type CircuitBreakerConfig struct {
	SlidingWindowSize             uint32        `yaml:"sliding_window_size"`
	MinimumRequiredCalls          uint32        `yaml:"minimum_required_calls"`
	FailureRateThreshold          float64       `yaml:"failure_rate_threshold"`
	PermittedCallsInHalfOpenState uint32        `yaml:"permitted_calls_in_half_open_state"`
	WaitDurationInOpenState       time.Duration `yaml:"wait_duration_in_open_state"`
}

type RateLimitConfig struct {
	RPS   float64 `yaml:"rps"`
	Burst int     `yaml:"burst"`
}
