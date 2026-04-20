package display_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Mikadov/envlens/internal/display"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrint_TableBasic(t *testing.T) {
	t.Parallel()
	entries := []display.Entry{
		{Key: "APP_NAME", Value: "MyApp"},
		{Key: "APP_KEY", Value: "ultra-secret"},
		{Key: "DB_HOST", Value: "127.0.0.1"},
		{Key: "DB_PASSWORD", Value: "hunter2"},
	}
	var buf bytes.Buffer
	require.NoError(t, display.Print(&buf, entries, display.Options{}))
	out := buf.String()
	assert.Contains(t, out, "KEY")
	assert.Contains(t, out, "VALUE")
	assert.Contains(t, out, "APP_NAME")
	assert.Contains(t, out, "MyApp")
	// Sensitive values masked.
	assert.Contains(t, out, "****")
	assert.NotContains(t, out, "ultra-secret")
	assert.NotContains(t, out, "hunter2")
	assert.Contains(t, out, "Total: 4 keys")
	// Box drawing characters.
	assert.Contains(t, out, "┌")
	assert.Contains(t, out, "┘")
}

func TestPrint_TableNoMask(t *testing.T) {
	t.Parallel()
	entries := []display.Entry{
		{Key: "APP_KEY", Value: "ultra-secret"},
	}
	var buf bytes.Buffer
	require.NoError(t, display.Print(&buf, entries, display.Options{NoMask: true}))
	assert.Contains(t, buf.String(), "ultra-secret")
	assert.NotContains(t, buf.String(), "****")
}

func TestPrint_JSON(t *testing.T) {
	t.Parallel()
	entries := []display.Entry{
		{Key: "APP_NAME", Value: "MyApp"},
		{Key: "APP_KEY", Value: "ultra-secret"},
	}
	var buf bytes.Buffer
	require.NoError(t, display.Print(&buf, entries, display.Options{JSON: true}))

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "MyApp", parsed["APP_NAME"])
	assert.Equal(t, "****", parsed["APP_KEY"], "sensitive keys are masked in JSON")
}

func TestPrint_JSONNoMask(t *testing.T) {
	t.Parallel()
	entries := []display.Entry{
		{Key: "APP_KEY", Value: "ultra-secret"},
	}
	var buf bytes.Buffer
	require.NoError(t, display.Print(&buf, entries, display.Options{JSON: true, NoMask: true}))

	var parsed map[string]string
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "ultra-secret", parsed["APP_KEY"])
}

func TestPrint_JSONPreservesOrder(t *testing.T) {
	t.Parallel()
	entries := []display.Entry{
		{Key: "Z_LAST", Value: "1"},
		{Key: "A_FIRST", Value: "2"},
		{Key: "M_MIDDLE", Value: "3"},
	}
	var buf bytes.Buffer
	require.NoError(t, display.Print(&buf, entries, display.Options{JSON: true}))
	out := buf.String()
	// Z_LAST should appear before A_FIRST in the document, reflecting input order.
	iZ := strings.Index(out, "Z_LAST")
	iA := strings.Index(out, "A_FIRST")
	iM := strings.Index(out, "M_MIDDLE")
	assert.True(t, iZ >= 0 && iA >= 0 && iM >= 0)
	assert.Less(t, iZ, iA)
	assert.Less(t, iA, iM)
}

func TestPrint_TableEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, display.Print(&buf, nil, display.Options{}))
	out := buf.String()
	// Header row still renders; total should be zero.
	assert.Contains(t, out, "KEY")
	assert.Contains(t, out, "VALUE")
	assert.Contains(t, out, "Total: 0 keys")
}

type failingWriter struct {
	after int
	n     int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.n++
	if f.n > f.after {
		return 0, assertWriteErr{}
	}
	return len(p), nil
}

type assertWriteErr struct{}

func (assertWriteErr) Error() string { return "write failed" }

func TestPrint_JSONPropagatesWriteErrors(t *testing.T) {
	t.Parallel()
	entries := []display.Entry{
		{Key: "A", Value: "1"},
		{Key: "B", Value: "2"},
	}
	// Each successful write consumes one call; fail progressively earlier to
	// exercise every error path in printJSON ("{\n", one line per entry, "}\n").
	for after := 0; after < 5; after++ {
		after := after
		t.Run("fail_after_"+string(rune('0'+after)), func(t *testing.T) {
			t.Parallel()
			w := &failingWriter{after: after}
			err := display.Print(w, entries, display.Options{JSON: true})
			if after < 4 {
				assert.Error(t, err)
			}
		})
	}
}

func TestPrint_TablePropagatesWriteErrors(t *testing.T) {
	t.Parallel()
	entries := []display.Entry{{Key: "A", Value: "1"}}
	// Cover each boundary in printTable: top line, header row, separator,
	// data row, bottom line, summary.
	for after := 0; after < 7; after++ {
		after := after
		t.Run("fail_after_"+string(rune('0'+after)), func(t *testing.T) {
			t.Parallel()
			w := &failingWriter{after: after}
			err := display.Print(w, entries, display.Options{})
			if after < 6 {
				assert.Error(t, err)
			}
		})
	}
}

func TestPrint_MultibyteAlignment(t *testing.T) {
	t.Parallel()
	// Ensure each row has the same visual width.
	entries := []display.Entry{
		{Key: "GREETING", Value: "こんにちは"},
		{Key: "APP_NAME", Value: "MyApp"},
	}
	var buf bytes.Buffer
	require.NoError(t, display.Print(&buf, entries, display.Options{}))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	// Skip blank lines and the summary line.
	var tableLines []string
	for _, l := range lines {
		if strings.HasPrefix(l, "┌") || strings.HasPrefix(l, "│") || strings.HasPrefix(l, "├") || strings.HasPrefix(l, "└") {
			tableLines = append(tableLines, l)
		}
	}
	require.NotEmpty(t, tableLines)
	// Use display-width, not len(), to check alignment.
	widths := make(map[int]int)
	for _, l := range tableLines {
		w := 0
		for _, r := range l {
			switch {
			case r < 0x80:
				w++
			case r >= 0x2500 && r <= 0x257F: // box drawing
				w++
			default:
				w += 2
			}
		}
		widths[w]++
	}
	assert.Len(t, widths, 1, "all table lines should have equal display width, got %v", widths)
}
