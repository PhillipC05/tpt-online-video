package handlers

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

const (
	maxTitleLen       = 200
	maxDescriptionLen = 5000
	maxBioLen         = 1000
	maxDisplayNameLen = 100
)

// sanitizeText strips null bytes and other non-printable control characters
// (excluding common whitespace: space, tab, newline, carriage return) from s,
// trims surrounding whitespace, and enforces a maximum rune length.
//
// It does NOT HTML-escape: the API returns JSON and React escapes on render.
// Stripping control chars defends against terminal injection and log poisoning.
func sanitizeText(s string, maxLen int) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\x00' {
			continue // drop null bytes
		}
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			continue // drop other control chars
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if maxLen > 0 {
		runes := []rune(out)
		if len(runes) > maxLen {
			return "", fmt.Errorf("must be %d characters or fewer", maxLen)
		}
	}
	return out, nil
}

// validateTitle sanitizes and enforces non-empty + max-length for title fields.
func validateTitle(s string) (string, error) {
	clean, err := sanitizeText(s, maxTitleLen)
	if err != nil {
		return "", err
	}
	if clean == "" {
		return "", fmt.Errorf("title cannot be empty")
	}
	return clean, nil
}

// validateDescription sanitizes and enforces max-length for description fields.
func validateDescription(s string) (string, error) {
	return sanitizeText(s, maxDescriptionLen)
}

// validateDisplayName sanitizes and enforces non-empty + max-length.
func validateDisplayName(s string) (string, error) {
	clean, err := sanitizeText(s, maxDisplayNameLen)
	if err != nil {
		return "", err
	}
	if clean == "" {
		return "", fmt.Errorf("display_name cannot be empty")
	}
	return clean, nil
}

// validateBio sanitizes and enforces max-length for bio fields.
func validateBio(s string) (string, error) {
	return sanitizeText(s, maxBioLen)
}

// isAllowedRedirectURL returns true if target is an absolute URL whose origin
// matches one of the allowed origins. Used to prevent open-redirect attacks.
func isAllowedRedirectURL(target string, allowedOrigins []string) bool {
	u, err := url.ParseRequestURI(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	targetOrigin := u.Scheme + "://" + u.Host
	for _, origin := range allowedOrigins {
		if strings.EqualFold(strings.TrimRight(targetOrigin, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}
