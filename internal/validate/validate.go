package validate

// Input is a single key/value to validate. It is decoupled from parser.Entry
// so that this package stays independent of the parser.
type Input struct {
	Key   string
	Value string
}

// Result is the outcome for one key.
type Result struct {
	Key   string
	Value string
	Rule  *Rule // nil when no rule matched
	Err   error // nil when the value passed, or when no rule matched
}

// OK reports whether the result represents a passing validation.
func (r Result) OK() bool {
	return r.Err == nil && r.Rule != nil
}

// All runs validation over every input. Keys without a matching rule are
// dropped from the output unless strict is true, in which case they appear
// with Rule=nil and Err=nil.
func All(inputs []Input, strict bool) []Result {
	out := make([]Result, 0, len(inputs))
	for _, in := range inputs {
		rule := MatchRule(in.Key)
		if rule == nil {
			if strict {
				out = append(out, Result{Key: in.Key, Value: in.Value})
			}
			continue
		}
		r := Result{Key: in.Key, Value: in.Value, Rule: rule}
		if err := rule.Validate(in.Value); err != nil {
			r.Err = err
		}
		out = append(out, r)
	}
	return out
}

// CountErrors returns the number of results with a non-nil Err.
func CountErrors(results []Result) int {
	n := 0
	for _, r := range results {
		if r.Err != nil {
			n++
		}
	}
	return n
}
