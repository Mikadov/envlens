// Package display renders parsed .env entries either as a Unicode box-drawing
// table (default) or as JSON. Values for sensitive keys are masked unless the
// caller opts out with Options.NoMask.
package display

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Mikadov/envlens/internal/validate"
)

// Entry is the minimal key/value pair display understands. Callers convert
// from parser.Entry by copying Key and Value.
type Entry struct {
	Key   string
	Value string
}

// Options controls how entries are rendered.
type Options struct {
	// NoMask disables automatic masking of sensitive keys.
	NoMask bool
	// JSON renders the entries as a pretty-printed JSON object instead of a
	// table. Masking still applies unless NoMask is set.
	JSON bool
}

// Print writes entries to w.
func Print(w io.Writer, entries []Entry, opts Options) error {
	masked := applyMasking(entries, opts.NoMask)
	if opts.JSON {
		return printJSON(w, masked)
	}
	return printTable(w, masked)
}

func applyMasking(entries []Entry, noMask bool) []Entry {
	out := make([]Entry, len(entries))
	for i, e := range entries {
		out[i] = e
		if !noMask && validate.ShouldMask(e.Key) {
			out[i].Value = validate.Masked
		}
	}
	return out
}

func printJSON(w io.Writer, entries []Entry) error {
	// Preserve key order via an ordered structure; json.Marshal on maps sorts
	// by key, which we don't want here — hand-build an ordered document and
	// write it directly to w.
	m := make(map[string]string, len(entries))
	order := make([]string, 0, len(entries))
	for _, e := range entries {
		if _, ok := m[e.Key]; !ok {
			order = append(order, e.Key)
		}
		m[e.Key] = e.Value
	}
	if _, err := io.WriteString(w, "{\n"); err != nil {
		return err
	}
	for i, k := range order {
		key, err := json.Marshal(k)
		if err != nil {
			return fmt.Errorf("marshal key %q: %w", k, err)
		}
		val, err := json.Marshal(m[k])
		if err != nil {
			return fmt.Errorf("marshal value for %q: %w", k, err)
		}
		comma := ","
		if i == len(order)-1 {
			comma = ""
		}
		if _, err := fmt.Fprintf(w, "  %s: %s%s\n", key, val, comma); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "}\n")
	return err
}

const (
	tlCorner = "┌"
	trCorner = "┐"
	blCorner = "└"
	brCorner = "┘"
	tJoin    = "┬"
	bJoin    = "┴"
	lJoin    = "├"
	rJoin    = "┤"
	cross    = "┼"
	hLine    = "─"
	vLine    = "│"
)

func printTable(w io.Writer, entries []Entry) error {
	const keyHeader, valueHeader = "KEY", "VALUE"
	keyWidth := stringWidth(keyHeader)
	valWidth := stringWidth(valueHeader)
	for _, e := range entries {
		if wk := stringWidth(e.Key); wk > keyWidth {
			keyWidth = wk
		}
		if wv := stringWidth(e.Value); wv > valWidth {
			valWidth = wv
		}
	}

	writeLine := func(left, mid, right string) error {
		_, err := fmt.Fprintf(w, "%s%s%s%s%s\n",
			left,
			strings.Repeat(hLine, keyWidth+2),
			mid,
			strings.Repeat(hLine, valWidth+2),
			right,
		)
		return err
	}
	writeRow := func(k, v string) error {
		_, err := fmt.Fprintf(w, "%s %s%s %s %s%s %s\n",
			vLine, k, strings.Repeat(" ", keyWidth-stringWidth(k)),
			vLine, v, strings.Repeat(" ", valWidth-stringWidth(v)),
			vLine,
		)
		return err
	}

	if err := writeLine(tlCorner, tJoin, trCorner); err != nil {
		return err
	}
	if err := writeRow(keyHeader, valueHeader); err != nil {
		return err
	}
	if err := writeLine(lJoin, cross, rJoin); err != nil {
		return err
	}
	for _, e := range entries {
		if err := writeRow(e.Key, e.Value); err != nil {
			return err
		}
	}
	if err := writeLine(blCorner, bJoin, brCorner); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nTotal: %d keys\n", len(entries)); err != nil {
		return err
	}
	return nil
}

// stringWidth returns an approximate terminal display width (in columns) for
// s. It treats ASCII as 1 column and most CJK / emoji ranges as 2 columns.
func stringWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

// runeWidth approximates the display width of a single rune. It is not a full
// UAX #11 implementation but covers the common cases (ASCII, CJK, emoji) well
// enough for .env files without pulling in any external dependency.
func runeWidth(r rune) int {
	switch {
	case r < 0x20, r == 0x7f:
		return 0
	case r < 0x80:
		return 1
	case r >= 0x1100 && r <= 0x115F,
		r >= 0x2E80 && r <= 0x303E,
		r >= 0x3041 && r <= 0x33FF,
		r >= 0x3400 && r <= 0x4DBF,
		r >= 0x4E00 && r <= 0x9FFF,
		r >= 0xA000 && r <= 0xA4CF,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE4F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x1F300 && r <= 0x1F64F,
		r >= 0x1F680 && r <= 0x1F9FF,
		r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}
