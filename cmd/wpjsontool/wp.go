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

type PostsFilter struct {
	PerPage int
	Offset  int
	After   string
	Before  string
}

func (filt *PostsFilter) Params() url.Values {

	params := url.Values{}

	if filt.PerPage > 0 {
		params.Set("per_page", strconv.Itoa(filt.PerPage))
	}
	if filt.Offset > 0 {
		params.Set("offset", strconv.Itoa(filt.Offset))
	}
	if filt.After != "" {
		params.Set("after", filt.After)
	}
	if filt.Before != "" {
		params.Set("before", filt.Before)
	}
	return params
}

// fetch a list of posts ("/wp/v2/posts)
// returns expected number of posts, posts, error
func FetchPosts(client *http.Client, apiURL string, filt *PostsFilter) (int, []Post, error) {
	params := filt.Params()

	u := apiURL + "/wp/v2/posts?" + params.Encode()

	// TODO: proper optional logging
	if verbose {
		fmt.Fprintf(os.Stderr, "fetch %s\n", u)
	}

	resp, err := client.Get(u)
	if err != nil {
		return 0, nil, err
	}

	// totalpages is returned as a header
	expectedTotal, err := strconv.Atoi(resp.Header.Get("X-WP-Total"))
	if err != nil {
		return 0, nil, err
	}
	raw, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return 0, nil, err
	}
	if resp.StatusCode != 200 {
		return 0, nil, fmt.Errorf("%s: %d\n", u, resp.StatusCode)
	}

	posts := []Post{}

	err = json.Unmarshal(raw, &posts)
	if err != nil {
		return 0, nil, err
	}

	return expectedTotal, posts, nil
}

// GET /wp/v2/tags/<id>
func FetchTag(client *http.Client, apiURL string, tagID int) (*Tag, error) {
	u := apiURL + fmt.Sprintf("/wp/v2/tags/%d", tagID)

	// TODO: proper optional logging
	if verbose {
		fmt.Fprintf(os.Stderr, "fetch %s\n", u)
	}

	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: %d\n", u, resp.StatusCode)
	}

	tag := &Tag{}

	err = json.Unmarshal(raw, tag)
	if err != nil {
		return nil, err
	}

	return tag, nil
}

// GET /wp/v2/categories/<id>
func FetchCategory(client *http.Client, apiURL string, catID int) (*Category, error) {
	u := apiURL + fmt.Sprintf("/wp/v2/categories/%d", catID)

	// TODO: proper optional logging
	if verbose {
		fmt.Fprintf(os.Stderr, "fetch %s\n", u)
	}

	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}

	raw, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: %d\n", u, resp.StatusCode)
	}

	cat := &Category{}

	err = json.Unmarshal(raw, cat)
	if err != nil {
		return nil, err
	}

	return cat, nil
}
