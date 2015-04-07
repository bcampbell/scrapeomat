package slurp

import (
//	"fmt"
)

type Publication struct {
	// Code is a short, unique name (eg "mirror")
	Code string `json:"code"`
	// Name is the 'pretty' name (eg "The Daily Mirror")
	Name   string `json:"name,omitempty"`
	Domain string `json:"domain,omitempty"`
}

type Author struct {
	Name    string `json:"name"`
	RelLink string `json:"rel_link,omitempty"`
	Email   string `json:"email,omitempty"`
	Twitter string `json:"twitter,omitempty"`
}

type Keyword struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// wire format for article data
type Article struct {
	//ID           int
	CanonicalURL string `json:"canonical_url"`

	// all known URLs for article (including canonical)
	// TODO: first url should be considered "preferred" if no canonical?
	URLs []string `json:"urls"`

	Headline string   `json:"headline"`
	Authors  []Author `json:"authors,omitempty"`

	// Content contains HTML, sanitised using a subset of tags
	Content string `json:"content"`

	// Published contains date of publication.
	// An ISO8601 string is used instead of time.Time, so that
	// less-precise representations can be held (eg YYYY-MM)
	Published   string      `json:"published,omitempty"`
	Updated     string      `json:"updated,omitempty"`
	Publication Publication `json:"publication,omitempty"`
	// Keywords contains data from rel-tags, meta keywords etc...
	Keywords []Keyword `json:"keywords,omitempty"`
	Section  string    `json:"section,omitempty"`
}
