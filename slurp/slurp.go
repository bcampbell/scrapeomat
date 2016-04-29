package slurp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Slurper is a client for talking to a slurp server
type Slurper struct {
	Client *http.Client
	// eg "http://localhost:12345/ukarticles
	Location string
}

func NewSlurper(location string) *Slurper {
	return &Slurper{Location: location}
}

// Msg is a single message - can hold an article or error message
type Msg struct {
	Article *Article `json:"article,omitempty"`
	Error   string   `json:"error,omitempty"`
	Next    struct {
		SinceID int `json:"since_id,omitempty"`
	} `json:"next,omitempty"`
}

type Filter struct {
	// date ranges are [from,to)
	PubFrom time.Time
	PubTo   time.Time
	//	AddedFrom time.Time
	//	AddedTo   time.Time
	PubCodes []string
	SinceID  int
	Count    int
}

// Slurp downloads a set of articles from the server
// returns a channel which streams out messages.
// errors are returned via Msg. In the case of network errors,
// Slurp may synthesise fake Msgs containing the error message.
// Will repeatedly request until all results returned.
// filter count param is not the total - it is the max articles to
// return per request.
func (s *Slurper) Slurp(filt *Filter) (chan Msg, chan struct{}) {

	params := url.Values{}

	if !filt.PubFrom.IsZero() {
		params.Set("pubfrom", filt.PubFrom.Format(time.RFC3339))
	}
	if !filt.PubTo.IsZero() {
		params.Set("pubto", filt.PubTo.Format(time.RFC3339))
	}
	for _, pubCode := range filt.PubCodes {
		params.Add("pub", pubCode)
	}

	if filt.SinceID > 0 {
		params.Set("since_id", strconv.Itoa(filt.SinceID))
	}
	if filt.Count > 0 {
		params.Set("count", strconv.Itoa(filt.Count))
	}

	out := make(chan Msg)
	cancel := make(chan struct{}, 1) // buffered to prevent deadlock
	go func() {
		defer close(out)
		defer close(cancel)

		client := s.Client
		if client == nil {
			client = &http.Client{}
		}

		for {
			u := s.Location + "/api/slurp?" + params.Encode()
			// fmt.Printf("request: %s\n", u)
			resp, err := client.Get(u)
			if err != nil {
				out <- Msg{Error: fmt.Sprintf("HTTP Get failed: %s", err)}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				out <- Msg{Error: fmt.Sprintf("HTTP Error: %s", resp.Status)}
				return
			}
			nextSinceID := 0
			dec := json.NewDecoder(resp.Body)
			for {
				// check for cancelation request
				select {
				case <-cancel:
					out <- Msg{Error: "Cancelled"}
					return
				default:
				}

				var msg Msg
				if err := dec.Decode(&msg); err == io.EOF {
					break
				} else if err != nil {
					out <- Msg{Error: fmt.Sprintf("Decode error: %s", err)}
					return
				}

				// is it a to-be-continued message?
				if msg.Next.SinceID > 0 {
					nextSinceID = msg.Next.SinceID
				} else {
					out <- msg
				}
			}

			if nextSinceID == 0 {
				break
			}
			// update the query params with the new since_id
			params.Set("since_id", strconv.Itoa(nextSinceID))
		}
	}()

	return out, cancel
}

func (s *Slurper) Slurp2(filt *Filter) *ArtStream {
	client := s.Client
	if client == nil {
		client = &http.Client{}
	}

	params := url.Values{}

	if !filt.PubFrom.IsZero() {
		params.Set("pubfrom", filt.PubFrom.Format(time.RFC3339))
	}
	if !filt.PubTo.IsZero() {
		params.Set("pubto", filt.PubTo.Format(time.RFC3339))
	}
	for _, pubCode := range filt.PubCodes {
		params.Add("pub", pubCode)
	}

	if filt.SinceID > 0 {
		params.Set("since_id", strconv.Itoa(filt.SinceID))
	}
	if filt.Count > 0 {
		params.Set("count", strconv.Itoa(filt.Count))
	}
	u := s.Location + "/api/slurp?" + params.Encode()

	out := &ArtStream{}
	// fmt.Printf("request: %s\n", u)
	resp, err := client.Get(u)
	if err != nil {
		out.err = fmt.Errorf("HTTP Get failed: %s", err)
		return out
	}
	out.response = resp

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out.err = fmt.Errorf("HTTP Error code %s", resp.Status)
		return out
	}

	out.dec = json.NewDecoder(out.response.Body)
	return out
}

type ArtStream struct {
	response *http.Response
	dec      *json.Decoder
	err      error

	// if there are more articles to grab, this will be set to non-zero when the stream ends
	NextSinceID int
}

func (as *ArtStream) Close() {
	if as.response != nil {
		as.response.Body.Close()
		as.response = nil
	}
}

// returns io.EOF at end of stream
func (as *ArtStream) Next() (*Article, error) {
	if as.err != nil {
		return nil, as.err
	}
	for {
		// grab the next message off the wire
		var msg Msg
		err := as.dec.Decode(&msg)
		if err == io.EOF {
			as.err = err
			return nil, err
		} else if err != nil {
			as.err = fmt.Errorf("Decode error: %s", err)
			return nil, as.err
		}

		// is it a to-be-continued message?
		if msg.Next.SinceID > 0 {
			as.NextSinceID = msg.Next.SinceID
			// probably that'll be the end of the stream, but loop until we hit the EOF anyway
		} else {
			return msg.Article, nil
		}
	}
}
