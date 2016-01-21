package discover

import (
	"regexp"
	"strings"
)

// simple patterns for simplified url matching

var patReplacer *strings.Replacer = strings.NewReplacer(
	"ID", `([0-9]{4,})`,
	"SLUG", `([^/]+-[^/]+)`,
	"YYYY", `(\d\d\d\d)`,
	"MM", `(\d\d)`,
	"DD", `(\d\d)`,
)

// turn a simplified pattern into a regexp
func patToRegexp(in string) (*regexp.Regexp, error) {
	suffix := ""
	// don't want to escape a trailing '$' if it's there....
	if strings.HasSuffix(in, "$") {
		in = in[0 : len(in)-1] // assumes single-byte rune...
		suffix = "$"
	}

	in = regexp.QuoteMeta(in)
	in = in + suffix
	in = patReplacer.Replace(in)

	return regexp.Compile(in)
}
