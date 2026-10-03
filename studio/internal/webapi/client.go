// Package webapi is a small, polite JSON/download client shared by the card-database importers: identifying
// User-Agent, a minimum gap between requests, and backoff on network errors, 429 and 5xx.
package webapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const userAgent = "TCGStudio/0.1 (TCG Card Shop Simulator custom cards mod)"

type Client struct {
	name string // shown in errors ("ygoprodeck: HTTP 404 …")
	gap  time.Duration
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// New returns a client that waits at least gap between requests (0 = no spacing).
func New(name string, gap time.Duration) *Client {
	return &Client{name: name, gap: gap, http: &http.Client{Timeout: 90 * time.Second}}
}

func (c *Client) GetJSON(ctx context.Context, u string, out any) error {
	resp, err := c.do(ctx, u, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: bad response from %s: %w", c.name, u, err)
	}
	return nil
}

func (c *Client) Download(ctx context.Context, u string) ([]byte, error) {
	resp, err := c.do(ctx, u, "*/*")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c *Client) do(ctx context.Context, u, accept string) (*http.Response, error) {
	backoff := time.Second
	for attempt := 0; ; attempt++ {
		c.wait()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", accept)
		resp, err := c.http.Do(req)
		if err != nil {
			if attempt < 4 && ctx.Err() == nil {
				sleep(ctx, backoff)
				backoff *= 2
				continue
			}
			return nil, err
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		resp.Body.Close()
		if (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500) && attempt < 5 {
			wait := backoff
			if ra, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && ra > 0 {
				wait = time.Duration(ra) * time.Second
			}
			sleep(ctx, wait)
			backoff *= 2
			continue
		}
		return nil, fmt.Errorf("%s: HTTP %d for %s", c.name, resp.StatusCode, u)
	}
}

func (c *Client) wait() {
	if c.gap <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := c.gap - time.Since(c.last); d > 0 {
		time.Sleep(d)
	}
	c.last = time.Now()
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
