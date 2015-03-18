package slurp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	// TODO: include info/progress report messages
}

// Slurp downloads a set of articles from the server
// returns a channel which streams out messages.
// errors are returned via Msg. In the case of network errors,
// Slurp may synthesise fake Msgs containing the error message.
func (s *Slurper) Slurp(dayFrom, dayTo string) chan Msg {
	out := make(chan Msg)

	go func() {
		defer close(out)
		u := fmt.Sprintf("%s/api/slurp?from=%s&to=%s", s.Location, dayFrom, dayTo)

		// TODO: request (and handle) gzip compression!

		client := s.Client
		if client == nil {
			client = &http.Client{}
		}

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

		dec := json.NewDecoder(resp.Body)
		for {
			var msg Msg
			if err := dec.Decode(&msg); err == io.EOF {
				break
			} else if err != nil {
				out <- Msg{Error: fmt.Sprintf("Decode error: %s", err)}
				return
			}

			out <- msg
		}
	}()

	return out
}
