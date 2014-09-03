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

type TmpArticle struct {
	arts.Article `bson:",inline"`
	ID           bson.ObjectId `bson:"_id,omitempty"`
	Pub          string        `bson:"pub"`
	Tags         []string      `bson:"tags"`
}

type Article struct {
	arts.Article
	ID   string
	Pub  string
	Tags []string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <outfile>\n", os.Args[0])
		flag.PrintDefaults()
	}
	var databaseURL = flag.String("database", "localhost/eurobot", "mongodb database url")
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

	fmt.Println("Fetching...")
	var coll *badger.Collection
	coll, err = fetchIt()
	if err != nil {
		fmt.Fprintf(os.Stderr, "doIt failed: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("Saving...")
	err = saveIt(coll, outFileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doIt failed: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("Done.")
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

		art := Article{Article: tmp.Article, ID: tmp.ID.Hex(), Pub: tmp.Pub, Tags: tmp.Tags}
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
