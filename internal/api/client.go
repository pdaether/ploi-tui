package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL   = "https://ploi.io"
	defaultPageSize  = 100
	defaultMaxPages  = 100
	maxRetryBackoff  = 8 * time.Second
	baseRetryBackoff = 400 * time.Millisecond
)

type Option func(*Client)

func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.http = hc }
}

func WithMaxRetries(n int) Option {
	return func(c *Client) { c.maxRetries = n }
}

func WithPageSize(n int) Option {
	return func(c *Client) { c.pageSize = n }
}

func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

func WithSleeper(s func(context.Context, time.Duration) error) Option {
	return func(c *Client) { c.sleep = s }
}

func WithClock(now func() time.Time) Option {
	return func(c *Client) { c.now = now }
}

type Quota struct {
	Limit       int
	Remaining   int
	CoolingDown bool
	CooldownFor time.Duration
}

type RateLimitTracker struct {
	mu            sync.Mutex
	limit         int
	remaining     int
	cooldownUntil time.Time
	now           func() time.Time
}

func newRateLimitTracker(now func() time.Time) *RateLimitTracker {
	return &RateLimitTracker{now: now}
}

func (r *RateLimitTracker) update(resp *http.Response) {
	if resp == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if v := headerInt(resp.Header, "X-RateLimit-Limit"); v != nil {
		r.limit = *v
	}
	if v := headerInt(resp.Header, "X-RateLimit-Remaining"); v != nil {
		r.remaining = *v
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		d := parseRetryAfter(resp.Header.Get("Retry-After"), r.now())
		if d <= 0 {
			d = time.Minute
		}
		r.cooldownUntil = r.now().Add(d)
	}
}

func (r *RateLimitTracker) cooldown() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	d := r.cooldownUntil.Sub(r.now())
	if d < 0 {
		return 0
	}
	return d
}

func (r *RateLimitTracker) snapshot() Quota {
	r.mu.Lock()
	defer r.mu.Unlock()
	q := Quota{Limit: r.limit, Remaining: r.remaining}
	if d := r.cooldownUntil.Sub(r.now()); d > 0 {
		q.CoolingDown = true
		q.CooldownFor = d
	}
	return q
}

func (r *RateLimitTracker) wait(ctx context.Context, sleep func(context.Context, time.Duration) error) error {
	d := r.cooldown()
	if d <= 0 {
		return nil
	}
	return sleep(ctx, d)
}

type Client struct {
	baseURL    string
	token      string
	http       *http.Client
	userAgent  string
	pageSize   int
	maxRetries int
	sleep      func(context.Context, time.Duration) error
	now        func() time.Time
	Rate       *RateLimitTracker
}

func New(token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		token:      token,
		http:       &http.Client{Timeout: 30 * time.Second},
		userAgent:  userAgentDefault,
		pageSize:   defaultPageSize,
		maxRetries: 3,
		sleep:      sleepCtx,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.Rate = newRateLimitTracker(c.now)
	return c
}

const userAgentDefault = "ploi-tui (+https://github.com/pdaether/ploi-tui)"

func BaseURLFromEnv() (string, bool) {
	u := os.Getenv("PLOI_TUI_API_URL")
	return u, u != ""
}

func (c *Client) Quota() Quota { return c.Rate.snapshot() }

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func headerInt(h http.Header, key string) *int {
	v := h.Get(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &n
}

func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	return dec.Decode(v)
}

type request struct {
	method string
	path   string
	query  url.Values
	body   []byte
	out    any
}

func (c *Client) do(ctx context.Context, req request) error {
	if err := c.Rate.wait(ctx, c.sleep); err != nil {
		return err
	}

	var lastErr error
	backoff := baseRetryBackoff
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		resp, err := c.send(ctx, req)
		if err != nil {
			lastErr = err
		} else {
			c.Rate.update(resp)

			switch {
			case resp.StatusCode == http.StatusTooManyRequests:
				lastErr = decodeAPIError(resp.StatusCode, readBody(resp))
				if err := resp.Body.Close(); err != nil {
					return fmt.Errorf("close response body: %w", err)
				}
				lastErr.(*APIError).RateLimited = true
				lastErr.(*APIError).RetryAfter = c.Rate.cooldown()
				if attempt < c.maxRetries {
					d := maxDuration(backoff, c.Rate.cooldown())
					if serr := c.sleep(ctx, d); serr != nil {
						return serr
					}
					backoff = minDuration(maxRetryBackoff, backoff*2)
					continue
				}
				return lastErr

			case resp.StatusCode == http.StatusUnauthorized:
				readAndClose(resp)
				return ErrUnauthorized

			case resp.StatusCode >= 400:
				apiErr := decodeAPIError(resp.StatusCode, readBody(resp))
				readAndClose(resp)
				if resp.StatusCode >= 500 && attempt < c.maxRetries {
					if serr := c.sleep(ctx, backoff); serr != nil {
						return serr
					}
					backoff = minDuration(maxRetryBackoff, backoff*2)
					continue
				}
				return apiErr

			case req.out != nil:
				body := readBody(resp)
				readAndClose(resp)
				if len(bytes.TrimSpace(body)) == 0 {
					return nil
				}
				if err := json.Unmarshal(body, req.out); err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				return nil

			default:
				readAndClose(resp)
				return nil
			}
		}

		if attempt < c.maxRetries {
			if serr := c.sleep(ctx, backoff); serr != nil {
				return serr
			}
			backoff = minDuration(maxRetryBackoff, backoff*2)
			continue
		}
	}
	return fmt.Errorf("ploi API request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

func maxDuration(a, b time.Duration) time.Duration {
	if a >= b {
		return a
	}
	return b
}

func minDuration(a, b time.Duration) time.Duration {
	if a <= b {
		return a
	}
	return b
}

func (c *Client) send(ctx context.Context, req request) (*http.Response, error) {
	u := c.baseURL + req.path
	if len(req.query) > 0 {
		u += "?" + req.query.Encode()
	}
	var bodyReader io.Reader
	if req.body != nil {
		bodyReader = bytes.NewReader(req.body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", c.userAgent)
	if req.body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(httpReq)
}

func readBody(resp *http.Response) []byte {
	if resp.Body == nil {
		return nil
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return b
}

func readAndClose(resp *http.Response) {
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
	}
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Links struct {
		Next *string `json:"next"`
	} `json:"links"`
	Meta *struct {
		CurrentPage int `json:"current_page"`
		LastPage    int `json:"last_page"`
		Total       int `json:"total"`
	} `json:"meta"`
}

func listPath[T any](c *Client, ctx context.Context, path string, extra url.Values) ([]T, error) {
	var all []T
	q := url.Values{}
	for k, vs := range extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("per_page", strconv.Itoa(c.pageSize))

	requestPath := path
	for page := 1; page <= defaultMaxPages; page++ {
		if q.Get("page") == "" {
			q.Set("page", strconv.Itoa(page))
		}
		var env envelope
		err := c.do(ctx, request{method: http.MethodGet, path: requestPath, query: q, out: &env})
		if err != nil {
			return nil, err
		}
		var items []T
		if len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, &items); err != nil {
				return nil, fmt.Errorf("decode %s: %w", path, err)
			}
		}
		all = append(all, items...)
		if len(items) == 0 {
			return all, nil
		}
		if env.Links.Next != nil && strings.TrimSpace(*env.Links.Next) != "" {
			nextPath, nextQuery, err := nextRequest(*env.Links.Next, requestPath, c.pageSize)
			if err != nil {
				return nil, fmt.Errorf("parse next link %s: %w", path, err)
			}
			requestPath, q = nextPath, nextQuery
			continue
		}
		if env.Meta == nil {
			return all, nil
		}
		currentPage := env.Meta.CurrentPage
		if currentPage < 1 {
			currentPage = page
		}
		if env.Meta.LastPage == 0 || currentPage >= env.Meta.LastPage {
			return all, nil
		}
		q.Set("page", strconv.Itoa(currentPage+1))
	}
	return all, nil
}

func nextRequest(raw, fallbackPath string, pageSize int) (string, url.Values, error) {
	next, err := url.Parse(raw)
	if err != nil {
		return "", nil, err
	}
	path := next.Path
	if path == "" {
		path = fallbackPath
	}
	query := next.Query()
	if query.Get("per_page") == "" {
		query.Set("per_page", strconv.Itoa(pageSize))
	}
	return path, query, nil
}

func (c *Client) User(ctx context.Context) (*User, error) {
	var wrapped struct {
		Data User `json:"data"`
	}
	err := c.do(ctx, request{method: http.MethodGet, path: "/api/user", out: &wrapped})
	if err != nil {
		return nil, err
	}
	return &wrapped.Data, nil
}

func (c *Client) Servers(ctx context.Context) ([]Server, error) {
	return listPath[Server](c, ctx, "/api/servers", nil)
}

func (c *Client) Server(ctx context.Context, id int64) (*Server, error) {
	var wrapped struct {
		Data Server `json:"data"`
	}
	err := c.do(ctx, request{method: http.MethodGet, path: fmt.Sprintf("/api/servers/%d", id), out: &wrapped})
	if err != nil {
		return nil, err
	}
	return &wrapped.Data, nil
}

func (c *Client) RestartServer(ctx context.Context, id int64) error {
	var wrapped struct {
		Message string `json:"message"`
	}
	return c.do(ctx, request{method: http.MethodPost, path: fmt.Sprintf("/api/servers/%d/restart", id), body: []byte("{}"), out: &wrapped})
}

func (c *Client) Monitoring(ctx context.Context, id int64) ([]MonitoringSample, error) {
	var wrapped struct {
		Data []MonitoringSample `json:"data"`
	}
	err := c.do(ctx, request{method: http.MethodGet, path: fmt.Sprintf("/api/servers/%d/monitor", id), out: &wrapped})
	if err != nil {
		return nil, err
	}
	return wrapped.Data, nil
}

func (c *Client) Databases(ctx context.Context, serverID int64) ([]Database, error) {
	return listPath[Database](c, ctx, fmt.Sprintf("/api/servers/%d/databases", serverID), nil)
}

func (c *Client) SystemUsers(ctx context.Context, serverID int64) ([]SystemUser, error) {
	return listPath[SystemUser](c, ctx, fmt.Sprintf("/api/servers/%d/system-users", serverID), nil)
}

func (c *Client) Sites(ctx context.Context, serverID int64) ([]Site, error) {
	return listPath[Site](c, ctx, fmt.Sprintf("/api/servers/%d/sites", serverID), nil)
}

func (c *Client) Site(ctx context.Context, serverID, siteID int64) (*Site, error) {
	var wrapped struct {
		Data Site `json:"data"`
	}
	path := fmt.Sprintf("/api/servers/%d/sites/%d", serverID, siteID)
	err := c.do(ctx, request{method: http.MethodGet, path: path, out: &wrapped})
	if err != nil {
		return nil, err
	}
	return &wrapped.Data, nil
}

func (c *Client) Certificates(ctx context.Context, serverID, siteID int64) ([]Certificate, error) {
	path := fmt.Sprintf("/api/servers/%d/sites/%d/certificates", serverID, siteID)
	return listPath[Certificate](c, ctx, path, nil)
}
