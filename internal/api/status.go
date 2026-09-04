package api

import (
	"strings"
	"time"
)

type Severity int

const (
	SeverityNeutral Severity = iota
	SeverityOK
	SeverityBusy
	SeverityBad
)

func NormalizeStatus(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "server ")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

var serverSeverity = map[string]Severity{
	"active":          SeverityOK,
	"created":         SeverityBusy,
	"building":        SeverityBusy,
	"refreshing":      SeverityBusy,
	"rebooting":       SeverityBusy,
	"destroying":      SeverityBusy,
	"unreachable":     SeverityBad,
	"creating-failed": SeverityBad,
	"building-failed": SeverityBad,
}

var siteSeverity = map[string]Severity{
	"active":                     SeverityOK,
	"created":                    SeverityBusy,
	"building":                   SeverityBusy,
	"deploying":                  SeverityBusy,
	"suspended":                  SeverityNeutral,
	"repository-installing":      SeverityBusy,
	"repository-uninstalling":    SeverityBusy,
	"wordpress-installing":       SeverityBusy,
	"wordpress-uninstalling":     SeverityBusy,
	"nextcloud-installing":       SeverityBusy,
	"nextcloud-uninstalling":     SeverityBusy,
	"statamic-installing":        SeverityBusy,
	"statamic-uninstalling":      SeverityBusy,
	"octobercms-installing":      SeverityBusy,
	"octobercms-uninstalling":    SeverityBusy,
	"clone-in-progress":          SeverityBusy,
	"staging-production-syncing": SeverityBusy,
	"deleting":                   SeverityBusy,
	"deploy-failed":              SeverityBad,
}

var certificateSeverity = map[string]Severity{
	"active":   SeverityOK,
	"created":  SeverityBusy,
	"deleting": SeverityBusy,
}

func ServerStatusSeverity(raw string) Severity {
	if s, ok := serverSeverity[NormalizeStatus(raw)]; ok {
		return s
	}
	return SeverityBad
}

func SiteStatusSeverity(raw string) Severity {
	if s, ok := siteSeverity[NormalizeStatus(raw)]; ok {
		return s
	}
	return SeverityBad
}

func CertificateStatusSeverity(raw string) Severity {
	if s, ok := certificateSeverity[NormalizeStatus(raw)]; ok {
		return s
	}
	return SeverityBad
}

const certWarnDays = 30

const certCriticalDays = 7

func CertExpirySeverity(expiresAt *Time, nowUTC time.Time) Severity {
	if expiresAt == nil || expiresAt.IsZero() {
		return SeverityNeutral
	}
	days := expiresAt.Sub(nowUTC).Hours() / 24
	switch {
	case days <= 0:
		return SeverityBad
	case days <= certCriticalDays:
		return SeverityBad
	case days <= certWarnDays:
		return SeverityBusy
	default:
		return SeverityOK
	}
}
