package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/momorph/cli/internal/logger"
	"github.com/momorph/cli/internal/version"
)

// HTTPClientConfig configures the HTTP client behavior
type HTTPClientConfig struct {
	Timeout        time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
	Debug          bool
	ConnectTimeout time.Duration
}

// DefaultHTTPConfig returns the default HTTP client configuration
func DefaultHTTPConfig() HTTPClientConfig {
	return HTTPClientConfig{
		Timeout:        30 * time.Second,
		MaxRetries:     3,
		RetryBaseDelay: 1 * time.Second,
		Debug:          false,
		ConnectTimeout: 10 * time.Second,
	}
}

// NewHTTPClient creates a new HTTP client with standard configuration
func NewHTTPClient() *http.Client {
	return NewHTTPClientWithConfig(DefaultHTTPConfig())
}

// NewHTTPClientWithConfig creates a new HTTP client with custom configuration
func NewHTTPClientWithConfig(cfg HTTPClientConfig) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   cfg.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// Force HTTPS only by not allowing proxy environment variables for plain HTTP
		ForceAttemptHTTP2: true,
	}

	return &http.Client{
		Timeout: cfg.Timeout,
		Transport: &instrumentedTransport{
			Transport: transport,
			debug:     cfg.Debug,
		},
	}
}

// instrumentedTransport adds User-Agent header and optional debug logging
type instrumentedTransport struct {
	Transport http.RoundTripper
	debug     bool
}

func (t *instrumentedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Generate request ID for tracing
	requestID := generateRequestID()
	req.Header.Set("User-Agent", "MoMorph-CLI/"+version.Version)
	req.Header.Set("X-Request-ID", requestID)

	start := time.Now()

	// Log request in debug mode
	if t.debug {
		t.logRequest(req, requestID)
	}

	resp, err := t.Transport.RoundTrip(req)
	duration := time.Since(start)

	// Log response in debug mode
	if t.debug {
		t.logResponse(resp, err, requestID, duration)
	}

	// Log basic request info (always, for observability)
	if resp != nil {
		logger.Debug("HTTP %s %s → %d (%v)", req.Method, sanitizeURL(req.URL.String()), resp.StatusCode, duration)
	} else if err != nil {
		logger.Debug("HTTP %s %s → ERROR: %v (%v)", req.Method, sanitizeURL(req.URL.String()), err, duration)
	}

	return resp, err
}

// logRequest logs the full HTTP request for debugging
func (t *instrumentedTransport) logRequest(req *http.Request, requestID string) {
	logger.Debug("=== HTTP Request [%s] ===", requestID)
	logger.Debug("%s %s", req.Method, req.URL.String())

	// Log headers (sanitized)
	for key, values := range req.Header {
		sanitizedKey := key
		for _, v := range values {
			if isSensitiveHeader(key) {
				logger.Debug("  %s: [REDACTED]", sanitizedKey)
			} else {
				logger.Debug("  %s: %s", sanitizedKey, v)
			}
		}
	}

	// Log body if present and debug dump is needed
	if req.Body != nil && req.ContentLength > 0 && req.ContentLength < 10240 {
		dump, err := httputil.DumpRequestOut(req, true)
		if err == nil {
			logger.Debug("Request body:\n%s", sanitizeBody(string(dump)))
		}
	}
}

// logResponse logs the full HTTP response for debugging
func (t *instrumentedTransport) logResponse(resp *http.Response, err error, requestID string, duration time.Duration) {
	logger.Debug("=== HTTP Response [%s] (took %v) ===", requestID, duration)

	if err != nil {
		logger.Debug("Error: %v", err)
		return
	}

	if resp == nil {
		logger.Debug("Response is nil")
		return
	}

	logger.Debug("Status: %s", resp.Status)

	// Log headers (sanitized)
	for key, values := range resp.Header {
		for _, v := range values {
			if isSensitiveHeader(key) {
				logger.Debug("  %s: [REDACTED]", key)
			} else {
				logger.Debug("  %s: %s", key, v)
			}
		}
	}
}

// DoWithRetry performs an HTTP request with exponential backoff retry
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int, baseDelay time.Duration) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := calculateBackoff(attempt, baseDelay)
			logger.Debug("Retry attempt %d/%d after %v", attempt, maxRetries, delay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Clone request for retry (body needs to be re-readable)
			req = cloneRequest(req)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			// Check if error is retryable
			if !isRetryableError(err) {
				return nil, wrapNetworkError(err)
			}
			continue
		}

		// Check if status code is retryable
		if isRetryableStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// calculateBackoff calculates exponential backoff with jitter
func calculateBackoff(attempt int, baseDelay time.Duration) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := baseDelay * time.Duration(1<<uint(attempt))

	// Add jitter (±25%)
	jitter := float64(delay) * 0.25 * (rand.Float64()*2 - 1)
	delay = delay + time.Duration(jitter)

	// Cap at 30 seconds
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors
	var netErr net.Error
	if ok := asNetError(err, &netErr); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	// Check for common retryable error strings
	errStr := err.Error()
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"connection timed out",
		"no such host",
		"EOF",
		"broken pipe",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}

	return false
}

// asNetError is a helper to check for net.Error
func asNetError(err error, target *net.Error) bool {
	for err != nil {
		if ne, ok := err.(net.Error); ok {
			*target = ne
			return true
		}
		err = unwrapError(err)
	}
	return false
}

// unwrapError unwraps an error
func unwrapError(err error) error {
	type unwrapper interface {
		Unwrap() error
	}
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}

// isRetryableStatus checks if an HTTP status code is retryable
func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, // 429
		http.StatusServiceUnavailable, // 503
		http.StatusGatewayTimeout,     // 504
		http.StatusBadGateway:         // 502
		return true
	default:
		return false
	}
}

// cloneRequest creates a clone of an HTTP request
func cloneRequest(req *http.Request) *http.Request {
	clone := req.Clone(req.Context())
	if req.Body != nil {
		if body, ok := req.Body.(io.Seeker); ok {
			body.Seek(0, io.SeekStart)
		}
	}
	return clone
}

// wrapNetworkError wraps a network error with a user-friendly message
func wrapNetworkError(err error) error {
	errStr := err.Error()

	if strings.Contains(errStr, "no such host") {
		return fmt.Errorf("unable to resolve host - please check your internet connection: %w", err)
	}
	if strings.Contains(errStr, "connection refused") {
		return fmt.Errorf("connection refused - the server may be down or unreachable: %w", err)
	}
	if strings.Contains(errStr, "connection timed out") || strings.Contains(errStr, "i/o timeout") {
		return fmt.Errorf("connection timed out - please check your internet connection: %w", err)
	}
	if strings.Contains(errStr, "TLS") || strings.Contains(errStr, "certificate") {
		return fmt.Errorf("TLS/SSL error - please ensure HTTPS is properly configured: %w", err)
	}

	return fmt.Errorf("network error: %w", err)
}

// generateRequestID generates a unique request ID for tracing
func generateRequestID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// sanitizeURL removes sensitive query parameters from URLs
func sanitizeURL(url string) string {
	// Remove common sensitive parameters
	sensitiveParams := []string{"token", "key", "secret", "password", "access_token", "api_key"}
	result := url

	for _, param := range sensitiveParams {
		// Match param=value pattern
		patterns := []string{
			param + "=",
		}
		for _, pattern := range patterns {
			if idx := strings.Index(strings.ToLower(result), pattern); idx != -1 {
				// Find the end of the value (next & or end of string)
				endIdx := strings.Index(result[idx:], "&")
				if endIdx == -1 {
					result = result[:idx] + param + "=[REDACTED]"
				} else {
					result = result[:idx] + param + "=[REDACTED]" + result[idx+endIdx:]
				}
			}
		}
	}

	return result
}

// sanitizeBody removes sensitive data from request/response bodies
func sanitizeBody(body string) string {
	// Simple redaction of common sensitive field patterns
	sensitivePatterns := []string{
		`"token":`,
		`"access_token":`,
		`"password":`,
		`"secret":`,
		`"api_key":`,
	}

	result := body
	for _, pattern := range sensitivePatterns {
		if idx := strings.Index(strings.ToLower(result), strings.ToLower(pattern)); idx != -1 {
			// Find the value and redact it
			start := idx + len(pattern)
			// Skip whitespace and opening quote
			for start < len(result) && (result[start] == ' ' || result[start] == '"') {
				start++
			}
			// Find end of value
			end := start
			for end < len(result) && result[end] != '"' && result[end] != ',' && result[end] != '}' {
				end++
			}
			if start < end {
				result = result[:start] + "[REDACTED]" + result[end:]
			}
		}
	}

	return result
}

// isSensitiveHeader checks if a header name is sensitive
func isSensitiveHeader(name string) bool {
	sensitive := []string{
		"authorization",
		"cookie",
		"set-cookie",
		"x-api-key",
		"x-auth-token",
	}

	lower := strings.ToLower(name)
	for _, s := range sensitive {
		if lower == s {
			return true
		}
	}
	return false
}

// ReadResponseBody reads and returns the response body, limiting size
func ReadResponseBody(resp *http.Response, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10MB default
	}

	// Limit reader to prevent memory exhaustion
	limitedReader := io.LimitReader(resp.Body, maxSize)

	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// DrainAndClose drains and closes a response body
func DrainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	// Drain up to 64KB to allow connection reuse
	io.CopyN(io.Discard, body, 64*1024)
	body.Close()
}

// NewRequestWithJSON creates a new request with JSON body
func NewRequestWithJSON(ctx context.Context, method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}
