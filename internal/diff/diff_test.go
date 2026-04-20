package diff_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mikadov/envlens/internal/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompare_KeysOnly(t *testing.T) {
	t.Parallel()
	a := diff.File{Name: ".env", Map: map[string]string{
		"APP_NAME":    "MyApp",
		"DB_HOST":     "localhost",
		"OLD_API_KEY": "deprecated",
	}}
	b := diff.File{Name: ".env.example", Map: map[string]string{
		"APP_NAME":          "MyApp",
		"DB_HOST":           "localhost",
		"STRIPE_SECRET_KEY": "sk_test",
		"MAIL_FROM_ADDRESS": "admin@example.com",
	}}

	res := diff.Compare(a, b, diff.CompareOptions{})
	assert.Equal(t, []string{"MAIL_FROM_ADDRESS", "STRIPE_SECRET_KEY"}, res.Missing)
	assert.Equal(t, []string{"OLD_API_KEY"}, res.Extra)
	assert.Empty(t, res.Changed) // disabled
	assert.False(t, res.Empty())
}

func TestCompare_WithValues(t *testing.T) {
	t.Parallel()
	a := diff.File{Name: ".env", Map: map[string]string{
		"APP_NAME": "MyApp",
		"DB_HOST":  "localhost",
	}}
	b := diff.File{Name: ".env.example", Map: map[string]string{
		"APP_NAME": "MyApp",          // same
		"DB_HOST":  "db.example.com", // different
	}}

	res := diff.Compare(a, b, diff.CompareOptions{WithValues: true})
	assert.Empty(t, res.Missing)
	assert.Empty(t, res.Extra)
	require.Len(t, res.Changed, 1)
	assert.Equal(t, "DB_HOST", res.Changed[0].Key)
	assert.Equal(t, "localhost", res.Changed[0].ValueA)
	assert.Equal(t, "db.example.com", res.Changed[0].ValueB)
}

func TestCompare_Empty(t *testing.T) {
	t.Parallel()
	m := map[string]string{"A": "1", "B": "2"}
	res := diff.Compare(
		diff.File{Name: "a", Map: m},
		diff.File{Name: "b", Map: m},
		diff.CompareOptions{WithValues: true},
	)
	assert.True(t, res.Empty())
}

func TestPrint_Empty(t *testing.T) {
	t.Parallel()
	res := &diff.Result{A: diff.File{Name: "a"}, B: diff.File{Name: "b"}}
	var buf bytes.Buffer
	require.NoError(t, diff.Print(&buf, res, diff.PrintOptions{}))
	assert.Contains(t, buf.String(), "No differences found.")
}

func TestPrint_KeysOnly(t *testing.T) {
	t.Parallel()
	res := &diff.Result{
		A:       diff.File{Name: ".env"},
		B:       diff.File{Name: ".env.example"},
		Missing: []string{"STRIPE_SECRET_KEY"},
		Extra:   []string{"OLD_API_KEY"},
	}
	var buf bytes.Buffer
	require.NoError(t, diff.Print(&buf, res, diff.PrintOptions{}))
	out := buf.String()
	assert.Contains(t, out, "Missing in .env (defined in .env.example):")
	assert.Contains(t, out, "STRIPE_SECRET_KEY")
	assert.Contains(t, out, "Extra in .env (not in .env.example):")
	assert.Contains(t, out, "OLD_API_KEY")
}

func TestPrint_WithValuesMasksSensitive(t *testing.T) {
	t.Parallel()
	res := &diff.Result{
		A: diff.File{Name: "a", Map: map[string]string{
			"APP_NAME": "ExtraApp",
			"OLD_KEY":  "should_be_masked",
		}},
		B: diff.File{Name: "b", Map: map[string]string{
			"STRIPE_SECRET_KEY": "sk_test_xxx",
			"DB_HOST":           "localhost",
		}},
		Missing: []string{"DB_HOST", "STRIPE_SECRET_KEY"},
		Extra:   []string{"APP_NAME", "OLD_KEY"},
	}
	var buf bytes.Buffer
	require.NoError(t, diff.Print(&buf, res, diff.PrintOptions{WithValues: true}))
	out := buf.String()
	// Sensitive values masked.
	assert.Contains(t, out, "STRIPE_SECRET_KEY = ****")
	assert.Contains(t, out, "OLD_KEY = ****")
	assert.NotContains(t, out, "sk_test_xxx")
	assert.NotContains(t, out, "should_be_masked")
	// Non-sensitive values shown.
	assert.Contains(t, out, "DB_HOST = localhost")
	assert.Contains(t, out, "APP_NAME = ExtraApp")
}

func TestPrint_ChangedValuesMasksSensitive(t *testing.T) {
	t.Parallel()
	res := &diff.Result{
		A: diff.File{Name: "a"},
		B: diff.File{Name: "b"},
		Changed: []diff.Change{
			{Key: "APP_KEY", ValueA: "secretA", ValueB: "secretB"},
			{Key: "DB_HOST", ValueA: "localhost", ValueB: "db.example.com"},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, diff.Print(&buf, res, diff.PrintOptions{WithValues: true}))
	out := buf.String()
	assert.Contains(t, out, "~ APP_KEY")
	assert.Contains(t, out, "a: ****")
	assert.Contains(t, out, "b: ****")
	assert.NotContains(t, out, "secretA")
	assert.NotContains(t, out, "secretB")
	assert.Contains(t, out, "a: localhost")
	assert.Contains(t, out, "b: db.example.com")
}

func TestPrint_Quiet(t *testing.T) {
	t.Parallel()
	res := &diff.Result{
		A:       diff.File{Name: "a"},
		B:       diff.File{Name: "b"},
		Missing: []string{"X"},
	}
	var buf bytes.Buffer
	require.NoError(t, diff.Print(&buf, res, diff.PrintOptions{Quiet: true}))
	assert.Empty(t, buf.String())
}

func TestPrint_ColorEnabled(t *testing.T) {
	t.Parallel()
	res := &diff.Result{
		A:       diff.File{Name: "a"},
		B:       diff.File{Name: "b"},
		Missing: []string{"X"},
		Extra:   []string{"Y"},
	}
	var buf bytes.Buffer
	require.NoError(t, diff.Print(&buf, res, diff.PrintOptions{Color: true}))
	assert.Contains(t, buf.String(), "\x1b[31m", "missing should be red")
	assert.Contains(t, buf.String(), "\x1b[33m", "extra should be yellow")
	assert.Contains(t, buf.String(), "\x1b[0m", "should reset")
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

func TestPrint_PropagatesWriteErrors(t *testing.T) {
	t.Parallel()
	res := &diff.Result{
		A:       diff.File{Name: "a"},
		B:       diff.File{Name: "b"},
		Missing: []string{"K1", "K2"},
		Extra:   []string{"K3"},
		Changed: []diff.Change{{Key: "K4", ValueA: "x", ValueB: "y"}},
	}
	// Fail at various write boundaries to exercise each error path.
	for after := 0; after < 10; after++ {
		after := after
		t.Run("fail_after_"+string(rune('0'+after)), func(t *testing.T) {
			t.Parallel()
			w := &failingWriter{after: after}
			err := diff.Print(w, res, diff.PrintOptions{WithValues: true})
			if after < 8 {
				assert.Error(t, err)
			}
		})
	}
	// Also cover the empty-result path.
	empty := &diff.Result{A: diff.File{Name: "a"}, B: diff.File{Name: "b"}}
	assert.Error(t, diff.Print(&failingWriter{after: 0}, empty, diff.PrintOptions{}))
}

func TestPrint_NoColorByDefault(t *testing.T) {
	t.Parallel()
	res := &diff.Result{
		A:       diff.File{Name: "a"},
		B:       diff.File{Name: "b"},
		Missing: []string{"X"},
	}
	var buf bytes.Buffer
	require.NoError(t, diff.Print(&buf, res, diff.PrintOptions{}))
	assert.False(t, strings.Contains(buf.String(), "\x1b["), "no ANSI codes when color disabled")
}
