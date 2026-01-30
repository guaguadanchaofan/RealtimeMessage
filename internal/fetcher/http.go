package fetcher

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	retryOn    map[int]bool
	maxAttempts int
	backoffMS int
	multiplier float64
	jitterMS int
}

func New(timeout time.Duration, retryOnStatus []int, maxAttempts, backoffMS int, multiplier float64, jitterMS int) *Client {
	m := make(map[int]bool)
	for _, s := range retryOnStatus {
		m[s] = true
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	if backoffMS <= 0 {
		backoffMS = 200
	}
	if multiplier <= 0 {
		multiplier = 2
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		retryOn:    m,
		maxAttempts: maxAttempts,
		backoffMS: backoffMS,
		multiplier: multiplier,
		jitterMS: jitterMS,
	}
}

func (c *Client) Do(ctx context.Context, req *http.Request) (int, []byte, error) {
	var lastErr error
	backoff := time.Duration(c.backoffMS) * time.Millisecond
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		resp, err := c.httpClient.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp.StatusCode, body, nil
			} else if c.retryOn[resp.StatusCode] {
				lastErr = errors.New(resp.Status)
			} else {
				return resp.StatusCode, body, errors.New(resp.Status)
			}
		}
		if attempt < c.maxAttempts {
			jitter := time.Duration(rand.Intn(c.jitterMS+1)) * time.Millisecond
			time.Sleep(backoff + jitter)
			backoff = time.Duration(float64(backoff) * c.multiplier)
		}
	}
	return 0, nil, lastErr
}
