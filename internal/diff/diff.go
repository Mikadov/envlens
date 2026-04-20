// Package diff computes key-level (and optionally value-level) differences
// between two parsed .env files and renders them to an io.Writer.
package diff

import (
	"fmt"
	"io"
	"sort"

	"github.com/Mikadov/envlens/internal/validate"
)

// File represents one side of a comparison.
type File struct {
	// Name is the display label (typically the file path).
	Name string
	// Map is the key -> value data from this file.
	Map map[string]string
}

// Change records a key present in both files with differing values.
type Change struct {
	Key    string
	ValueA string
	ValueB string
}

// Result is the structured outcome of a comparison.
type Result struct {
	A       File
	B       File
	Missing []string // keys in B not in A
	Extra   []string // keys in A not in B
	Changed []Change // populated only when CompareOptions.WithValues is true
}

// Empty reports whether the two files match.
func (r *Result) Empty() bool {
	return len(r.Missing) == 0 && len(r.Extra) == 0 && len(r.Changed) == 0
}

// CompareOptions controls Compare's behaviour.
type CompareOptions struct {
	// WithValues, when true, populates Result.Changed by comparing values of
	// keys present in both files.
	WithValues bool
}

// Compare computes the structured diff between a and b.
func Compare(a, b File, opts CompareOptions) *Result {
	res := &Result{A: a, B: b}
	for k := range b.Map {
		if _, ok := a.Map[k]; !ok {
			res.Missing = append(res.Missing, k)
		}
	}
	for k := range a.Map {
		if _, ok := b.Map[k]; !ok {
			res.Extra = append(res.Extra, k)
		}
	}
	sort.Strings(res.Missing)
	sort.Strings(res.Extra)

	if opts.WithValues {
		for k, va := range a.Map {
			vb, ok := b.Map[k]
			if !ok || va == vb {
				continue
			}
			res.Changed = append(res.Changed, Change{Key: k, ValueA: va, ValueB: vb})
		}
		sort.Slice(res.Changed, func(i, j int) bool {
			return res.Changed[i].Key < res.Changed[j].Key
		})
	}
	return res
}

// PrintOptions controls rendered output.
type PrintOptions struct {
	Color      bool
	WithValues bool
	Quiet      bool
}

const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
	ansiGreen  = "\x1b[32m"
)

// Print writes a human-readable diff to w. When r is empty, it prints a
// "No differences found." confirmation. Quiet mode writes nothing.
func Print(w io.Writer, r *Result, opts PrintOptions) error {
	if opts.Quiet {
		return nil
	}
	if r.Empty() {
		return writeLn(w, colored(opts.Color, ansiGreen, "No differences found."))
	}

	if len(r.Missing) > 0 {
		header := fmt.Sprintf("Missing in %s (defined in %s):", r.A.Name, r.B.Name)
		if err := writeLn(w, colored(opts.Color, ansiRed, header)); err != nil {
			return err
		}
		for _, k := range r.Missing {
			line := fmt.Sprintf("  - %s", formatKeyValue(k, r.B.Map[k], opts.WithValues))
			if err := writeLn(w, colored(opts.Color, ansiRed, line)); err != nil {
				return err
			}
		}
		if err := writeLn(w, ""); err != nil {
			return err
		}
	}

	if len(r.Extra) > 0 {
		header := fmt.Sprintf("Extra in %s (not in %s):", r.A.Name, r.B.Name)
		if err := writeLn(w, colored(opts.Color, ansiYellow, header)); err != nil {
			return err
		}
		for _, k := range r.Extra {
			line := fmt.Sprintf("  - %s", formatKeyValue(k, r.A.Map[k], opts.WithValues))
			if err := writeLn(w, colored(opts.Color, ansiYellow, line)); err != nil {
				return err
			}
		}
		if err := writeLn(w, ""); err != nil {
			return err
		}
	}

	if opts.WithValues && len(r.Changed) > 0 {
		if err := writeLn(w, colored(opts.Color, ansiCyan, "Different values:")); err != nil {
			return err
		}
		for _, c := range r.Changed {
			if err := writeLn(w, colored(opts.Color, ansiCyan, "  ~ "+c.Key)); err != nil {
				return err
			}
			if err := writeLn(w, fmt.Sprintf("      %s: %s", r.A.Name, maskedValue(c.Key, c.ValueA))); err != nil {
				return err
			}
			if err := writeLn(w, fmt.Sprintf("      %s: %s", r.B.Name, maskedValue(c.Key, c.ValueB))); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatKeyValue(key, value string, withValues bool) string {
	if !withValues {
		return key
	}
	return fmt.Sprintf("%s = %s", key, maskedValue(key, value))
}

func maskedValue(key, value string) string {
	if validate.ShouldMask(key) {
		return validate.Masked
	}
	return value
}

func colored(enabled bool, color, s string) string {
	if !enabled {
		return s
	}
	return color + s + ansiReset
}

func writeLn(w io.Writer, s string) error {
	_, err := fmt.Fprintln(w, s)
	return err
}
