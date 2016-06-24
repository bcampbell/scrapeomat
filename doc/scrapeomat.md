#Scrapeomat usage

## Intro

The scrapeomat tool grabs news articles from web sites, extracts the content
and meta data, and loads the results into a postgresql database.


    Usage:
    scrapeomat [options] publication(s)

    Options:
      -a string
            archive dir to dump .warc files into (default "archive")
      -db string
            database connection string (eg postgres://scrapeomat:password@localhost/scrapeomat)
      -discover
            run discovery for target sites, output article links to stdout, then exit
      -i string
            input file of URLs (runs scrapers then exit)
      -l	List target sites and exit
      -s string
            path for scraper configs (default "scrapers")
      -v int
            verbosity of output (0=errors only 1=info 2=debug) (default 1)


## Database connection

You can specify a postgresql connection string via the `-db` flag, but it's
more advisable to use the `SCRAPEOMAT_DB` environment variable instead:

    export SCRAPEOMAT_DB="user=scrape dbname=ukarts host=/var/run/postgresql port=5434 sslmode=disable"

See the [postgresql docs](https://www.postgresql.org/docs/current/static/libpq-connect.html#LIBPQ-CONNSTRING)
for details.

## Running from a list of URLs

Using the `-i` flag, you can skip the discovery phase and instead pass in a
file which lists article URLs to scrape. The file should have one URL per
line.

In this mode, only one publication (scraper config) can be specified. Each
URL in the list is subjected to the URL rules for that publication. URLs
which fail this test are skipped (eg URLs from the wrong domain or which
don't conform to the defined URL patterns).

This mode is useful when backfilling using a list of URLs obtained by other
means, such as the sitemap.xml or via a search engine.




