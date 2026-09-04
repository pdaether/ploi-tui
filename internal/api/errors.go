package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized: token missing, invalid or revoked")

type APIError struct {
	StatusCode  int
	Message     string
	Errors      []string
	Links       []string
	RateLimited bool
	RetryAfter  time.Duration
}

func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("ploi API error (%d): %s", e.StatusCode, strings.Join(e.Errors, "; "))
	}
	if e.Message == "" {
		return fmt.Sprintf("ploi API error: unexpected status %d", e.StatusCode)
	}
	return fmt.Sprintf("ploi API error (%d): %s", e.StatusCode, e.Message)
}

func (e *APIError) MonitoringUnavailable() bool {
	if e.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	msgs := append([]string{e.Message}, e.Errors...)
	for _, msg := range msgs {
		m := strings.ToLower(msg)
		if strings.Contains(m, "monitoring") || strings.Contains(m, "subscription") {
			return true
		}
	}
	return false
}

func decodeAPIError(status int, body []byte) *APIError {
	var parsed struct {
		Message string   `json:"message"`
		Error   string   `json:"error"`
		Errors  []string `json:"errors"`
		Links   []string `json:"links"`
	}
	err := strictUnmarshal(body, &parsed)

	apiErr := &APIError{StatusCode: status}
	if err == nil {
		switch {
		case parsed.Message != "":
			apiErr.Message = parsed.Message
		case parsed.Error != "":
			apiErr.Message = parsed.Error
		default:
			apiErr.Message = strings.TrimSpace(string(body))
		}
		apiErr.Errors = parsed.Errors
		apiErr.Links = parsed.Links
	} else {
		apiErr.Message = strings.TrimSpace(string(body))
	}
	if apiErr.Message == "" && len(apiErr.Errors) == 0 {
		apiErr.Message = http.StatusText(status)
	}
	return apiErr
}
