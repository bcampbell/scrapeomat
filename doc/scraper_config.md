# Scraper configuration


name
:   the name of the scraper

url
:   the root url for crawling (eg http://example.com/news")

navsel
:   css selector to identify section links
    eg ".navigation-container a"

xnavpats?
:   css selectors to ignore when looking for section links

artpat
:   treat urls matching this regex as articles

xartpat
:   exclude any article urls matching this regex

artform="/SLUG"
:   ID    number with 4+ digits
    YYYY
    MM
    DD
    SLUG

xartform
:   exclude any article urls matching this

hostpat
:   regex. urls from non matching hosts will be rejected
    applies to both discovery and article url filtering
    default: only accept same host as starting url

baseerrorthreshold
:   default 5

nostripquery
:   by default, strip off query part of article urls (eg "?parma=1234")

cookies
:   use cookies when making http requests

pubcode
:   short publication code for this site
