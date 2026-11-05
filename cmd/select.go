package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/benkenawell/dotenv/internal/dotenv"
)

// occurrenceTable renders occurrences of key in the same layout as "ls --all",
// for use in error messages.
func occurrenceTable(key string, occ []dotenv.Occurrence) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Line\tKey\tValue\tDescription")
	for _, o := range occ {
		fmt.Fprintf(w, "%4d\t%s\t%s\t%s\n", o.Line, displayKey(key, o.Disabled), oneLine(o.Value), strings.ReplaceAll(o.Description, "\n", " "))
	}
	w.Flush()
	return strings.TrimRight(b.String(), "\n")
}

// selectOccurrence resolves KEY plus an optional VALUE or --line selector to
// exactly one occurrence. candidates filters which occurrences may be chosen
// when no selector is given (nil means all); hint names the command to use
// when the choice is ambiguous.
func selectOccurrence(f *dotenv.File, path, key string, value *string, line int, candidates func(dotenv.Occurrence) bool, hint string) (dotenv.Occurrence, error) {
	all := f.Occurrences(key)
	if len(all) == 0 {
		return dotenv.Occurrence{}, fmt.Errorf("%s: key %q not found", path, key)
	}
	if line > 0 {
		for _, o := range all {
			if o.Line == line {
				return o, nil
			}
		}
		return dotenv.Occurrence{}, fmt.Errorf("%s: no occurrence of %q at line %d\n%s", path, key, line, occurrenceTable(key, all))
	}
	if value != nil {
		var match []dotenv.Occurrence
		for _, o := range all {
			if o.Value == *value {
				match = append(match, o)
			}
		}
		switch len(match) {
		case 0:
			return dotenv.Occurrence{}, fmt.Errorf("%s: no occurrence of %q with value %q\n%s", path, key, *value, occurrenceTable(key, all))
		case 1:
			return match[0], nil
		default:
			return dotenv.Occurrence{}, fmt.Errorf("%s: %d occurrences of %q have value %q; choose one with --line\n%s", path, len(match), key, *value, occurrenceTable(key, match))
		}
	}
	pool := all
	if candidates != nil {
		pool = nil
		for _, o := range all {
			if candidates(o) {
				pool = append(pool, o)
			}
		}
	}
	switch len(pool) {
	case 0:
		return dotenv.Occurrence{}, errNoCandidates
	case 1:
		return pool[0], nil
	default:
		return dotenv.Occurrence{}, fmt.Errorf("%s: %q has %d occurrences; pass a VALUE or --line%s\n%s", path, key, len(pool), hint, occurrenceTable(key, pool))
	}
}

// errNoCandidates signals that the key exists but nothing matched the
// candidate filter; callers decide whether that is an error.
var errNoCandidates = fmt.Errorf("no candidates")

// valueArg returns args[1] as a selector when present.
func valueArg(args []string) *string {
	if len(args) > 1 {
		return &args[1]
	}
	return nil
}
