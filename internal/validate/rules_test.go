package validate_test

import (
	"testing"

	"github.com/Mikadov/envlens/internal/validate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key      string
		wantName string // "" => no match
	}{
		{"APP_URL", "url"},
		{"DATABASE_URI", "url"},
		{"app_url", "url"}, // case-insensitive
		{"DB_PORT", "port"},
		{"APP_KEY", "secret"},
		{"STRIPE_SECRET", "secret"},
		{"GITHUB_TOKEN", "secret"},
		{"DB_PASSWORD", "secret"},
		{"FROM_EMAIL", "email"},
		{"MAIL_FROM_ADDRESS", "email"},
		{"FEATURE_ENABLED", "bool"},
		{"APP_DEBUG", "bool"},
		{"DARK_MODE_FLAG", "bool"},
		{"APP_NAME", ""}, // no match
		{"DB_HOST", ""},  // no match
		{"URLISH", ""},   // suffix must start with underscore
		{"", ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			r := validate.MatchRule(tt.key)
			if tt.wantName == "" {
				assert.Nil(t, r)
				return
			}
			require.NotNil(t, r)
			assert.Equal(t, tt.wantName, r.Name)
		})
	}
}

func TestShouldMask(t *testing.T) {
	t.Parallel()
	assert.True(t, validate.ShouldMask("APP_KEY"))
	assert.True(t, validate.ShouldMask("STRIPE_SECRET"))
	assert.True(t, validate.ShouldMask("GITHUB_TOKEN"))
	assert.True(t, validate.ShouldMask("DB_PASSWORD"))
	assert.False(t, validate.ShouldMask("APP_URL"))
	assert.False(t, validate.ShouldMask("DB_PORT"))
	assert.False(t, validate.ShouldMask("APP_NAME"))
}

func TestRuleValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
	}{
		// URL — scheme-qualified
		{"url_ok_http", "APP_URL", "http://example.com", false},
		{"url_ok_https", "APP_URL", "https://example.com/path", false},
		{"url_ok_postgres", "DATABASE_URL", "postgres://user:pass@host:5432/db", false},
		{"url_ok_http_localhost", "APP_URL", "http://localhost:3000", false},
		// URL — bare hosts (permitted)
		{"url_ok_bare_localhost", "APP_URL", "localhost", false},
		{"url_ok_bare_ipv4", "APP_URL", "127.0.0.1", false},
		{"url_ok_bare_domain", "APP_URL", "example.com", false},
		// URL — rejected
		{"url_bad_empty", "APP_URL", "", true},
		{"url_bad_no_host", "APP_URL", "http://", true},
		{"url_bad_whitespace", "APP_URL", "foo bar", true},
		{"url_bad_leading_sep", "APP_URL", "://missing-scheme", true},
		// URL — bare host tightening: must be "localhost" or contain "."
		{"url_bad_bare_garbage", "APP_URL", "garbage", true},
		{"url_bad_bare_typo", "APP_URL", "todo", true},
		{"url_ok_bare_localhost_upper", "APP_URL", "LOCALHOST", false},
		// URL — bare host with port
		{"url_ok_bare_localhost_port", "APP_URL", "localhost:3000", false},
		{"url_ok_bare_ipv4_port", "APP_URL", "127.0.0.1:5432", false},
		{"url_ok_bare_domain_port", "APP_URL", "example.com:443", false},
		{"url_bad_bare_garbage_port", "APP_URL", "garbage:3000", true},
		// URL — file URIs (empty host is legitimate)
		{"url_ok_file_absolute", "APP_URL", "file:///etc/config", false},
		{"url_ok_file_with_host", "APP_URL", "file://localhost/etc/config", false},
		// Port
		{"port_ok_min", "DB_PORT", "1", false},
		{"port_ok_max", "DB_PORT", "65535", false},
		{"port_ok_common", "DB_PORT", "5432", false},
		{"port_bad_zero", "DB_PORT", "0", true},
		{"port_bad_too_large", "DB_PORT", "65536", true},
		{"port_bad_negative", "DB_PORT", "-1", true},
		{"port_bad_non_numeric", "DB_PORT", "abc", true},
		{"port_bad_empty", "DB_PORT", "", true},
		// Secret (non-empty)
		{"secret_ok", "APP_KEY", "anything", false},
		{"secret_bad_empty", "APP_KEY", "", true},
		{"token_ok", "GITHUB_TOKEN", "ghp_xxx", false},
		{"password_bad_empty", "DB_PASSWORD", "", true},
		// Email
		{"email_ok", "FROM_EMAIL", "user@example.com", false},
		{"email_ok_dotted", "FROM_EMAIL", "user.name+tag@sub.example.com", false},
		{"email_bad_no_at", "FROM_EMAIL", "user_example.com", true},
		{"email_bad_display_name", "FROM_EMAIL", "Name <user@example.com>", true},
		{"email_bad_empty", "FROM_EMAIL", "", true},
		// Bool
		{"bool_true", "APP_DEBUG", "true", false},
		{"bool_false", "APP_DEBUG", "false", false},
		{"bool_one", "APP_DEBUG", "1", false},
		{"bool_zero", "APP_DEBUG", "0", false},
		{"bool_mixed_case", "APP_DEBUG", "TRUE", false},
		{"bool_bad_yes", "APP_DEBUG", "yes", true},
		{"bool_bad_empty", "APP_DEBUG", "", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rule := validate.MatchRule(tt.key)
			require.NotNil(t, rule, "key %q should match a rule", tt.key)
			err := rule.Validate(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAll(t *testing.T) {
	t.Parallel()
	inputs := []validate.Input{
		{Key: "APP_URL", Value: "https://example.com"},
		{Key: "DB_PORT", Value: "abc"},
		{Key: "APP_KEY", Value: "secret"},
		{Key: "APP_NAME", Value: "MyApp"}, // no rule
	}

	t.Run("non_strict_skips_unmatched", func(t *testing.T) {
		t.Parallel()
		results := validate.All(inputs, false)
		// APP_NAME should be skipped.
		require.Len(t, results, 3)
		keys := make([]string, 0, len(results))
		for _, r := range results {
			keys = append(keys, r.Key)
		}
		assert.Equal(t, []string{"APP_URL", "DB_PORT", "APP_KEY"}, keys)
		assert.Equal(t, 1, validate.CountErrors(results))
	})

	t.Run("strict_includes_unmatched", func(t *testing.T) {
		t.Parallel()
		results := validate.All(inputs, true)
		require.Len(t, results, 4)
		// Unmatched entry has Rule=nil and Err=nil.
		var found bool
		for _, r := range results {
			if r.Key == "APP_NAME" {
				assert.Nil(t, r.Rule)
				assert.NoError(t, r.Err)
				found = true
			}
		}
		assert.True(t, found)
	})
}

func TestResultOK(t *testing.T) {
	t.Parallel()
	rule := validate.MatchRule("APP_URL")
	require.NotNil(t, rule)

	assert.True(t, validate.Result{Key: "APP_URL", Rule: rule}.OK())
	assert.False(t, validate.Result{Key: "APP_URL", Rule: rule, Err: assertErr{}}.OK())
	assert.False(t, validate.Result{Key: "APP_NAME"}.OK(), "no rule match is not OK")
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
