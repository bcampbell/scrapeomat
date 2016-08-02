package store

import (
	"fmt"
	"strings"
	"time"
)

// Filter is a set of params for querying the store
type Filter struct {
	// date ranges are [from,to)
	PubFrom   time.Time
	PubTo     time.Time
	AddedFrom time.Time
	AddedTo   time.Time
	// if empty, accept all publications (else only ones in list)
	PubCodes []string
	// exclude any publications in XPubCodes
	XPubCodes []string
	// Only return articles with ID > SinceID
	SinceID int
	// max number of articles wanted
	Count int
}

// Describe returns a concise description of the filter for logging/debugging/whatever
func (filt *Filter) Describe() string {
	s := "[ "

	if !filt.PubFrom.IsZero() && !filt.PubTo.IsZero() {
		s += fmt.Sprintf("pub %s..%s ", filt.PubFrom.Format(time.RFC3339), filt.PubTo.Format(time.RFC3339))
	} else if !filt.PubFrom.IsZero() {
		s += fmt.Sprintf("pub %s.. ", filt.PubFrom.Format(time.RFC3339))
	} else if !filt.PubTo.IsZero() {
		s += fmt.Sprintf("pub ..%s ", filt.PubTo.Format(time.RFC3339))
	}

	if !filt.AddedFrom.IsZero() && !filt.AddedTo.IsZero() {
		s += fmt.Sprintf("added %s..%s ", filt.AddedFrom.Format(time.RFC3339), filt.AddedTo.Format(time.RFC3339))
	} else if !filt.AddedFrom.IsZero() {
		s += fmt.Sprintf("added %s.. ", filt.AddedFrom.Format(time.RFC3339))
	} else if !filt.AddedTo.IsZero() {
		s += fmt.Sprintf("added ..%s ", filt.AddedTo.Format(time.RFC3339))
	}

	if len(filt.PubCodes) > 0 {
		s += strings.Join(filt.PubCodes, "|") + " "
	}

	if len(filt.XPubCodes) > 0 {
		foo := make([]string, len(filt.XPubCodes))
		for i, x := range filt.XPubCodes {
			foo[i] = "!" + x
		}
		s += strings.Join(foo, "|") + " "
	}

	if filt.Count > 0 {
		s += fmt.Sprintf("cnt %d ", filt.Count)
	}
	if filt.SinceID > 0 {
		s += fmt.Sprintf("since %d ", filt.SinceID)
	}

	s += "]"
	return s
}
