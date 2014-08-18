package store

import (
	"encoding/json"
	"fmt"
	"github.com/bcampbell/arts/arts"
	"github.com/donovanhide/eventsource"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

type Store interface {
	WhichAreNew([]string) ([]string, error)

	// Stash saves an article to the store.
	// If the article is already in the store, the old one will be replaced
	// with the new one and the new ID returned. The URLs in the new article
	// will be the union of URLs from both versions - ie _all_ the known urls
	// for the article.
	Stash(*arts.Article) (string, error)
	Close()

	// Replay to implement eventsource.Repository
	Replay(string, string) chan eventsource.Event
}
type storedArticle struct {
	arts.Article `bson:",inline"`
	ID           bson.ObjectId `bson:"_id,omitempty"`
}

// ArticleEvent wraps up an Article for use as a server-sent event.
type ArticleEvent struct {
	payload *arts.Article
	eventID string
}

func NewArticleEvent(payLoad *arts.Article, id string) *ArticleEvent {
	return &ArticleEvent{payload: payLoad, eventID: id}
}

func (ev *ArticleEvent) Id() string {
	return ev.eventID
}

func (ev *ArticleEvent) Event() string {
	return "article"
}

func (ev *ArticleEvent) Data() string {
	out, err := json.Marshal(*ev.payload)
	if err != nil {
		panic(err)
	}
	return string(out)
}

// TestStore is a null store which does nothing
type TestStore struct{}

func NewTestStore() *TestStore {
	store := &TestStore{}
	return store
}
func (store *TestStore) Close() {
}

func (store *TestStore) WhichAreNew(artURLs []string) ([]string, error) {
	return artURLs, nil
}

func (store *TestStore) Stash(art *arts.Article) (string, error) {
	u := art.CanonicalURL
	if u == "" {
		u = art.URLs[0]
	}
	fmt.Printf("%s \"%s\" [%s]\n", art.Published, art.Headline, art.CanonicalURL)
	return "", nil
}

func (store *TestStore) Replay(channel, lastEventId string) chan eventsource.Event {
	panic("unsupported")
	return nil
}

// MongoStore stashes articles in a mongodb
type MongoStore struct {
	session *mgo.Session
	artColl string
}

func NewMongoStore(mongoURL string) *MongoStore {

	store := MongoStore{artColl: "articles"}
	var err error
	store.session, err = mgo.Dial(mongoURL)
	if err != nil {
		panic(err)
	}
	urlsIndex := mgo.Index{
		Key:        []string{"urls"},
		Unique:     true,
		DropDups:   false,
		Background: false,
		Sparse:     false,
	}
	err = store.session.DB("").C(store.artColl).EnsureIndex(urlsIndex)
	if err != nil {
		panic(err)
	}

	return &store
}

func (store *MongoStore) Close() {
	store.session.Close()
}

func (store *MongoStore) Stash(art *arts.Article) (string, error) {

	// NOTE: we use id for event ids, so we want them to be ascending.
	// This should be fine, but keep in mind that in theory it could glitch
	// if we were using multiple nodes with skewed clocks.
	id := bson.NewObjectId()
	doc := storedArticle{Article: *art, ID: id}
	//fmt.Printf("[%v] %d urls: %v", id, len(doc.URLs), doc.URLs)
	db := store.session.DB("")
	coll := db.C(store.artColl)
	err := coll.Insert(doc)

	if err == nil {
		// yay. all nice and simple
		return id.Hex(), nil
	}

	if !mgo.IsDup(err) {
		return "", err
	}

	//fmt.Printf("already got: %v\n", art.URLs)

	// looks we've got the article already - we might have multiple urls
	// this can happen when we discover new urls (eg canonical url) after scraping

	// find existing one
	var existing storedArticle
	err = coll.Find(bson.M{"urls": bson.M{"$in": art.URLs}}).One(&existing)
	if err != nil {
		return "", fmt.Errorf("huh? dupe but no urls (%s) (%v)", err, art.URLs)
	}

	// merge all the known urls into the new doc
	uniqURLs := map[string]struct{}{}
	for _, u := range existing.URLs {
		uniqURLs[u] = struct{}{}
	}
	for _, u := range doc.URLs {
		uniqURLs[u] = struct{}{}
	}
	doc.URLs = make([]string, 0, len(uniqURLs))
	for u, _ := range uniqURLs {
		doc.URLs = append(doc.URLs, u)
	}

	// delete old doc
	err = coll.RemoveId(existing.ID)
	if err != nil {
		return "", fmt.Errorf("huh? RemoveID failed (id=%s) (%s)", existing.ID, err)
	}

	// replace with new one
	err = coll.Insert(doc)
	if err != nil {
		return "", fmt.Errorf("huh? Insert failed (%s)", err)
	}

	return id.Hex(), err
}

func (store *MongoStore) WhichAreNew(artURLs []string) ([]string, error) {

	c := store.session.DB("").C(store.artColl)
	goodUns := make([]string, 0)
	// TODO: Should replace this with a single "$in" query
	for _, u := range artURLs {
		cnt, err := c.Find(bson.M{"urls": u}).Count()
		if err != nil {
			return nil, err
		}
		if cnt == 0 {
			goodUns = append(goodUns, u)
		}
	}

	return goodUns, nil
}

func (store *MongoStore) Replay(channel, lastEventId string) chan eventsource.Event {
	out := make(chan eventsource.Event)
	go func() {
		// copy session to avoid blocking
		sesh := store.session.Copy()
		defer sesh.Close()
		c := sesh.DB("").C(store.artColl)

		it := c.Find(bson.M{"_id": bson.M{"$gte": bson.ObjectIdHex(lastEventId)}}).Iter()
		var result storedArticle
		for it.Next(&result) {
			//fmt.Printf("Result: %v\n", result.ID.Hex())
			out <- &ArticleEvent{&result.Article, result.ID.Hex()}
		}
		if err := it.Close(); err != nil {
			panic(err)
		}
	}()
	return out
}
