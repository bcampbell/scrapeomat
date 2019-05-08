# Server Setup

This covers setting up a server to scrape articles, store them and provide an API
endpoint for people to slurp them down for analysis.
It's been used on a Linux server, but should apply equally to most other OSes
- Windows, the BSDs, OSX.

## Quick Overview

There are two components to the system:

1. scrapeomat - responsible for finding and fetching articles, extracting them in a structured form (title, content, date etc), and storing them in the database.
2. slurpserver - provides an HTTP-based slurp API for accessing the article database.

The server setup generally follows these steps:

1. build `scrapeomat` and `slurpserver` from source (they are written in golang).
1. Setup a PostgreSQL database to hold your collected articles.
2. Write config files describing how to scrape the sites you want.
3. Run scrapeomat to perform scraping.
4. Run slurpsurver to provide an API to access the data.


## Building from source

You'll need a working [golang](https://golang.org/) install to build the tools from source.

Grab `github.com/bcampbell/scrapeomat` (via `git` or `go get ...`), then:

    $ cd scrapeomat
    $ go build
    $ cd cmd/slurpserver
    $ go build

And, optionally, `go install` for each one.

(TODO: cover package dependencies. Personally I just keep running `go build` then`go get` anything missing, repeating until done :- )


## Database setup

[PostgreSQL](https://postgresql.org/) is used for storing scraped article data.
It's assumed that you've got a PostgreSQL server installed.

You'll need to create a user and database. For example:

    $ sudo -u postgres createuser --no-createdb {DBUSER}
    $ sudo -u postgres createdb -O {DBUSER} -E utf8 {DBNAME}

PostgreSQL has a complex permissions system which is a little outside the scope of this guide, but there are some notes at the end on setting it up for local development (but probably not suitable for production use).

Once your database is set up, you need to load the base schema:

    $ psql -U {DBUSER} {DBNAME} <store/pg/schema.sql

Your database should now be ready to have articles stored in it.


## Scrapeomat

Scrapeomat is designed to be a long-running process. It will scrape the sites it is configured for, sleep for a couple of hours and repeat.

Scraping a site takes two steps:

1. Article discovery - looking for article URLs, usually by scanning all the "front" pages for sections on the site.
2. Article scraping -  this breaks down further:
    1. Check the database, and ignore article URLs we've already got.
    2. Fetch each URL in turn, extract the content & metadata, store it in the database.


### Configuring Target Sites

Each site you want to scrape requires a configuration entry.
These are read from `.cfg` files in the `scrapers` directory.

A simple config file example, `scrapers/notarealnewspaper.cfg`:
```
[scraper "notarealnewspaper"]

# where we start looking for article links
url="https://notarealnewspaper.com"

# Pattern to identify article URLs
# eg: https://notarealnewspaper.com/articles/moon-made-of-cheese
artform="/articles/SLUG"

# css selector to find other pages to scan for article links
# (we want to match links in the site's menu system)
navsel=".site-header nav a"

```
Notes:

- comments start with `#`.
- the `[scraper ...]` line denotes the start of a scraper definition and assigns it a name ("notarealnewspaper", in this case).
- you can define as many site configs as you need. Usually you'd put them each in their own file, but it's the `[scraper ...]` line which defines them, so you could put them all in a single file, or group them in multiple files, or whatever.
 - The configuration syntax has a lot of options. Look at the [reference doc](scraper_config.md) for more details.


Here's the config for the scrapeomat at http://slurp.stenoproject.org/govblog,
for scraping the UK government blogs. Note that this one uses the sites pagination links rather than menu links to discover articles:
```
[scraper "blog.gov.uk"]

# start crawling on page containing recent posts...
url="https://www.blog.gov.uk/all-posts/"

# ...and follow the pagination links when looking for articles...
navsel="nav.pagination-container a"

# ...but exclude any pages with 2 or more digits (so first 10 pages only)
xnavpat="all-posts/page/[0-9]{2,}"

# we're looking for links with this url form:
# (eg https://ssac.blog.gov.uk/2019/02/20/a-learning-journey-social-security-in-scotland/)
artform="/YYYY/MM/DD/SLUG$"

# allow posts on any subdomains
hostpat=".*[.]blog.gov.uk"
```

Crafting these configurations is usually a case of going to the site and
using your web browsers 'inspect element' feature to examine the structure of
the HTML.


You can run the discovery phase on it's own like this:

    $ ./scrapeomat -discover -v=2 govblog

If all goes well, this will output a list of article links discovered.

(The `-v=2` turns up the verbosity, and will output of each nav page fetched during discovery).



### Running Scrapeomat

Once you have config files set up in the `scrapers` dir, you can run the scrapeomat eg:

    $ ./scrapeomat -v=2 ALL

"ALL" is required to run all the scrapers.
The scrapers will be executed in parallel.

You need to specify which database to store articles in, by passing in a database connection string.
This can be passed in as a commandline flag (`-db`) or via the `SCRAPEOMAT_DB` environment variable, eg:

    $ export SCRAPEOMAT_DB="user=scrape dbname=govblog host=/var/run/postgresql sslmode=disable"
    $ ./scrapeomat ALL

### Installing Scrapeomat as a Service

For a proper server setup, you'd want to set scrapeomat to be automatically run
when the machine starts up and to direct it's `stderr` output to a logfile.

Typically, I use `systemd` and `rsyslog` to handle these.

TODO: add in the govblog example systemd unit file and rsyslog config here.


## SlurpServer

Slurpserver provides an HTTP server and can serve up articles
The read the [API reference](../cmd/slurpserver/api.txt) for details on the endpoints.


### Running

As with scrapeomat, slurpserver needs to be told which database to connect to.
Use the `-db` commandline flag or the `SCRAPEOMAT_DB` environment variable.

To run the slurp api:

    $ slurpserver -port 12345 -prefix /slurptest -v=1

This would accept API requests at http://localhost:12345/slurptest.


### Behaving as a Server

On a production server you'd want SlurpServer running 
TODO: systemd and rsyslog config examples

### Setting up SlurpServer Behind a "real" Web Server

Typically, I run SlurpServers behind an nginx web server.
Nginx handles https and sets the public-facing URLs, and passes requests on to the slurp server running on whatever random local port number... 

The slurpserver "-prefix" flag can be used to run multiple slurp servers behind a single public-facing site. This makes it simpler if you have multiple scrapeomats and multiple databases running.

TODO: add some nginx config examples, and maybe some for other web servers (eg Apache)

## Miscellaneous Notes

### PostgreSQL Permissions

For development, I usually do something like this:

    $ sudoedit /etc/postgresql/9.5/main/pg_hba.conf


add a line:
```
local   {DBNAME}        scrape                    peer map=scrapedev
```

add to `/etc/postgresql/9.5/main/pg_ident.conf`:
```
# MAPNAME       SYSTEM-USERNAME         PG-USERNAME
scrapedev       ben                     scrape
```

Force PostgreSQL to reread the config files:

    $ sudo systemctl reload postgresql

Now, the unix user `ben` should be able to access the database using the postgresql user `scrape`.



