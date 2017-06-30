package main

import (
	"fmt"
	"github.com/andybalholm/cascadia"
	"github.com/bcampbell/arts/util"
	"golang.org/x/net/html"
	"net/http"
    "net/url"
    "encoding/json"
    "strconv"
    "strings"
    "io/ioutil"
    "bytes"
)

// use search ajax:
// http://www.milenio.com/milappservices/search.json?term=una&orderby=desc&contentType=&page=2&limit=100&seccion=&iniDate=2017-01-01&endDate=2017-01-05

// ({"data":{"results":"","count":""},"error":0,"message":""})
// where results contains html snippet



func DoMilenio(opts *Options) error {

    var raw struct {
        Data struct {
            Results string `json:"results"`
            /*Count int   `json:"count"`*/
        } `json:"data"`
/*        Error int   `json:"error"`
        Message string `json:"message"`*/
    }

    if opts.dayFrom == "" || opts.dayTo=="" {
        return fmt.Errorf("date range required")
    }


	linkSel := cascadia.MustCompile(`.md-listing-item h3 a.lnk`)

	client := &http.Client{Transport: util.NewPoliteTripper()}

    for page:=1; ; page++ {
        v := url.Values{}
	    v.Set("term", "una")
        v.Set("orderby", "desc")
        v.Set("contentType", "")
        v.Set("page", strconv.Itoa(page))
        v.Set("limit", "200")
        v.Set("seccion","")
        v.Set("iniDate", opts.dayFrom)
        v.Set("endDate", opts.dayTo)

        u := "http://www.milenio.com/milappservices/search.json?" + v.Encode()

        //fmt.Println(u)
        req, err := http.NewRequest("GET", u, nil)
        if err != nil {
            return err
        }
        resp, err := client.Do(req)
        if err != nil {
            return err
        }
        b, err := ioutil.ReadAll(resp.Body)
        resp.Body.Close()
        if err != nil {
            return err
        }

        // kill annoying wrapper
        b = bytes.TrimSpace(b)
        b = bytes.TrimPrefix(b, []byte("("))
        b = bytes.TrimSuffix(b, []byte(")"))

        err = json.Unmarshal(b, &raw)
        if err != nil {
            return err
        }

//        fmt.Printf("%q\n", raw);

        if(raw.Data.Results=="") {
            break
        }

        root,err := html.Parse( strings.NewReader(raw.Data.Results))
        if err != nil {
            return err
        }


		links, err := grabLinks(root, linkSel, u)
		if err != nil {
			return err
		}

		for _, l := range links {
			fmt.Println(l)
		}
	}
	return nil
}
