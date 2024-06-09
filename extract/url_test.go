package extract

import (
	neturl "net/url"
	"testing"
)

func TestSlugFromURL(t *testing.T) {
	data := []struct{ url, slug string }{
		{"https://example.com/foo/this-is-a-slug", "this-is-a-slug"},
		{"https://example.com/foo/this_is_a_slug", "this_is_a_slug"},
		{"https://example.com/foo/This-is-a-Slug", "This-is-a-Slug"},
		{"https://example.com/foo/this-is-a-slug/", "this-is-a-slug"},
		{"https://example.com/foo/this_is_a_slug/", "this_is_a_slug"},
		{"https://example.com/foo/this-is-a-slug-/", "this-is-a-slug-"},
		{"https://example.com/foo/foobar-1/", "foobar-1"},
		{"https://example.com/not-a-slug/blah/", ""},
		{"https://example.com/short-one", "short-one"},
		{"https://example.com/foo", ""},
	}

	for _, d := range data {
		u, err := neturl.Parse(d.url)
		if err != nil {
			t.Fatalf("url.Parse failed: %s\n", err)
		}

		got := GetSlug(u)
		if got != d.slug {
			t.Fatalf(`GetSlug("%s") = "%s", want "%s"`, d.url, got, d.slug)
		}
	}
}
