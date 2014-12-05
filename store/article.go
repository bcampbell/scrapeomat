package store

import (
	"github.com/bcampbell/arts/arts"
)

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

type Publication struct {
	Code   string `json:"code"` // short unique code for publication
	Name   string `json:"name,omitempty"`
	Domain string `json:"domain,omitempty"`

	// TODO: add publication versions of rel-author
	// eg "article:publisher", rel-publisher
}

type Article struct {
	CanonicalURL string `json:"canonical_url,omitempty"`
	// all known URLs for article (including canonical)
	// TODO: first url should be considered "preferred" if no canonical?
	URLs     []string `json:"urls,omitempty"`
	Headline string   `json:"headline,omitempty"`
	Authors  []Author `json:"authors,omitempty"`
	Content  string   `json:"content,omitempty"`
	// Published contains date of publication.
	// An ISO8601 string is used instead of time.Time, so that
	// less-precise representations can be held (eg YYYY-MM)
	Published   string      `json:"published,omitempty"`
	Updated     string      `json:"updated,omitempty"`
	Publication Publication `json:"publication,omitempty"`
	Keywords    []Keyword   `json:"keywords,omitempty"`
}

// copy an arts.Article into our struct
func ConvertArticle(src *arts.Article) *Article {
	art := &Article{
		CanonicalURL: src.CanonicalURL,
		URLs:         make([]string, len(src.URLs)),
		Headline:     src.Headline,
		Authors:      make([]Author, len(src.Authors)),
		Content:      src.Content,
		Published:    src.Published,
		Updated:      src.Updated,
		Publication: Publication{
			Name:   src.Publication.Name,
			Domain: src.Publication.Domain,
		},
		Keywords: make([]Keyword, len(src.Keywords)),
	}

	for i, u := range src.URLs {
		art.URLs[i] = u
	}
	for i, a := range src.Authors {
		art.Authors[i] = Author{Name: a.Name, RelLink: a.RelLink, Email: a.Email, Twitter: a.Twitter}
	}
	for i, kw := range src.Keywords {
		art.Keywords[i] = Keyword{Name: kw.Name, URL: kw.URL}
	}
	return art
}
