package main

import (
	"fmt"
	"net/http"
	"net/url"
	"semprini/scrapeomat/store"
	"strconv"
	"time"
)

// helper to make it easier to use Filter in templates
type Filter store.Filter

func (filt *Filter) IsPubSet(pubCode string) bool {
	for _, code := range filt.PubCodes {
		if code == pubCode {
			return true
		}
	}
	return false
}

const yyyymmddLayout = "2006-01-02"

func (filt *Filter) PubFromString() string {
	if filt.PubFrom.IsZero() {
		return ""
	}
	return filt.PubFrom.Format(yyyymmddLayout)
}

func (filt *Filter) PubToString() string {
	if filt.PubTo.IsZero() {
		return ""
	}
	return filt.PubTo.Format(yyyymmddLayout)
}

func (filt *Filter) Params() url.Values {
	v := url.Values{}
	if !filt.PubFrom.IsZero() {
		v.Set("pubfrom", filt.PubFrom.Format(yyyymmddLayout))
	}
	if !filt.PubTo.IsZero() {
		v.Set("pubto", filt.PubTo.Format(yyyymmddLayout))
	}
	if !filt.AddedFrom.IsZero() {
		v.Set("addedfrom", filt.PubFrom.Format(yyyymmddLayout))
	}
	if !filt.AddedTo.IsZero() {
		v.Set("addedto", filt.PubTo.Format(yyyymmddLayout))
	}

	if filt.SinceID != 0 {
		v.Set("since_id", strconv.Itoa(filt.SinceID))
	}
	if filt.Count != 0 {
		v.Set("count", strconv.Itoa(filt.Count))
	}

	for _, pubCode := range filt.PubCodes {
		v.Add("pub", pubCode)
	}
	for _, pubCode := range filt.XPubCodes {
		v.Add("xpub", pubCode)
	}

	// v.Encode() == "name=Ava&friend=Jess&friend=Sarah&friend=Zoe"

	return v
}

func parseTime(in string) (time.Time, error) {

	t, err := time.ParseInLocation(time.RFC3339, in, time.UTC)
	if err == nil {
		return t, nil
	}

	// short form - assumes you want utc days rather than local days...
	t, err = time.ParseInLocation(yyyymmddLayout, in, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date/time format")
	}

	return t, nil

}

func getFilter(r *http.Request) (*store.Filter, error) {
	maxCount := 20000

	filt := &store.Filter{}

	// deprecated!
	if r.FormValue("from") != "" {
		t, err := parseTime(r.FormValue("from"))
		if err != nil {
			return nil, fmt.Errorf("bad 'from' param")
		}

		filt.PubFrom = t
	}

	// deprecated!
	if r.FormValue("to") != "" {
		t, err := parseTime(r.FormValue("to"))
		if err != nil {
			return nil, fmt.Errorf("bad 'to' param")
		}
		t = t.AddDate(0, 0, 1) // add one day
		filt.PubTo = t
	}

	// TODO: addedfrom/addedto???

	if r.FormValue("pubfrom") != "" {
		t, err := parseTime(r.FormValue("pubfrom"))
		if err != nil {
			return nil, fmt.Errorf("bad 'pubfrom' param")
		}

		filt.PubFrom = t
	}
	if r.FormValue("pubto") != "" {
		t, err := parseTime(r.FormValue("pubto"))
		if err != nil {
			return nil, fmt.Errorf("bad 'pubto' param")
		}

		filt.PubTo = t
	}

	if r.FormValue("since_id") != "" {
		sinceID, err := strconv.Atoi(r.FormValue("since_id"))
		if err != nil {
			return nil, fmt.Errorf("bad 'since_id' param")
		}
		if sinceID > 0 {
			filt.SinceID = sinceID
		}
	}

	if r.FormValue("count") != "" {
		cnt, err := strconv.Atoi(r.FormValue("count"))
		if err != nil {
			return nil, fmt.Errorf("bad 'count' param")
		}
		filt.Count = cnt
	} else {
		// default to max
		filt.Count = maxCount
	}

	// enforce max count
	if filt.Count > maxCount {
		return nil, fmt.Errorf("'count' too high (max %d)", maxCount)
	}

	// publication codes?
	if pubs, got := r.Form["pub"]; got {
		filt.PubCodes = pubs
	}

	// publication codes to exclude?
	if xpubs, got := r.Form["xpub"]; got {
		filt.XPubCodes = xpubs
	}

	return filt, nil
}
