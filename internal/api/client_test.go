package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New("test-token",
		WithBaseURL(srv.URL),
		WithSleeper(func(context.Context, time.Duration) error { return nil }),
	)
	return c
}

func writeResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Error(err)
	}
}

func TestUser(t *testing.T) {
	var gotAuth, gotUA string
	var gotPath string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "59")
		writeResponse(t, w, `{"data":{"name":"Dennis","email":"dennis@example.com","plan":"Pro","created_at":"2018-03-01 00:00:00"}}`)
	}))

	user, err := c.User(context.Background())
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q", gotAuth)
	}
	if !strings.Contains(gotUA, "ploi-tui") {
		t.Errorf("User-Agent header = %q", gotUA)
	}
	if gotPath != "/api/user" {
		t.Errorf("path = %q", gotPath)
	}
	if user.Email != "dennis@example.com" || user.Name != "Dennis" || user.Plan != "Pro" {
		t.Errorf("unexpected user: %+v", user)
	}
	if user.CreatedAt.Year() != 2018 {
		t.Errorf("created_at = %v", user.CreatedAt.Time)
	}
	q := c.Quota()
	if q.Limit != 60 || q.Remaining != 59 {
		t.Errorf("quota = %+v", q)
	}
}

func TestServersPagination(t *testing.T) {
	type pageReq struct{ page string }
	var requests []pageReq
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, pageReq{r.URL.Query().Get("page")})
		if e := r.URL.Query().Get("per_page"); e != "100" {
			t.Errorf("per_page = %q", e)
		}
		switch r.URL.Query().Get("page") {
		case "1":
			writeResponse(t, w, `{"data":[{"id":1,"name":"web-1","ip_address":"139.59.201.10","php_version":8.3,"mysql_version":"8.0","sites_count":12,"status":"Server active"}],"links":{"next":"/api/servers?page=2"},"meta":{"current_page":1,"last_page":2,"total":2}}`)
		case "2":
			writeResponse(t, w, `{"data":[{"id":2,"name":"db-prod","ip_address":"139.59.201.12","sites_count":0,"status":"rebooting"}],"links":{"next":null},"meta":{"current_page":2,"last_page":2,"total":2}}`)
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	}))

	servers, err := c.Servers(context.Background())
	if err != nil {
		t.Fatalf("Servers: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("made %d requests, want 2 (%v)", len(requests), requests)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if servers[0].Name != "web-1" || servers[0].IPAddress != "139.59.201.10" {
		t.Errorf("servers[0] = %+v", servers[0])
	}
	if servers[0].PHPVersion != 8.3 || servers[0].MySQLVersion != 8.0 {
		t.Errorf("flex versions not parsed: %+v", servers[0])
	}
	if NormalizeStatus(servers[0].Status) != "active" {
		t.Errorf("status normalize = %q", NormalizeStatus(servers[0].Status))
	}
}

func TestServersPaginationFollowsLinksWithoutMeta(t *testing.T) {
	var pages []string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages = append(pages, r.URL.Query().Get("page"))
		switch len(pages) {
		case 1:
			writeResponse(t, w, `{"data":[{"id":1,"name":"web-1"}],"links":{"next":"/api/servers?page=2"}}`)
		case 2:
			writeResponse(t, w, `{"data":[{"id":2,"name":"web-2"}],"links":{"next":null}}`)
		default:
			t.Errorf("unexpected request %d", len(pages))
		}
	}))

	servers, err := c.Servers(context.Background())
	if err != nil {
		t.Fatalf("Servers: %v", err)
	}
	if len(servers) != 2 || len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Errorf("servers = %+v, pages = %v", servers, pages)
	}
}

func TestServerSingle(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers/7" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeResponse(t, w, `{"data":{"id":7,"status":"active","name":"web-1","monitoring":true,"php_version":8.3}}`)
	}))
	srv, err := c.Server(context.Background(), 7)
	if err != nil {
		t.Fatalf("Server: %v", err)
	}
	if !srv.Monitoring || srv.ID != 7 {
		t.Errorf("server = %+v", srv)
	}
}

func TestSitesList(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers/3/sites" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeResponse(t, w, `{"data":[{"id":1,"domain":"app.example.io","project_type":"laravel","deploy_script":"cd /home/ploi/app.example.io && php artisan deploy","php_version":"8.3","system_user":"ploi","disk_usage":{"bytes":123456789,"human":"117.74 MB"},"last_deploy_at":null}],"links":{},"meta":{"current_page":1,"last_page":1}}`)
	}))
	sites, err := c.Sites(context.Background(), 3)
	if err != nil {
		t.Fatalf("Sites: %v", err)
	}
	if len(sites) != 1 {
		t.Fatalf("got %d sites", len(sites))
	}
	s := sites[0]
	if s.Domain != "app.example.io" || s.ProjectType != "laravel" || s.PHPVersion != 8.3 {
		t.Errorf("site = %+v", s)
	}
	if s.DiskUsage == nil || s.DiskUsage.Human != "117.74 MB" {
		t.Errorf("disk usage = %+v", s.DiskUsage)
	}
	if s.LastDeployAt != nil {
		t.Errorf("last_deploy_at should be nil")
	}
	if s.DeployScript != "cd /home/ploi/app.example.io && php artisan deploy" {
		t.Errorf("deploy_script = %q", s.DeployScript)
	}
}

func TestSiteSingle(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers/3/sites/9" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeResponse(t, w, `{"data":{"id":9,"domain":"api.example.io","status":"active","web_directory":"/public","has_repository":true}}`)
	}))
	site, err := c.Site(context.Background(), 3, 9)
	if err != nil {
		t.Fatalf("Site: %v", err)
	}
	if site.WebDirectory != "/public" || !site.HasRepository {
		t.Errorf("site = %+v", site)
	}
}

func TestCertificates(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers/3/sites/9/certificates" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeResponse(t, w, `{"data":[{"id":1,"type":"letsencrypt","domain":"*.app.example.io","status":"active","active":true,"expires_at":"2026-10-06T03:57:07.000000Z"}],"links":{},"meta":{"current_page":1,"last_page":1}}`)
	}))
	certs, err := c.Certificates(context.Background(), 3, 9)
	if err != nil {
		t.Fatalf("Certificates: %v", err)
	}
	if len(certs) != 1 || certs[0].Domain != "*.app.example.io" || !certs[0].Active {
		t.Fatalf("certs = %+v", certs)
	}
	if certs[0].ExpiresAt == nil || certs[0].ExpiresAt.Month() != 10 || certs[0].ExpiresAt.Second() != 7 {
		t.Errorf("expires_at = %+v", certs[0].ExpiresAt)
	}
}

func TestMonitoring(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers/3/monitor" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeResponse(t, w, `{"data":[{"cpu":"5.2","ram":"21.81","disk":"40","load_average":"0.19","date":"2021-05-18 05:50:14"},{"cpu":6,"ram":22.09,"disk":40,"load_average":0.26,"date":"2021-05-18 05:40:06"}]}`)
	}))
	samples, err := c.Monitoring(context.Background(), 3)
	if err != nil {
		t.Fatalf("Monitoring: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("got %d samples", len(samples))
	}
	if samples[0].CPU != 5.2 || samples[0].RAM != 21.81 || samples[0].LoadAverage != 0.19 {
		t.Errorf("sample0 = %+v", samples[0])
	}
	if samples[1].CPU != 6 {
		t.Errorf("numeric sample not parsed: %+v", samples[1])
	}
}

func TestDatabases(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers/3/databases" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeResponse(t, w, `{"data":[{"id":1,"type":"postgresql","name":"application","server_id":3,"status":"active"},{"id":2,"type":"mysql","name":"legacy","server_id":3,"status":"active"}],"links":{},"meta":{"current_page":1,"last_page":1}}`)
	}))
	databases, err := c.Databases(context.Background(), 3)
	if err != nil {
		t.Fatalf("Databases: %v", err)
	}
	if len(databases) != 2 || databases[0].Type != "postgresql" || databases[1].Name != "legacy" {
		t.Fatalf("databases = %+v", databases)
	}
}

func TestSystemUsers(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/servers/3/system-users" {
			t.Errorf("path = %q", r.URL.Path)
		}
		writeResponse(t, w, `{"data":[{"id":1,"name":"customer","root":"/home/customer","created_at":"2020-01-01 12:00:00"}],"links":{},"meta":{"current_page":1,"last_page":1}}`)
	}))
	users, err := c.SystemUsers(context.Background(), 3)
	if err != nil {
		t.Fatalf("SystemUsers: %v", err)
	}
	if len(users) != 1 || users[0].Name != "customer" || users[0].Root != "/home/customer" {
		t.Fatalf("system users = %+v", users)
	}
}

func TestRestartServer(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/servers/5/restart" {
			t.Errorf("method/path = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
			t.Errorf("body decode: %v %v", body, err)
		}
		writeResponse(t, w, `{"message":"Server is now rebooting"}`)
	}))
	if err := c.RestartServer(context.Background(), 5); err != nil {
		t.Fatalf("RestartServer: %v", err)
	}
}

func TestUnauthorized(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		writeResponse(t, w, `{"message":"Unauthenticated."}`)
	}))
	if _, err := c.User(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestValidationErrorMapping(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeResponse(t, w, `{"message":"The given data was invalid.","errors":["You do not have monitoring installed on this server."],"links":[]}`)
	}))
	_, err := c.Monitoring(context.Background(), 3)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if !apiErr.MonitoringUnavailable() {
		t.Errorf("MonitoringUnavailable() = false for %v", apiErr)
	}
	if !strings.Contains(apiErr.Error(), "monitoring installed") {
		t.Errorf("error text = %q", apiErr.Error())
	}
}

func TestSubscriptionErrorMapping(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeResponse(t, w, `{"error":"Your subscription is not sufficient enough to allow monitoring.","links":["http://ploi.io/pricing"]}`)
	}))
	_, err := c.Monitoring(context.Background(), 3)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
	if !strings.Contains(apiErr.Message, "subscription is not sufficient") || len(apiErr.Links) != 1 {
		t.Errorf("apiErr = %+v", apiErr)
	}
	if !apiErr.MonitoringUnavailable() {
		t.Errorf("expected monitoring-unavailable detection")
	}
}

func TestRateLimitRetryAfter(t *testing.T) {
	var calls int
	var slept []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "54")
			w.WriteHeader(http.StatusTooManyRequests)
			writeResponse(t, w, `{"message":"Too many requests"}`)
			return
		}
		writeResponse(t, w, `{"data":{"email":"ok@example.com"}}`)
	}))
	t.Cleanup(srv.Close)

	c := New("t",
		WithBaseURL(srv.URL),
		WithSleeper(func(_ context.Context, d time.Duration) error {
			slept = append(slept, d)
			return nil
		}),
	)

	user, err := c.User(context.Background())
	if err != nil {
		t.Fatalf("User after retry: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
	if len(slept) != 1 || slept[0] < 53*time.Second {
		t.Errorf("sleeps = %v, want one ~54s wait honoring Retry-After", slept)
	}
	if user.Email != "ok@example.com" {
		t.Errorf("user = %+v", user)
	}
}

func TestRateLimitExhausted(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		writeResponse(t, w, `{"message":"Too many requests"}`)
	}))
	_, err := c.User(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("want APIError, got %v", err)
	}
	if !apiErr.RateLimited {
		t.Errorf("RateLimited flag not set")
	}
	if apiErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
}

func TestServerErrorRetried(t *testing.T) {
	var calls int
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeResponse(t, w, `{"data":{"email":"fine@example.com"}}`)
	}))
	user, err := c.User(context.Background())
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
	if user.Email != "fine@example.com" {
		t.Errorf("user = %+v", user)
	}
}

func TestContextCancelledDuringCooldown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := New("t", WithBaseURL("http://127.0.0.1:1"))
	c.Rate.mu.Lock()
	c.Rate.cooldownUntil = time.Now().Add(time.Hour)
	c.Rate.mu.Unlock()

	err := ctx.Err()
	if _, got := c.User(ctx); !errors.Is(got, err) {
		t.Fatalf("want context error, got %v", got)
	}
}

func TestFlexNumAndTimeEdgeCases(t *testing.T) {
	var v struct {
		A FlexNum `json:"a"`
		B FlexNum `json:"b"`
		C FlexNum `json:"c"`
		T Time    `json:"t"`
		N Time    `json:"n"`
		P *Time   `json:"p"`
		S Server  `json:"srv"`
		U User    `json:"usr"`
	}
	data := `{"a":"12.5","b":3,"c":"none","t":"2024-02-29 23:59:59","n":null}`
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.A != 12.5 || v.B != 3 || v.C != 0 {
		t.Errorf("flex numbers: %+v", v)
	}
	if v.T.Month() != time.February || v.T.Day() != 29 || v.T.Second() != 59 {
		t.Errorf("time = %v", v.T.Time)
	}
	if !v.N.IsZero() {
		t.Errorf("null time should be zero, got %v", v.N.Time)
	}
	if v.P != nil {
		t.Errorf("pointer to null should stay nil")
	}
	out, err := json.Marshal(Time{})
	if err != nil || string(out) != "null" {
		t.Errorf("marshal zero time = %s, %v", out, err)
	}
	out2, err := json.Marshal(User{CreatedAt: Time{Time: time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)}})
	if err != nil || !strings.Contains(string(out2), `"2025-01-02 03:04:05"`) {
		t.Errorf("marshal time round-trip = %s, %v", out2, err)
	}
}
