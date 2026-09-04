package api

import (
	"testing"
	"time"
)

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"Server active": "active",
		"Active":        "active",
		" active ":      "active",
		"deploy-failed": "deploy-failed",
		"DEPLOY_FAILED": "deploy-failed",
		"Rebooting":     "rebooting",
	}
	for in, want := range cases {
		if got := NormalizeStatus(in); got != want {
			t.Errorf("NormalizeStatus(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeverities(t *testing.T) {
	serverCases := map[string]Severity{
		"active":          SeverityOK,
		"Server active":   SeverityOK,
		"building":        SeverityBusy,
		"rebooting":       SeverityBusy,
		"refreshing":      SeverityBusy,
		"unreachable":     SeverityBad,
		"building-failed": SeverityBad,
		"":                SeverityBad,
		"mystery":         SeverityBad,
	}
	for raw, want := range serverCases {
		if got := ServerStatusSeverity(raw); got != want {
			t.Errorf("ServerStatusSeverity(%q) = %v, want %v", raw, got, want)
		}
	}

	siteCases := map[string]Severity{
		"active":        SeverityOK,
		"deploying":     SeverityBusy,
		"suspended":     SeverityNeutral,
		"deploy-failed": SeverityBad,
		"deleting":      SeverityBusy,
	}
	for raw, want := range siteCases {
		if got := SiteStatusSeverity(raw); got != want {
			t.Errorf("SiteStatusSeverity(%q) = %v, want %v", raw, got, want)
		}
	}

	certCases := map[string]Severity{
		"active":   SeverityOK,
		"created":  SeverityBusy,
		"deleting": SeverityBusy,
	}
	for raw, want := range certCases {
		if got := CertificateStatusSeverity(raw); got != want {
			t.Errorf("CertificateStatusSeverity(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestCertExpirySeverity(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	mk := func(days int) *Time {
		return &Time{Time: now.Add(time.Duration(days) * 24 * time.Hour)}
	}

	cases := []struct {
		days *Time
		want Severity
	}{
		{mk(62), SeverityOK},
		{mk(31), SeverityOK},
		{mk(30), SeverityBusy},
		{mk(8), SeverityBusy},
		{mk(7), SeverityBad},
		{mk(-1), SeverityBad},
	}
	for _, c := range cases {
		if got := CertExpirySeverity(c.days, now); got != c.want {
			t.Errorf("CertExpirySeverity(+%dd) = %v, want %v", int(c.days.Sub(now).Hours()/24), got, c.want)
		}
	}
	if got := CertExpirySeverity(nil, now); got != SeverityNeutral {
		t.Errorf("CertExpirySeverity(nil) = %v, want neutral", got)
	}
	if got := CertExpirySeverity(&Time{}, now); got != SeverityNeutral {
		t.Errorf("CertExpirySeverity(zero) = %v, want neutral", got)
	}
}
