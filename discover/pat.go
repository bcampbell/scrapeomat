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
	in = regexp.QuoteMeta(in)
	in = in + "$"
	in = patReplacer.Replace(in)

	return regexp.Compile(in)
}
