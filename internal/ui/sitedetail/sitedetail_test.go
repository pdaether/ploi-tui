package sitedetail

import (
	"strings"
	"testing"
	"time"

	"github.com/pdaether/ploi-tui/internal/api"
)

func TestViewShowsSiteAndCertificateDetails(t *testing.T) {
	now := time.Now().UTC()
	m := Model{
		Server: &api.Server{ID: 7, Name: "web-1"},
		Site: &api.Site{
			ID:            9,
			Domain:        "app.example.io",
			Status:        "active",
			ProjectType:   "laravel",
			SystemUser:    "ploi",
			PHPVersion:    8.3,
			WebDirectory:  "/public",
			ProjectRoot:   "/home/ploi/app.example.io",
			HasRepository: true,
			DiskUsage:     &api.DiskUsage{Human: "1.2 GB"},
		},
		CertificatesLoaded: true,
		Certificates: []api.Certificate{
			{Type: "letsencrypt", Domain: "*.app.example.io", Status: "active", ExpiresAt: &api.Time{Time: now.Add(62 * 24 * time.Hour)}},
			{Type: "letsencrypt", Domain: "app.example.io", Status: "active", ExpiresAt: &api.Time{Time: now.Add(6 * 24 * time.Hour)}},
		},
	}

	view := m.View(100)
	for _, want := range []string{"Site ID", "9", "Server ID", "7", "Domain", "app.example.io", "laravel", "ploi", "/public", "1.2 GB", "CERTIFICATES", "Let's Encrypt", "62d", "6d", "ACTIONS", "SSH into site", "Open in Ploi", "Open domain"} {
		if !strings.Contains(view, want) {
			t.Errorf("site detail missing %q:\n%s", want, view)
		}
	}
}

func TestCertificateExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if got := certificateExpiry(nil, now); !strings.Contains(got, "—") {
		t.Errorf("nil expiry = %q", got)
	}
	if got := certificateExpiry(&api.Time{Time: now.Add(-time.Hour)}, now); !strings.Contains(got, "expired") {
		t.Errorf("expired certificate = %q", got)
	}
	if got := certificateExpiry(&api.Time{Time: now.Add(8 * 24 * time.Hour)}, now); !strings.Contains(got, "8d") {
		t.Errorf("warning certificate = %q", got)
	}
}
