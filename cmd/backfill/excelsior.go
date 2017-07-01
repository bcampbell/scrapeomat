package main

import (
    "fmt"
    "github.com/andybalholm/cascadia"
    "github.com/bcampbell/arts/util"
    "github.com/bcampbell/htmlutil"
//  "golang.org/x/net/html"
    "net/http"
    "net/url"
    "strconv"
    "regexp"
//  "os"
//  "strings"
//  "time"
)

// search page straightforward enough, but only returns first 10000 articles
// so to go back further have to split inro sections
//
// http://www.excelsior.com.mx/buscador?b=una&f={%22periodo%22%3A365%2C%22tipo%22%3A%22articulo%22%2C%22seccion%22%3A%22nacional%22}&p=1000
func DoExcelsior(opts *Options) error {

    filts := []string{}
    sections := []string{ "nacional","global","dinero","comunidad","adrenalina",
        "funcion","hacker","expresiones" }
//    types = :=[]string{ "articulo", "columna" }
    for _,section := range sections {
        f := fmt.Sprintf(`{"periodo":365,"tipo":"articulo","seccion":"%s"}`, section)
        filts = append(filts,f)
    }
    filts = append(filts,`{"periodo":365,"tipo":"columna"}`)



    //_,dayTo,err := opts.parseDays()

    client := &http.Client{
        Transport: util.NewPoliteTripper(),
    }
    resultSel := cascadia.MustCompile("#imx-resultados-lista li")
    linkSel := cascadia.MustCompile("h3 a")
    dateSel := cascadia.MustCompile(".imx-nota-fecha")

    dayPat := regexp.MustCompile( `(\d{1,2})/(\d{1,2})/(\d{4})`)

    maxPage := 1000;    // 10 per page - clips out at page 1000 :-(

    for _,filt := range filts {

        for page := 1; page<=maxPage; page++ {

            v := url.Values{}
            v.Set("b","una")
            v.Set("f",filt)
            v.Set("p",strconv.Itoa(page))

            u := "http://www.excelsior.com.mx/buscador?" + v.Encode()

            root, err := fetchAndParse(client, u)
            if err != nil {
                return fmt.Errorf("%s failed: %s\n", page, err)
            }

            for _,item := range resultSel.MatchAll(root) {
                link := linkSel.MatchFirst(item)
                dt := dateSel.MatchFirst(item)
                href := GetAttr(link,"href")


                m := dayPat.FindStringSubmatch( htmlutil.TextContent(dt))

//                nDay,_ := strconv.Atoi(m[1])
//                nMonth,_ := strconv.Atoi(m[2])
//                nYear,_ := strconv.Atoi(m[3)

                // cheese out - only want 2017
                if m[3] == "2016" {
                    page = maxPage
                    continue
                }

                fmt.Println(href)
            }
        }
    }

    return nil
}

