package main

// in theory this could be broken out into a standalone
// wp-json client, but really should just replace it with an existing one.

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

// Client holds everything we need to perform WP api queries.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string // eg "https://example.com/wp-json"
	Verbose    bool
}

//
type Tag struct {
	ID          int    `json:"id"`
	Count       int    `json:"count"`
	Description string `json:"description"`
	Link        string `json:"link"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Taxonomy    string `json:"taxonomy"`
}

type Category struct {
	ID          int    `json:"id"`
	Count       int    `json:"count"`
	Description string `json:"description"`
	Link        string `json:"link"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Taxonomy    string `json:"taxonomy"`
	Parent      int    `json:"parent"`
}

// Post data returned from wp/posts endpoint
type Post struct {
	Link  string `json:"link"`
	Title struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
	Date     string `json:"date"`
	Modified string `json:"modified"`
	Content  struct {
		Rendered string `json:"rendered"`
	} `json:"content"`
	Tags       []int `json:"tags"`
	Categories []int `json:"categories"`
}

// fetch a list of posts ("/wp/v2/posts)
// returns expected number of posts, posts, error
func (wp *Client) ListPosts(params url.Values) ([]*Post, int, error) {
	u := wp.BaseURL + "/wp/v2/posts?" + params.Encode()

	if wp.Verbose {
		fmt.Fprintf(os.Stderr, "fetch %s\n", u)
	}

	resp, err := wp.HTTPClient.Get(u)
	if err != nil {
		return nil, 0, err
	}

	// totalpages is returned as a header
	expectedTotal, err := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	if err != nil {
		return nil, 0, err
	}
	raw, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("%s: %d\n", u, resp.StatusCode)
	}

	posts := []*Post{}

	err = json.Unmarshal(raw, &posts)
	if err != nil {
		return nil, 0, err
	}

	return posts, expectedTotal, nil
}

// ListPostsAll repeatedly calls ListPosts until the whole set has been
// retrieved. The pagination-related params will overriden, but all others
// will be passed on verbatim with each request.
// The postSink callback will be invoked as each batch of posts is reeceived.
func (wp *Client) ListPostsAll(params url.Values, postSink func([]*Post, int) error) error {
	perPage := 100
	numReceived := 0

	for {
		// We override pagination-related params
		params.Set("per_page", strconv.Itoa(perPage))
		params.Set("offset", strconv.Itoa(numReceived))
		params.Del("page")

		batch, expectedTotal, err := wp.ListPosts(params)
		if err != nil {
			return err
		}

		err = postSink(batch, expectedTotal)
		if err != nil {
			return err
		}
		numReceived += len(batch)

		fmt.Fprintf(os.Stderr, "received %d/%d\n", numReceived, expectedTotal)
		if len(batch) == 0 || numReceived >= expectedTotal {
			break
		}
	}
	return nil
}

// GET /wp/v2/tags
// params we're interested in:
func (wp *Client) ListTags(params url.Values) ([]*Tag, int, error) {
	u := wp.BaseURL + "/wp/v2/tags?" + params.Encode()

	if wp.Verbose {
		fmt.Fprintf(os.Stderr, "fetch %s\n", u)
	}

	resp, err := wp.HTTPClient.Get(u)
	if err != nil {
		return nil, 0, err
	}

	// totalpages is returned as a header
	expectedTotal, err := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	if err != nil {
		return nil, 0, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("%s: %d\n", u, resp.StatusCode)
	}

	tags := []*Tag{}

	err = json.Unmarshal(raw, &tags)
	if err != nil {
		return nil, 0, err
	}

	return tags, expectedTotal, nil
}

// ListTagsAll repeatedly calls ListTags until the whole set has been
// retrieved. The pagination-related params will overriden, but all others
// will be passed on verbatim with each request.
func (wp *Client) ListTagsAll(params url.Values) ([]*Tag, error) {
	perPage := 100

	tags := []*Tag{}

	for {
		// We override pagination-related params
		params.Set("per_page", strconv.Itoa(perPage))
		params.Set("offset", strconv.Itoa(len(tags)))
		params.Del("page")

		batch, expectedTotal, err := wp.ListTags(params)

		if err != nil {
			return nil, err
		}

		tags = append(tags, batch...)
		if wp.Verbose {
			fmt.Fprintf(os.Stderr, " %d/%d\n", len(tags), expectedTotal)
		}
		if len(batch) == 0 || len(tags) >= expectedTotal {
			break
		}
	}
	return tags, nil
}

// GET /wp/v2/categories
// params we're interested in:
func (wp *Client) ListCategories(params url.Values) ([]*Category, int, error) {
	u := wp.BaseURL + "/wp/v2/categories?" + params.Encode()

	if wp.Verbose {
		fmt.Fprintf(os.Stderr, "fetch %s\n", u)
	}

	resp, err := wp.HTTPClient.Get(u)
	if err != nil {
		return nil, 0, err
	}

	// totalpages is returned as a header
	expectedTotal, err := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	if err != nil {
		return nil, 0, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != 200 {
		return nil, 0, fmt.Errorf("%s: %d\n", u, resp.StatusCode)
	}

	categories := []*Category{}

	err = json.Unmarshal(raw, &categories)
	if err != nil {
		return nil, 0, err
	}

	return categories, expectedTotal, nil
}

// ListCategoriesAll repeatedly calls ListCategories until the whole set has been
// retrieved. The pagination-related params will overriden, but all others
// will be passed on verbatim with each request.
func (wp *Client) ListCategoriesAll(params url.Values) ([]*Category, error) {
	perPage := 100

	categories := []*Category{}

	for {
		// We override pagination-related params
		params.Set("per_page", strconv.Itoa(perPage))
		params.Set("offset", strconv.Itoa(len(categories)))
		params.Del("page")

		batch, expectedTotal, err := wp.ListCategories(params)

		if err != nil {
			return nil, err
		}

		categories = append(categories, batch...)
		if wp.Verbose {
			fmt.Fprintf(os.Stderr, " %d/%d\n", len(categories), expectedTotal)
		}
		if len(batch) == 0 || len(categories) >= expectedTotal {
			break
		}
	}
	return categories, nil
}
