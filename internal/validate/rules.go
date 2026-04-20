// Package validate infers validation rules from key name suffixes and runs
// them against parsed values. The suffix table is also the source of truth
// for masking, so that `show`, `diff --values`, and `validate` all agree on
// which keys are sensitive.
package validate

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
)

// Rule classifies a key by one or more case-insensitive suffixes, provides a
// validation function, and records whether values for matched keys should be
// masked when displayed.
type Rule struct {
	Name     string             // short identifier, e.g. "url"
	Suffixes []string           // uppercase suffixes, e.g. ["_URL", "_URI"]
	Validate func(string) error // required; run against each matched value
	Mask     bool               // true when the value is sensitive
}

// Rules is the ordered list of known validation rules. The order determines
// matching priority when a key could in principle match multiple rules; more
// specific suffixes should appear earlier.
var Rules = []*Rule{
	{
		Name:     "url",
		Suffixes: []string{"_URL", "_URI"},
		Validate: validateURL,
	},
	{
		Name:     "port",
		Suffixes: []string{"_PORT"},
		Validate: validatePort,
	},
	{
		Name:     "secret",
		Suffixes: []string{"_KEY", "_SECRET", "_TOKEN", "_PASSWORD"},
		Validate: validateNonEmpty,
		Mask:     true,
	},
	{
		Name:     "email",
		Suffixes: []string{"_EMAIL", "_ADDRESS"},
		Validate: validateEmail,
	},
	{
		Name:     "bool",
		Suffixes: []string{"_ENABLED", "_DEBUG", "_FLAG"},
		Validate: validateBool,
	},
}

// MatchRule returns the first rule whose suffix matches key (case-insensitive),
// or nil when no rule applies.
func MatchRule(key string) *Rule {
	upper := strings.ToUpper(key)
	for _, r := range Rules {
		for _, s := range r.Suffixes {
			if strings.HasSuffix(upper, s) {
				return r
			}
		}
	}
	return nil
}

// ShouldMask reports whether values for the given key should be masked when
// displayed. It is safe to call with any key; unmatched keys return false.
func ShouldMask(key string) bool {
	r := MatchRule(key)
	return r != nil && r.Mask
}

// Masked is the canonical masked representation used across all commands.
const Masked = "****"

// validateURL accepts a scheme-qualified URL ("scheme://host[/path]...") or a
// bare host (optionally with ":port"). Scheme-qualified values must have a
// scheme and, except for "file" URIs, a host. The host portion of a bare
// value must either be "localhost" (case-insensitive) or contain a "."
// (matching FQDNs like example.com and IPv4 literals like 127.0.0.1).
// Whitespace is never allowed. This keeps the validator usefully strict
// without rejecting common bare-host cases like "localhost:3000".
func validateURL(v string) error {
	invalid := func() error { return fmt.Errorf("not a valid URL (got %q)", v) }
	if v == "" || strings.ContainsAny(v, " \t\r\n") {
		return invalid()
	}
	if strings.Contains(v, "://") {
		u, err := url.Parse(v)
		if err != nil || u.Scheme == "" {
			return invalid()
		}
		// file:// URIs legitimately have an empty host ("file:///path").
		if u.Host == "" && !strings.EqualFold(u.Scheme, "file") {
			return invalid()
		}
		return nil
	}
	// Bare value: split off an optional ":port" suffix so the host portion
	// can be checked against the allow-list.
	host := v
	if i := strings.IndexByte(v, ':'); i >= 0 {
		host = v[:i]
	}
	if strings.EqualFold(host, "localhost") || strings.Contains(host, ".") {
		return nil
	}
	return invalid()
}

func validatePort(v string) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("not a valid port number (got %q)", v)
	}
	return nil
}

func validateNonEmpty(v string) error {
	if v == "" {
		return errors.New("must not be empty")
	}
	return nil
}

func validateEmail(v string) error {
	addr, err := mail.ParseAddress(v)
	if err != nil || addr.Address != v || addr.Name != "" {
		return fmt.Errorf("not a valid email address (got %q)", v)
	}
	return nil
}

func validateBool(v string) error {
	switch strings.ToLower(v) {
	case "true", "false", "1", "0":
		return nil
	default:
		return fmt.Errorf("must be one of true/false/1/0 (got %q)", v)
	}
}
