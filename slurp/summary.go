package slurp

import (
	"encoding/json"
	"fmt"
	"net/http"
	//	"net/url"
	"sort"
	"time"
)

// map of maps pubcodes -> days -> counts
type RawSummary map[string]map[string]int

// returns a map of maps pubcodes -> days -> counts
func (s *Slurper) Summary(filt *Filter) (RawSummary, error) {

	if filt.Count != 0 {
		return nil, fmt.Errorf("Count must be zero")
	}

	params := filt.params()

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
	Max  int
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

func dayExtents(raw RawSummary) (time.Time, time.Time) {
	maxDay := time.Time{}
	minDay := time.Date(999999, 0, 0, 0, 0, 0, 0, time.UTC)
	for _, days := range raw {
		for day, _ := range days {
			if day == "" {
				continue
			}
			t, err := time.Parse("2006-01-02", day)
			if err != nil {
				continue
			}

			if t.Before(minDay) {
				minDay = t
			}
			if t.After(maxDay) {
				maxDay = t
			}
		}
	}
	return minDay, maxDay
}

// cooks raw article counts, filling in missing days
func CookSummary(raw RawSummary) *CookedSummary {

	// get date extents
	minDay, maxDay := dayExtents(raw)

	// create continuous day range, no gaps
	days := []string{} //dayRange(dayFrom, dayTo)
	for day := minDay; !day.After(maxDay); day = day.AddDate(0, 0, 1) {
		days = append(days, day.Format("2006-01-02"))
	}

	//
	pubCodes := make([]string, 0, len(raw))
	for pubCode, _ := range raw {
		pubCodes = append(pubCodes, pubCode)
	}
	sort.Strings(pubCodes)

	cooked := CookedSummary{
		Days:     days,
		PubCodes: pubCodes,
		Data:     make([][]int, len(pubCodes)),
		Max:      0,
	}

	maxCnt := 0
	for i, pubCode := range pubCodes {
		counts := make([]int, len(days))
		for j, day := range days {
			cnt := raw[pubCode][day]
			if cnt > maxCnt {
				maxCnt = cnt
			}
			counts[j] = cnt
		}
		cooked.Data[i] = counts
	}
	cooked.Max = maxCnt

	return &cooked
}
