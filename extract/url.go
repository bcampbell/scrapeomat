package extract

import (
	neturl "net/url"
	"regexp"
	// "github.com/andybalholm/cascadia"
)

var (
	//	selLink         = cascadia.MustCompile(`a`)
	//	selRelCanonical = cascadia.MustCompile(`link[rel="canonical"]`)
	reSlugDate  = regexp.MustCompile(`/\d{4}/\d{2}/\d{2}/`)
	reNumericID = regexp.MustCompile(`\d{5,}`)
	reSlug      = regexp.MustCompile(`(?i)/([a-z0-9]+(?:[-_.][-_.%a-z0-9]+)+)/?$`)
)

func SlugFromURL(u *neturl.URL) string {
	path := u.EscapedPath()
	m := reSlug.FindStringSubmatch(path)
	if m == nil {
		return ""
	} else {
		return m[1]
	}
}

//return strings.ToLower(ReplaceAll(slug, "_", "-"))
