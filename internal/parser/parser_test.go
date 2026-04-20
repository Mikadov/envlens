package parser_test

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/Mikadov/envlens/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFile_Basic(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/basic.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"APP_NAME": "MyApp",
		"DB_HOST":  "localhost",
		"APP_ENV":  "production",
	}, res.Map())
	assert.Equal(t, []string{"APP_NAME", "DB_HOST", "APP_ENV"}, res.Keys())
}

func TestParseFile_Comments(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/comments.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"APP_NAME": "MyApp",
		"DB_HOST":  "localhost",
		"APP_KEY":  "secret",
	}, res.Map())
}

func TestParseFile_Quoted(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/quoted.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"DOUBLE":           "double quoted",
		"SINGLE":           "single quoted",
		"EMPTY_DOUBLE":     "",
		"EMPTY_SINGLE":     "",
		"WITH_HASH":        "value # not a comment",
		"WITH_SPACE":       "hello world",
		"TRAILING_COMMENT": "ok",
	}, res.Map())
}

func TestParseFile_ExportPrefix(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/export.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"APP_NAME": "MyApp",
		"DB_HOST":  "localhost",
		"DB_PORT":  "5432",
	}, res.Map())
}

func TestParseFile_Empty(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/empty.env")
	require.NoError(t, err)
	assert.Empty(t, res.Entries)
	assert.Empty(t, res.Warnings)
}

func TestParseFile_CommentOnly(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/comment_only.env")
	require.NoError(t, err)
	assert.Empty(t, res.Entries)
	assert.Empty(t, res.Warnings)
}

func TestParseFile_Duplicates(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/duplicates.env")
	require.NoError(t, err)
	// Last value wins in Map().
	assert.Equal(t, "third", res.Map()["KEY"])
	assert.Equal(t, "one", res.Map()["OTHER"])
	// Two duplicate warnings (lines 3 and 4).
	assert.Len(t, res.Warnings, 2)
	for _, w := range res.Warnings {
		assert.Contains(t, w.Message, `duplicate key "KEY"`)
	}
	// All four entries preserved in order.
	require.Len(t, res.Entries, 4)
	assert.Equal(t, "first", res.Entries[0].Value)
	assert.Equal(t, "third", res.Entries[3].Value)
}

func TestParseFile_EmptyValues(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/empty_values.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"APP_NAME":   "",
		"APP_KEY":    "",
		"APP_SECRET": "",
	}, res.Map())
}

func TestParseFile_Multibyte(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/multibyte.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"GREETING": "こんにちは",
		"EMOJI":    "🚀",
		"MIXED":    "héllo wörld",
	}, res.Map())
}

func TestParseFile_HashInValue(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/hash_in_value.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"FRAGMENT":    "http://example.com/page#section",
		"COLOR":       "#ff0000",
		"NOT_COMMENT": "value#still_value",
		"HAS_COMMENT": "value",
	}, res.Map())
}

func TestParseFile_Interpolation(t *testing.T) {
	t.Parallel()
	// v1 leaves ${...} literal — no interpolation, no warning.
	res, err := parser.ParseFile("testdata/interpolation.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, "${BASE_URL}/v1", res.Map()["APP_URL"])
}

func TestParseFile_UnmatchedQuote(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/unmatched_quote.env")
	require.NoError(t, err)
	// BEFORE + AFTER parsed; BROKEN skipped with warning.
	assert.Equal(t, map[string]string{
		"BEFORE": "ok",
		"AFTER":  "still_works",
	}, res.Map())
	require.Len(t, res.Warnings, 1)
	assert.Contains(t, res.Warnings[0].Message, "unmatched")
	assert.Equal(t, 2, res.Warnings[0].Line)
}

func TestParseFile_UnsupportedLines(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/unsupported.env")
	require.NoError(t, err)
	// VALID and VALID2 parsed; the three malformed lines produce warnings.
	assert.Equal(t, map[string]string{
		"VALID":  "yes",
		"VALID2": "ok",
	}, res.Map())
	require.Len(t, res.Warnings, 3)
}

func TestParseFile_WindowsCRLF(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/windows_crlf.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	assert.Equal(t, map[string]string{
		"APP_NAME": "MyApp",
		"DB_HOST":  "localhost",
		"DB_PORT":  "5432",
	}, res.Map())
}

func TestParseFile_UTF8BOM(t *testing.T) {
	t.Parallel()
	res, err := parser.ParseFile("testdata/bom.env")
	require.NoError(t, err)
	assert.Empty(t, res.Warnings)
	// Key must NOT contain the BOM.
	_, ok := res.Map()["APP_NAME"]
	assert.True(t, ok, "APP_NAME should be present (BOM stripped)")
	assert.Equal(t, "MyApp", res.Map()["APP_NAME"])
	assert.Equal(t, "localhost", res.Map()["DB_HOST"])
}

func TestParseFile_Missing(t *testing.T) {
	t.Parallel()
	_, err := parser.ParseFile("testdata/does_not_exist.env")
	require.Error(t, err)
}

func TestParse_ReaderInline(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		wantMap  map[string]string
		wantWarn int
	}{
		{
			name:    "single_line",
			input:   "A=1",
			wantMap: map[string]string{"A": "1"},
		},
		{
			name:    "spaces_around_equals",
			input:   "A = 1",
			wantMap: map[string]string{"A": "1"},
		},
		{
			name:    "tab_before_inline_comment",
			input:   "A=value\t# tab comment",
			wantMap: map[string]string{"A": "value"},
		},
		{
			name:    "trailing_whitespace_trimmed",
			input:   "A=value   ",
			wantMap: map[string]string{"A": "value"},
		},
		{
			name:    "export_with_tab",
			input:   "export\tFOO=bar",
			wantMap: map[string]string{"FOO": "bar"},
		},
		{
			name:     "invalid_identifier_digit_start",
			input:    "1BAD=1",
			wantMap:  map[string]string{},
			wantWarn: 1,
		},
		{
			name:     "key_with_space_rejected",
			input:    "BAD KEY=1",
			wantMap:  map[string]string{},
			wantWarn: 1,
		},
		{
			name:     "non_ascii_key_rejected",
			input:    "こんにちは=1",
			wantMap:  map[string]string{},
			wantWarn: 1,
		},
		{
			name:     "mixed_ascii_non_ascii_key_rejected",
			input:    "APPこんにちは=1",
			wantMap:  map[string]string{},
			wantWarn: 1,
		},
		{
			name:    "blank_lines_ignored",
			input:   "\n\nA=1\n\n",
			wantMap: map[string]string{"A": "1"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res, err := parser.Parse(strings.NewReader(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.wantMap, res.Map())
			assert.Len(t, res.Warnings, tt.wantWarn)
		})
	}
}

func TestWarning_String(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "line 5: boom", parser.Warning{Line: 5, Message: "boom"}.String())
	assert.Equal(t, "no-line msg", parser.Warning{Line: 0, Message: "no-line msg"}.String())
}

func TestResult_Ordered(t *testing.T) {
	t.Parallel()
	res, err := parser.Parse(strings.NewReader("A=1\nB=2\nA=3\nC=4\n"))
	require.NoError(t, err)
	keys, m := res.Ordered()
	assert.Equal(t, []string{"A", "B", "C"}, keys)
	assert.Equal(t, map[string]string{"A": "3", "B": "2", "C": "4"}, m)
}

func TestParse_ReaderErrorPropagates(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	_, err := parser.Parse(iotest.ErrReader(sentinel))
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "read:")
}
