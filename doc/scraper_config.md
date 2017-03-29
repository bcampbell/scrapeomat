# Scraper configuration


name
:   the name of the scraper

url
:   the root url for crawling (eg http://example.com/news")

navsel
:   css selector to identify section links
    eg ".navigation-container a"

xnavpat
:   regex, urls to ignore when looking for section links.
    Multiple xnavpat lines can be used.
    Handy for excluding overly-numerous navigation pages
    eg: "/tag/", "/category/"
    

artpat
:   treat urls matching this regex as articles.

    Multiple artpat (and artform) lines can be used, to
    show multiple URL forms (a lot of sites use multiple
    URL schemes)

    The URLs filtered by artpat (and artform) already have their
    query and fragment parts stripped, unless nostripquery or
    nostripfragment are also set.

xartpat
:   exclude any article urls matching this regex

artform
:   Simplified (non-regexp) pattern matching for URLs.
    ID    number with 4+ digits
    YYYY
    MM
    DD
    SLUG   - anything with a hyphen in it, excluding slashes (/)
             eg moon-made-of-cheese
             moon-made-of-cheese.html
             1234-moon-made-of-cheese^1434
    $      - match end of line

    eg: artform="/SLUG.html$"


xartform
:   exclude any article urls matching this

hostpat
:   regex. urls from non matching hosts will be rejected
    applies to both discovery and article url filtering
    default: only accept same host as starting url

baseerrorthreshold
:   default 5

nostripquery
:   by default, the query part of article urls is stripped off.
    eg "www.example.com/news?article=1234" becomes "www.example.com/news"

    Most of the time, the query part is cruft and/or tracking rubbish, but
    some sites will require it.
    Add `nostripquery` to turn this behaviour off.


cookies
:   Retain cookies when making http requests
    Used mainly for paywalled sites

pubcode
:   short publication code for this site
    TODO: add details.




