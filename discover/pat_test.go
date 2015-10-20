package discover

import (
	//"fmt"
	//	"regexp"
	"testing"
)

func TestPatToRegexp(t *testing.T) {
	data := []struct {
		pat     string
		match   []string
		noMatch []string
	}{
		{
			pat: "/ID-SLUG",
			match: []string{
				"/news/space/12345-moon-made-of-cheese",
				"/1234-blah-blah"},
			noMatch: []string{},
		},
		{
			pat: "/YYYY/MM/SLUG.html",
			match: []string{
				"/2001/04/moon-made-of-cheese.html",
			},
			noMatch: []string{
				"/2001/04/moon-made-of-cheese",
			},
		},
	}
	/*
		pats := []string{
			"/ID-SLUG",
			"/SLUG.html",
		}
	*/

	for _, dat := range data {
		re, err := patToRegexp(dat.pat)
		if err != nil {
			t.Errorf("%q failed to compile: %s", dat.pat, err)
			continue
		}
		for _, u := range dat.match {
			if !re.MatchString(u) {
				t.Errorf("%q didn't match %q", dat.pat, u)
			}
		}
		for _, u := range dat.noMatch {
			if re.MatchString(u) {
				t.Errorf("%q incorrectly matched %q", dat.pat, u)
			}
		}
	}
}
