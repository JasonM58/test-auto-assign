package autoassign

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// =====================
// DEFAULTS
// =====================

const (
	defaultQueryTimeout        = 10 * time.Second
	defaultMaxRetries          = 3
	defaultRetryBaseDelay      = 500 * time.Millisecond
	defaultMaxIdleConns        = 10
	defaultIdleConnTimeout     = 90 * time.Second
	defaultTLSHandshakeTimeout = 10 * time.Second
	defaultDialTimeout         = 5 * time.Second
	defaultKeepAlive           = 30 * time.Second
)

// =====================
// STRUCT
// =====================

type PrometheusMetrics struct {
	BaseURL string
	Client  *http.Client

	// Optional production-tuning fields.
	// Zero values fall back to sensible defaults.
	QueryTimeout   time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
}

// NewPrometheusMetrics creates a PrometheusMetrics with a production-grade
// HTTP client. If client is nil a default transport with connection pooling,
// timeouts, and keep-alive is created.
func NewPrometheusMetrics(baseURL string, client *http.Client) *PrometheusMetrics {
	if client == nil {
		client = &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   defaultDialTimeout,
					KeepAlive: defaultKeepAlive,
				}).DialContext,
				MaxIdleConns:        defaultMaxIdleConns,
				MaxIdleConnsPerHost: defaultMaxIdleConns,
				IdleConnTimeout:     defaultIdleConnTimeout,
				TLSHandshakeTimeout: defaultTLSHandshakeTimeout,
			},
			// Per-request timeouts are set via context; this is a safety net.
			Timeout: 30 * time.Second,
		}
	}

	return &PrometheusMetrics{
		BaseURL:        baseURL,
		Client:         client,
		QueryTimeout:   defaultQueryTimeout,
		MaxRetries:     defaultMaxRetries,
		RetryBaseDelay: defaultRetryBaseDelay,
	}
}

// =====================
// INTERNAL RESPONSE STRUCT
// =====================

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

// =====================
// PUBLIC METHODS (IMPLEMENT INTERFACE)
// =====================

// GetReviewWorkload returns reviewer workload based on the pr_reviewers_load metric.
func (p *PrometheusMetrics) GetReviewWorkload(ctx context.Context) (map[string]int, error) {
	query := `sum by (reviewer)(last_over_time(pr_reviewers_load[30m]))`
	return p.query(ctx, query)
}

// GetRecentReviewCount returns the recent review count per reviewer.
func (p *PrometheusMetrics) GetRecentReviewCount(ctx context.Context) (map[string]int, error) {
	query := `sum by (reviewer)(increase(pr_reviews_total[7d]))`
	return p.query(ctx, query)
}

// =====================
// HEALTH CHECK
// =====================

// Ping verifies that the Prometheus / VictoriaMetrics backend is reachable
// and healthy. Useful as a readiness probe.
func (p *PrometheusMetrics) Ping(ctx context.Context) error {
	if p.BaseURL == "" {
		return fmt.Errorf("prometheus base URL is not configured")
	}

	timeout := p.queryTimeout()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	endpoint := fmt.Sprintf("%s/api/v1/query?query=up", p.BaseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("prometheus ping: create request failed: %w", err)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return fmt.Errorf("prometheus ping: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus ping: unexpected status %d", resp.StatusCode)
	}

	return nil
}

func (p *PrometheusMetrics) queryTimeout() time.Duration {
	if p.QueryTimeout > 0 {
		return p.QueryTimeout
	}
	return defaultQueryTimeout
}

func (p *PrometheusMetrics) maxRetries() int {
	if p.MaxRetries > 0 {
		return p.MaxRetries
	}
	return defaultMaxRetries
}

func (p *PrometheusMetrics) retryBaseDelay() time.Duration {
	if p.RetryBaseDelay > 0 {
		return p.RetryBaseDelay
	}
	return defaultRetryBaseDelay
}

// isRetryable returns true for status codes where a retry is reasonable.
func isRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (p *PrometheusMetrics) query(ctx context.Context, promQL string) (map[string]int, error) {
	if p.BaseURL == "" {
		return nil, fmt.Errorf("prometheus base URL is not configured")
	}

	endpoint := fmt.Sprintf(
		"%s/api/v1/query?query=%s",
		p.BaseURL,
		url.QueryEscape(promQL),
	)

	timeout := p.queryTimeout()
	maxAttempts := p.maxRetries()
	baseDelay := p.retryBaseDelay()

	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: baseDelay * 2^(attempt-1) with jitter cap
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}

			slog.Warn("[PROMETHEUS] retrying query",
				slog.Int("attempt", attempt+1),
				slog.Duration("backoff", delay),
				slog.String("query", promQL),
			)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		attemptCtx, cancel := context.WithTimeout(ctx, timeout)

		result, err := p.doQuery(attemptCtx, endpoint)
		cancel()

		if err == nil {
			if attempt > 0 {
				slog.Info("[PROMETHEUS] query succeeded after retry",
					slog.Int("attempt", attempt+1),
					slog.String("query", promQL),
				)
			}
			return result, nil
		}

		lastErr = err

		// Only retry on retryable errors; break immediately on non-retryable ones.
		if !isRetryableError(err) {
			break
		}

		slog.Warn("[PROMETHEUS] query attempt failed",
			slog.Int("attempt", attempt+1),
			slog.String("error", err.Error()),
		)
	}

	return nil, fmt.Errorf("prometheus query failed after %d attempt(s): %w", maxAttempts, lastErr)
}

// retryableError wraps an error that is safe to retry.
type retryableError struct {
	err error
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func isRetryableError(err error) bool {
	_, ok := err.(*retryableError)
	return ok
}

func (p *PrometheusMetrics) doQuery(ctx context.Context, endpoint string) (map[string]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		// Network errors are generally retryable (timeouts, connection refused, etc.)
		return nil, &retryableError{err: fmt.Errorf("request failed: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if isRetryable(resp.StatusCode) {
			return nil, &retryableError{
				err: fmt.Errorf("prometheus returned retryable status %d", resp.StatusCode),
			}
		}
		return nil, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	var result promResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	if result.Status != "" && result.Status != "success" {
		return nil, fmt.Errorf("prometheus query error: type=%s msg=%s", result.ErrorType, result.Error)
	}

	out := make(map[string]int)

	for _, r := range result.Data.Result {

		// get the reviewer label
		reviewer, ok := r.Metric["reviewer"]
		if !ok {
			continue
		}

		// get value (string)
		if len(r.Value) < 2 {
			continue
		}
		valStr, ok := r.Value[1].(string)
		if !ok {
			continue
		}

		// convert to float → int
		valFloat, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		out[reviewer] = int(valFloat + 0.5)
	}

	fmt.Printf("[PROMETHEUS] query=%s result=%v\n", endpoint, out)

	return out, nil
}
