// Package parser implements a small, stdlib-only parser for .env files.
//
// The supported syntax is intentionally a subset of the various dotenv dialects:
// comments (#), simple KEY=value pairs, single/double quoted values, empty
// values, inline comments (when preceded by whitespace on unquoted values),
// and the optional `export` prefix. Unsupported constructs (variable
// interpolation, multi-line values, nested quotes) are not expanded; the
// parser either passes the raw text through or skips the line with a warning.
package parser

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

// Entry is a single KEY=VALUE pair parsed from an .env file.
type Entry struct {
	Key   string
	Value string
	Line  int // 1-indexed line number in the source
}

// Warning records a non-fatal issue encountered during parsing.
type Warning struct {
	Line    int // 1-indexed; 0 when not tied to a specific line
	Message string
}

// String returns the warning formatted as "line N: message".
func (w Warning) String() string {
	if w.Line <= 0 {
		return w.Message
	}
	return fmt.Sprintf("line %d: %s", w.Line, w.Message)
}

// Result is the outcome of parsing a source.
type Result struct {
	Entries  []Entry
	Warnings []Warning
}

// Map returns key -> value. When a key appears multiple times, the last
// occurrence wins.
func (r *Result) Map() map[string]string {
	m := make(map[string]string, len(r.Entries))
	for _, e := range r.Entries {
		m[e.Key] = e.Value
	}
	return m
}

// Keys returns the keys in the order they first appeared.
func (r *Result) Keys() []string {
	seen := make(map[string]struct{}, len(r.Entries))
	keys := make([]string, 0, len(r.Entries))
	for _, e := range r.Entries {
		if _, ok := seen[e.Key]; ok {
			continue
		}
		seen[e.Key] = struct{}{}
		keys = append(keys, e.Key)
	}
	return keys
}

// Ordered returns the keys in first-occurrence order together with the
// deduplicated key -> value map (last value wins). It is a single-pass
// combination of Keys and Map for callers that need both.
func (r *Result) Ordered() ([]string, map[string]string) {
	m := make(map[string]string, len(r.Entries))
	order := make([]string, 0, len(r.Entries))
	for _, e := range r.Entries {
		if _, ok := m[e.Key]; !ok {
			order = append(order, e.Key)
		}
		m[e.Key] = e.Value
	}
	return order, m
}

// Parse reads .env-formatted content from r.
func Parse(r io.Reader) (*Result, error) {
	res := &Result{}
	scanner := bufio.NewScanner(r)
	// Allow larger lines than the default 64KB.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	firstLine := true
	seen := make(map[string]int)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if firstLine {
			line = strings.TrimPrefix(line, "\ufeff")
			firstLine = false
		}
		// Handle Windows line endings.
		line = strings.TrimRight(line, "\r")

		entry, ok, warn := parseLine(line)
		if warn != "" {
			res.Warnings = append(res.Warnings, Warning{Line: lineNum, Message: warn})
		}
		if !ok {
			continue
		}
		entry.Line = lineNum
		if prev, dup := seen[entry.Key]; dup {
			res.Warnings = append(res.Warnings, Warning{
				Line:    lineNum,
				Message: fmt.Sprintf("duplicate key %q (first seen on line %d; last value wins)", entry.Key, prev),
			})
		}
		seen[entry.Key] = lineNum
		res.Entries = append(res.Entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return res, nil
}

// ParseFile opens path and parses it as an .env file.
func ParseFile(path string) (*Result, error) {
	f, err := os.Open(path) //nolint:gosec // envlens reads user-supplied .env paths by design.
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only file close failure is not actionable.
	return Parse(f)
}

// parseLine parses a single line. It returns ok=false for comments, blank
// lines, and lines that should be skipped with a warning. The caller is
// responsible for populating Entry.Line.
func parseLine(line string) (Entry, bool, string) {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return Entry{}, false, ""
	}

	// Strip optional `export ` prefix.
	if after, stripped := stripExport(trimmed); stripped {
		trimmed = after
	}

	eq := strings.IndexByte(trimmed, '=')
	if eq < 0 {
		return Entry{}, false, fmt.Sprintf("missing '=' in %q; skipping", line)
	}

	key := strings.TrimRightFunc(trimmed[:eq], unicode.IsSpace)
	if key == "" {
		return Entry{}, false, fmt.Sprintf("empty key in %q; skipping", line)
	}
	if !isValidKey(key) {
		return Entry{}, false, fmt.Sprintf("invalid key %q; skipping", key)
	}

	value, warn := parseValue(trimmed[eq+1:])
	if warn != "" {
		return Entry{}, false, warn
	}
	return Entry{Key: key, Value: value}, true, ""
}

func stripExport(s string) (string, bool) {
	const prefix = "export"
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	rest := s[len(prefix):]
	if rest == "" || !(rest[0] == ' ' || rest[0] == '\t') {
		return s, false
	}
	return strings.TrimLeftFunc(rest, unicode.IsSpace), true
}

// isValidKey enforces POSIX identifier rules: starts with an ASCII letter or
// underscore, subsequent characters may also be ASCII digits. Non-ASCII
// characters are rejected — POSIX shells (and therefore real-world env vars)
// don't support them, and allowing them here would accept keys that could
// never actually be exported.
func isValidKey(k string) bool {
	for i, r := range k {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// parseValue returns the value string and an optional warning. The input is
// everything after the first '=' character on a line.
func parseValue(raw string) (string, string) {
	// Strip optional leading whitespace between '=' and the value.
	s := strings.TrimLeft(raw, " \t")
	if s == "" {
		return "", ""
	}
	if s[0] == '"' || s[0] == '\'' {
		return parseQuotedValue(s)
	}
	return parseUnquotedValue(s), ""
}

func parseQuotedValue(s string) (string, string) {
	quote := s[0]
	end := strings.IndexByte(s[1:], quote)
	if end < 0 {
		return "", fmt.Sprintf("unmatched %c quote; skipping", quote)
	}
	return s[1 : 1+end], ""
}

// parseUnquotedValue treats the value as literal up to an inline comment
// (whitespace followed by '#') or end of line, then trims trailing whitespace.
func parseUnquotedValue(s string) string {
	end := len(s)
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			continue
		}
		j := i
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j < len(s) && s[j] == '#' {
			end = i
			break
		}
	}
	return strings.TrimRight(s[:end], " \t")
}
