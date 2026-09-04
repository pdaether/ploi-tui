package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const ploiTimeLayout = "2006-01-02 15:04:05"

type Time struct{ time.Time }

func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(strings.Trim(string(b), `"`))
	if s == "" || s == "null" {
		t.Time = time.Time{}
		return nil
	}
	for _, layout := range []string{ploiTimeLayout, time.RFC3339Nano, time.RFC3339} {
		var parsed time.Time
		var err error
		if layout == ploiTimeLayout {
			parsed, err = time.ParseInLocation(layout, s, time.UTC)
		} else {
			parsed, err = time.Parse(layout, s)
		}
		if err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("parse time %q: expected %s or RFC3339", s, ploiTimeLayout)
}

func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte(`null`), nil
	}
	return json.Marshal(t.Format(ploiTimeLayout))
}

type FlexNum float64

func (f *FlexNum) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	if len(s) > 1 && s[0] == '"' {
		unquoted, err := strconv.Unquote(s)
		if err != nil {
			return err
		}
		s = strings.TrimSpace(unquoted)
		if s == "" || strings.EqualFold(s, "none") {
			*f = 0
			return nil
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("parse number %q: %w", s, err)
	}
	*f = FlexNum(v)
	return nil
}

type User struct {
	Avatar        string `json:"avatar"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Country       string `json:"country"`
	Timezone      string `json:"timezone"`
	CreatedAt     Time   `json:"created_at"`
	Plan          string `json:"plan"`
	PlanExpiresAt *Time  `json:"plan_expires_at"`
	BillingDetail any    `json:"billing_details"`
}

type DiskUsage struct {
	Bytes int64  `json:"bytes"`
	Human string `json:"human"`
}

type Server struct {
	ID           int64   `json:"id"`
	Type         string  `json:"type"`
	Name         string  `json:"name"`
	IPAddress    string  `json:"ip_address"`
	PHPVersion   FlexNum `json:"php_version"`
	MySQLVersion FlexNum `json:"mysql_version"`
	SitesCount   int     `json:"sites_count"`
	Status       string  `json:"status"`
	StatusID     int     `json:"status_id"`
	Monitoring   bool    `json:"monitoring"`
	CreatedAt    Time    `json:"created_at"`
}

type Database struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	ServerID  int64  `json:"server_id"`
	Status    string `json:"status"`
	CreatedAt Time   `json:"created_at"`
}

type SystemUser struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Root      string `json:"root"`
	CreatedAt Time   `json:"created_at"`
}

type Site struct {
	ID            int64      `json:"id"`
	Status        string     `json:"status"`
	ServerID      int64      `json:"server_id"`
	Domain        string     `json:"domain"`
	DeployScript  string     `json:"deploy_script"`
	WebDirectory  string     `json:"web_directory"`
	ProjectType   string     `json:"project_type"`
	ProjectRoot   string     `json:"project_root"`
	LastDeployAt  *Time      `json:"last_deploy_at"`
	SystemUser    string     `json:"system_user"`
	PHPVersion    FlexNum    `json:"php_version"`
	HealthURL     *string    `json:"health_url"`
	HasRepository bool       `json:"has_repository"`
	QuickDeploy   bool       `json:"quick_deploy"`
	DiskUsage     *DiskUsage `json:"disk_usage"`
	CreatedAt     Time       `json:"created_at"`
}

type Certificate struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	Domain    string `json:"domain"`
	Type      string `json:"type"`
	Active    bool   `json:"active"`
	SiteID    int64  `json:"site_id"`
	ServerID  int64  `json:"server_id"`
	ExpiresAt *Time  `json:"expires_at"`
	CreatedAt Time   `json:"created_at"`
}

type MonitoringSample struct {
	CPU         FlexNum `json:"cpu"`
	RAM         FlexNum `json:"ram"`
	Disk        FlexNum `json:"disk"`
	LoadAverage FlexNum `json:"load_average"`
	Date        Time    `json:"date"`
}
