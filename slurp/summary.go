package slurp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"
)

// map of maps pubcodes -> days -> counts
type RawSummary map[string]map[string]int

// returns a map of maps pubcodes -> days -> counts
func (s *Slurper) Summary(from time.Time, to time.Time) (RawSummary, error) {

	params := url.Values{}
	params.Set("from", from.Format(time.RFC3339))
	params.Set("to", to.Format(time.RFC3339))

	client := s.Client
	if client == nil {
		client = &http.Client{}
	}

	u := s.Location + "/api/summary?" + params.Encode()
	// fmt.Printf("request: %s\n", u)
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err = fmt.Errorf("HTTP error: %s", resp.Status)
		return nil, err
	}

	dec := json.NewDecoder(resp.Body)

	var raw struct {
		Counts RawSummary `json:"counts"`
	}

	err = dec.Decode(&raw)
	if err != nil {
		return nil, err
	}

	return raw.Counts, nil
}

type CookedSummary struct {
	PubCodes []string
	Days     []string
	// An array of array of counts
	// access as: Data[pubcodeindex][dayindex]
	Data [][]int
}

// return a range of days (sorted, inclusive)
func dayRange(dayFrom string, dayTo string) []string {
	days := []string{}

	tFrom, err := time.Parse("2006-01-02", dayFrom)
	if err != nil {
		return days
	}

	tTo, err := time.Parse("2006-01-02", dayTo)
	if err != nil {
		return days
	}

	for day := tFrom; !day.After(tTo); day = day.AddDate(0, 0, 1) {
		days = append(days, day.Format("2006-01-02"))
	}
	return days
}

// cooks raw article counts, filling in missing days
func CookSummary(raw RawSummary, dayFrom string, dayTo string) *CookedSummary {
	days := dayRange(dayFrom, dayTo)

	pubCodes := make([]string, 0, len(raw))
	for pubCode, _ := range raw {
		pubCodes = append(pubCodes, pubCode)
	}
	sort.Strings(pubCodes)

	cooked := CookedSummary{
		Days:     days,
		PubCodes: pubCodes,
		Data:     make([][]int, len(pubCodes)),
	}

	for i, pubCode := range pubCodes {
		counts := make([]int, len(days))
		for j, day := range days {
			counts[j] = raw[pubCode][day]
		}
		cooked.Data[i] = counts
	}

	return &cooked
}
