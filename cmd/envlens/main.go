// Command envlens is a zero-dependency CLI for comparing, validating, and
// displaying .env files.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/Mikadov/envlens/internal/diff"
	"github.com/Mikadov/envlens/internal/display"
	"github.com/Mikadov/envlens/internal/parser"
	"github.com/Mikadov/envlens/internal/validate"
)

// version is overwritten at build time by GoReleaser via ldflags.
var version = "dev"

// Exit codes.
const (
	exitOK    = 0
	exitDiff  = 1 // diff/validate found differences or errors
	exitUsage = 2 // argument or I/O error
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return exitUsage
	}
	switch args[1] {
	case "--version", "-v", "version":
		fmt.Fprintln(stdout, versionString())
		return exitOK
	case "-h", "--help", "help":
		printUsage(stdout)
		return exitOK
	case "diff":
		return runDiff(args[2:], stdout, stderr)
	case "validate":
		return runValidate(args[2:], stdout, stderr)
	case "show":
		return runShow(args[2:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "envlens: unknown command %q\n\n", args[1])
		printUsage(stderr)
		return exitUsage
	}
}

func versionString() string {
	if version != "dev" {
		return "envlens v" + version
	}
	// GoReleaser injects the tag via ldflags, so the branch above covers
	// release binaries. When installed via `go install <module>@<tag>` there
	// is no ldflags step, so fall back to the module version recorded in the
	// build info.
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return "envlens " + v
		}
	}
	return "envlens dev"
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `envlens — compare, validate, and display .env files

Usage:
  envlens diff <file1> <file2> [--values] [--quiet] [--no-color]
  envlens validate <file> [--strict] [--no-color]
  envlens show <file> [--no-mask] [--json]
  envlens --version

Commands:
  diff       Compare keys (and optionally values) between two .env files.
             Exit code 1 on differences, 0 on clean.
  validate   Validate values against rules inferred from key name suffixes.
             Exit code 1 on errors, 0 on clean.
  show       Display the parsed .env contents as a table, with sensitive
             values masked by default.

Flags:
  --values    (diff)     Include value differences in output.
  --quiet     (diff)     Suppress output; only the exit code is meaningful.
  --strict    (validate) Warn on keys with no matching rule.
  --no-mask   (show)     Display values without masking sensitive keys.
  --json      (show)     Output JSON instead of a table.
  --no-color  (diff, validate) Disable ANSI color output.

Run 'envlens <command> --help' to see flags for a specific command.
Run 'envlens --version' (or 'envlens version') to print the version.
`)
}

// ---------------------------------------------------------------------------
// diff
// ---------------------------------------------------------------------------

func runDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	withValues := fs.Bool("values", false, "include value differences")
	quiet := fs.Bool("quiet", false, "suppress output; only the exit code is meaningful")
	noColor := fs.Bool("no-color", false, "disable ANSI color output")
	if code, done := parseSubArgs(fs, args, stdout, stderr); done {
		return code
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "envlens diff: expected exactly two file arguments")
		return exitUsage
	}
	fileA, fileB := fs.Arg(0), fs.Arg(1)

	resA, err := parser.ParseFile(fileA)
	if err != nil {
		fmt.Fprintf(stderr, "envlens: %v\n", err)
		return exitUsage
	}
	printWarnings(stderr, fileA, resA.Warnings)
	resB, err := parser.ParseFile(fileB)
	if err != nil {
		fmt.Fprintf(stderr, "envlens: %v\n", err)
		return exitUsage
	}
	printWarnings(stderr, fileB, resB.Warnings)

	out := diff.Compare(
		diff.File{Name: fileA, Map: resA.Map()},
		diff.File{Name: fileB, Map: resB.Map()},
		diff.CompareOptions{WithValues: *withValues},
	)
	if err := diff.Print(stdout, out, diff.PrintOptions{
		Color:      shouldColor(stdout, *noColor),
		WithValues: *withValues,
		Quiet:      *quiet,
	}); err != nil {
		fmt.Fprintf(stderr, "envlens: %v\n", err)
		return exitUsage
	}
	if out.Empty() {
		return exitOK
	}
	return exitDiff
}

// ---------------------------------------------------------------------------
// validate
// ---------------------------------------------------------------------------

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	strict := fs.Bool("strict", false, "warn on keys with no matching rule")
	noColor := fs.Bool("no-color", false, "disable ANSI color output")
	if code, done := parseSubArgs(fs, args, stdout, stderr); done {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "envlens validate: expected exactly one file argument")
		return exitUsage
	}

	res, err := parser.ParseFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "envlens: %v\n", err)
		return exitUsage
	}
	printWarnings(stderr, fs.Arg(0), res.Warnings)
	inputs := make([]validate.Input, 0, len(res.Entries))
	for _, e := range res.Entries {
		inputs = append(inputs, validate.Input{Key: e.Key, Value: e.Value})
	}

	results := validate.All(inputs, *strict)
	printValidate(stdout, results, shouldColor(stdout, *noColor))

	errs := validate.CountErrors(results)
	if errs > 0 {
		return exitDiff
	}
	return exitOK
}

const (
	symCheck = "✓"
	symCross = "✗"
	symQ     = "?"
)

func printValidate(w io.Writer, results []validate.Result, color bool) {
	const (
		red    = "\x1b[31m"
		green  = "\x1b[32m"
		yellow = "\x1b[33m"
		reset  = "\x1b[0m"
	)
	errs := 0
	for _, r := range results {
		switch {
		case r.Rule == nil:
			// Appears only in strict mode.
			line := fmt.Sprintf("%s %s  no rule defined", symQ, r.Key)
			fmt.Fprintln(w, colorWrap(color, yellow, reset, line))
		case r.Err != nil:
			errs++
			line := fmt.Sprintf("%s %s  %s", symCross, r.Key, r.Err.Error())
			fmt.Fprintln(w, colorWrap(color, red, reset, line))
		default:
			line := fmt.Sprintf("%s %s  OK", symCheck, r.Key)
			fmt.Fprintln(w, colorWrap(color, green, reset, line))
		}
	}
	fmt.Fprintln(w)
	if errs == 0 {
		fmt.Fprintln(w, colorWrap(color, green, reset, "No errors found."))
		return
	}
	fmt.Fprintf(w, "%d error(s) found.\n", errs)
}

func colorWrap(enabled bool, prefix, suffix, s string) string {
	if !enabled {
		return s
	}
	return prefix + s + suffix
}

// ---------------------------------------------------------------------------
// show
// ---------------------------------------------------------------------------

func runShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	noMask := fs.Bool("no-mask", false, "display values without masking")
	asJSON := fs.Bool("json", false, "output JSON instead of a table")
	if code, done := parseSubArgs(fs, args, stdout, stderr); done {
		return code
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "envlens show: expected exactly one file argument")
		return exitUsage
	}

	res, err := parser.ParseFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "envlens: %v\n", err)
		return exitUsage
	}
	printWarnings(stderr, fs.Arg(0), res.Warnings)

	keys, m := res.Ordered()
	entries := make([]display.Entry, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, display.Entry{Key: k, Value: m[k]})
	}

	if err := display.Print(stdout, entries, display.Options{
		NoMask: *noMask,
		JSON:   *asJSON,
	}); err != nil {
		fmt.Fprintf(stderr, "envlens: %v\n", err)
		return exitUsage
	}
	return exitOK
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// printWarnings emits parser warnings to stderr, one per line, prefixed with
// "envlens: <path>: ". Warnings are informational (duplicate keys, unmatched
// quotes, invalid identifiers, etc.) and never affect the exit code.
func printWarnings(stderr io.Writer, path string, warnings []parser.Warning) {
	for _, w := range warnings {
		fmt.Fprintf(stderr, "envlens: %s: %s\n", path, w.String())
	}
}

// parseSubArgs drives fs.Parse while routing output correctly: when the user
// asked for help (-h / --help), the flag library's auto-generated usage goes
// to stdout and the process exits successfully; other parse errors go to
// stderr and exit with code 2. The second return reports whether the caller
// should return immediately with the provided exit code.
func parseSubArgs(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (int, bool) {
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	err := fs.Parse(args)
	switch {
	case errors.Is(err, flag.ErrHelp):
		_, _ = io.Copy(stdout, &buf)
		return exitOK, true
	case err != nil:
		_, _ = io.Copy(stderr, &buf)
		return exitUsage, true
	}
	return 0, false
}

// shouldColor reports whether ANSI colors should be emitted. Colors are
// disabled when the user passed --no-color, when NO_COLOR is set, when TERM
// is "dumb", or when the output is not a terminal.
func shouldColor(w io.Writer, noColorFlag bool) bool {
	if noColorFlag {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
