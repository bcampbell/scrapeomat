package main

// export articles from mongodb into our own db format

import (
	//	"encoding/gob"
	"flag"
	"fmt"
	"github.com/bcampbell/arts/arts"
	"github.com/bcampbell/badger"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"os"
)

var sesh *mgo.Session
var collName = "articles"

// article in mongo
type TmpArticle struct {
	arts.Article `bson:",inline"`
	ID           bson.ObjectId `bson:"_id,omitempty"`
	Pub          string        `bson:"pub"`
	Tags         []string      `bson:"tags"`
}

// processed article (TODO: share with steno)
type Article struct {
	arts.Article
	ID   string
	Pub  string
	KW   []string // flattened list of keywords (no urls)
	Tags []string
}

type foo struct{ sn, tag string }

var augment = map[string]struct{ sn, tag string }{
	"blogs.spectator.co.uk":          {"spectator", "nat"},
	"blogs.telegraph.co.uk":          {"telegraph", "nat"},
	"labourlist.org":                 {"labourlist", "blog"},
	"liberalconspiracy.org":          {"liberalconspiracy", "blog"},
	"order-order.com":                {"order-order", "blog"},
	"politicalscrapbook.net":         {"politicalscrapbook", "blog"},
	"www.politics.co.uk":             {"politics.co.uk", "blog"},
	"politicshome.com":               {"politicshome", "blog"},
	"ukpollingreport.co.uk":          {"ukpollingreport", "blog"},
	"www.bbc.co.uk":                  {"bbc", "nat"},
	"www.express.co.uk":              {"express", "nat"},
	"www.conservativehome.com":       {"conservativehome", "blog"},
	"www.dailymail.co.uk":            {"dailymail", "nat"},
	"www.thisismoney.co.uk":          {"dailymail", "nat"},
	"www.ft.com":                     {"ft", "nat"},
	"blogs.ft.com":                   {"ft", "nat"},
	"www.leftfootforward.org":        {"leftfootforward", "blog"},
	"www.iaindale.com":               {"iaindale", "blog"},
	"www.independent.co.uk":          {"independent", "nat"},
	"live.independent.co.uk":         {"independent", "nat"},
	"blogs.independent.co.uk":        {"independent", "nat"},
	"www.mirror.co.uk":               {"mirror", "nat"},
	"www.irishmirror.ie":             {"mirror", "nat"},
	"www.newstatesman.com":           {"newstatesman", "nat"},
	"www.spectator.co.uk":            {"spectator", "nat"},
	"www.telegraph.co.uk":            {"telegraph", "nat"},
	"fashion.telegraph.co.uk":        {"telegraph", "nat"},
	"www.thecommentator.com":         {"thecommentator", "blog"},
	"www.theguardian.com":            {"guardian", "nat"},
	"www.thesun.co.uk":               {"sun", "nat"},
	"www.thescottishsun.co.uk":       {"scottishsun", "scot"},
	"www.thesundaytimes.co.uk":       {"sundaytimes", "nat"},
	"www.thetimes.co.uk":             {"times", "nat"},
	"www.scotsman.com":               {"scotsman", "scot"},
	"www.edinburghnews.scotsman.com": {"scotsman", "scot"},
	"www.dailyrecord.co.uk":          {"dailyrecord", "scot"},
	"www.heraldscotland.com":         {"herald", "scot"},
	"www.eveningtimes.co.uk":         {"eveningtimes", "scot"},
	"www.eveningtelegraph.co.uk":     {"thetele", "scot"},
	"www.thecourier.co.uk":           {"thecourier", "scot"},
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <outfile>\n", os.Args[0])
		flag.PrintDefaults()
	}
	var databaseURL = flag.String("database", "", "mongodb database url (eg localhost/scrapeomat)")
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Missing output filename\n")
		os.Exit(1)
	}
	outFileName := flag.Arg(0)

	var err error
	sesh, err = mgo.Dial(*databaseURL)
	defer sesh.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to mongodb: %s\n", err)
		os.Exit(1)
	}

	//	fmt.Println("Fetching...")
	var coll *badger.Collection
	coll, err = fetchIt()
	if err != nil {
		fmt.Fprintf(os.Stderr, "doIt failed: %s\n", err)
		os.Exit(1)
	}
	//	fmt.Println("Saving...")
	err = saveIt(coll, outFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doIt failed: %s\n", err)
		os.Exit(1)
	}
	//	fmt.Println("Done.")
}

func fetchIt() (*badger.Collection, error) {
	s := sesh.Clone()
	defer s.Close()
	c := s.DB("").C(collName)
	it := c.Find(nil).Iter()
	store := badger.NewCollection(&Article{})
	for {
		var tmp TmpArticle

		if it.Next(&tmp) == false {
			break
		}

		art := Article{Article: tmp.Article, ID: tmp.ID.Hex(), Pub: tmp.Pub, KW: []string{}, Tags: tmp.Tags}

		for _, k := range tmp.Keywords {
			art.KW = append(art.KW, k.Name)
		}

		// apply tag/pub rules
		aug, got := augment[art.Publication.Domain]
		if got {
			art.Pub = aug.sn
			art.Tags = append(art.Tags, aug.tag)
		}

		store.Put(&art)
	}

	err := it.Err()
	if err != nil {
		return nil, err
	}
	return store, nil
}

func saveIt(coll *badger.Collection, outFileName string) error {

	outFile, err := os.Create(outFileName)
	if err != nil {
		return err
	}

	defer outFile.Close()
	err = coll.Write(outFile)
	return err
}
