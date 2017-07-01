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
    "os"
    "time"
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


    // do it day by day. server gets slow for big ranges
    /*
    days,err := opts.DayRange()
    if err != nil {
        return err
    }
    for _,day := range days {
*/
        for page:=1; ; page++ {
            v := url.Values{}
            v.Set("term", "una")
            v.Set("orderby", "desc")
            v.Set("contentType", "")
            v.Set("page", strconv.Itoa(page))
            v.Set("limit", "100")  // max is 200?
            v.Set("seccion","")
            /*
            v.Set("iniDate", day.Format("2006-01-02"))
            v.Set("endDate", day.Format("2006-01-02"))
            */
            v.Set("iniDate", opts.dayFrom)
            v.Set("endDate", opts.dayTo)

            u := "http://www.milenio.com/milappservices/search.json?" + v.Encode()

            fmt.Fprintln(os.Stderr,"FETCH ", u)
            req, err := http.NewRequest("GET", u, nil)
            if err != nil {
                return err
            }
            
            b := []byte{}
            retries := 0
            skip := false
            for {

                resp, err := client.Do(req)
                if err != nil {
                    return err
                }
                b, err = ioutil.ReadAll(resp.Body)
                resp.Body.Close()
                if err != nil {
                    return err
                }

                if (resp.StatusCode == 200 ) {
                    break
                }

                retries++
                if retries >10 {
                    fmt.Fprintf(os.Stderr,"SKIP %s - too many reties. skip.", u)
                    skip = true;
                    break
                }
                fmt.Fprintf(os.Stderr,"HTTP %d: %s - retry in 5s",resp.StatusCode, u)
                time.Sleep(5*time.Second)
            }
            if skip {
                continue
            }

            // kill annoying wrapper
            b = bytes.TrimSpace(b)
            b = bytes.TrimPrefix(b, []byte("("))
            b = bytes.TrimSuffix(b, []byte(")"))

            err = json.Unmarshal(b, &raw)
            if err != nil {
                return fmt.Errorf("json err, page %d: %s",page,err)
            }

    //        fmt.Printf("%q\n", raw);

            if(raw.Data.Results=="") {
                break
            }

            root,err := html.Parse( strings.NewReader(raw.Data.Results))
            if err != nil {
                return fmt.Errorf("html parse err, page %d: %s",page,err)
            }


            links, err := grabLinks(root, linkSel, u)
            if err != nil {
                return err
            }

            for _, l := range links {
                fmt.Println(l)
            }
        }
        /*
    }
    */
	return nil
}
