package main

import (
	"flag"
	"fmt"
	"io"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"os"
	"time"
)

var sesh *mgo.Session
var collName string = "articles"

type Cooked struct {
	Pub    string
	Counts []int
}

func main() {

	fromDefault := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	toDefault := time.Now().Format("2006-01-02")
	var dayFirst, dayLast string
	flag.StringVar(&dayFirst, "from", fromDefault, "start day YYYY-MM-DD")
	flag.StringVar(&dayLast, "to", toDefault, "end day YYYY-MM-DD")
	var databaseURL string
	flag.StringVar(&databaseURL, "database", "localhost/scrapeomat", "mongodb database url")
	flag.Parse()

	var err error
	sesh, err = mgo.Dial(databaseURL)
	if err != nil {
		panic(err)
	}
	defer sesh.Close()

	c := sesh.DB("").C(collName)
	days := calcDays(dayFirst, dayLast)
	// to go from date to index
	dayLookup := map[string]int{}
	for idx, day := range days {
		dayLookup[day] = idx
	}

	pipe := c.Pipe([]bson.M{
		{
			"$match": bson.M{
				"published": bson.M{
					"$gte": dayFirst, "$lte": dayLast,
				},
			},
		},
		{
			"$project": bson.M{
				"pub": "$publication.domain",
				"day": bson.M{
					"$substr": []interface{}{"$published", 0, 10},
				},
			},
		},
		{
			"$group": bson.M{
				"_id": bson.M{
					"pub": "$pub",
					"day": "$day",
				},
				"count": bson.M{"$sum": 1},
			},
		},
		{
			"$group": bson.M{
				"_id": "$_id.pub",
				"counts": bson.M{"$push": bson.M{
					"day": "$_id.day",
					"cnt": "$count",
				}},
			},
		},
		{
			"$sort": bson.M{"_id": 1},
		},
	})
	iter := pipe.Iter()

	var raw struct {
		Pub    string `bson:"_id,omitempty"`
		Counts []struct {
			Day string
			Cnt int
		}
	}

	results := []Cooked{}

	//	publicationSet := map[string]struct{}{}
	for iter.Next(&raw) {

		ck := Cooked{Pub: raw.Pub, Counts: make([]int, len(days))}
		for _, cnt := range raw.Counts {
			ck.Counts[dayLookup[cnt.Day]] = cnt.Cnt
		}
		results = append(results, ck)
	}
	if err := iter.Close(); err != nil {
		panic(err)
	}

	prettyOutput(os.Stdout, results, days)

	/*
		// get unique publications
		pubs := []string{}
		for pub, _ := range publicationSet {
			pubs = append(pubs, pub)
		}

			// build array of counts. Columns for publication, one day per row
			results := [][]int{}
			for _, d := range days {
				row := make([]int, len(pubs))
				for i, pub := range pubs {
					row[i] = collated[d][pub]
				}
				results = append(results, row)
			}

			//
			fmt.Printf("day")
			for _, pub := range pubs {
				fmt.Printf("\t%s", pub)
			}

			for i, row := range results {
				fmt.Printf("%s", days[i])
				for _, cnt := range row {
					fmt.Printf("\t%d", cnt)
				}
				fmt.Printf("\n")
			}
	*/
}

// generate a range of days, in "YYYY-MM-DD" form
func calcDays(first, last string) []string {
	out := []string{}

	start, err := time.Parse("2006-01-02", first)
	if err != nil {
		panic(err)
	}
	end, err := time.Parse("2006-01-02", last)
	if err != nil {
		panic(err)
	}
	end = end.AddDate(0, 0, 1)

	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}

	return out
}

func prettyOutput(w io.Writer, results []Cooked, days []string) {

	fmt.Fprintf(w, "%24s", "")

	for _, day := range days {
		t, err := time.Parse("2006-01-02", day)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(w, "%6s", t.Format("02Jan"))
	}
	fmt.Fprintf(w, "\n")

	for _, row := range results {
		fmt.Fprintf(w, "%24s", row.Pub)
		for _, cnt := range row.Counts {
			fmt.Fprintf(w, "%6d", cnt)
		}
		fmt.Fprintf(w, "\n")
	}

}
