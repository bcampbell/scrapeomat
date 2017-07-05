# Backfilling techniques

Backfilling is the process of collecting URLs of older articles to fill
in gaps in coverage.



## Using `sitemaps.xml`

A lot of sites (most sites?) have a sitemap file, which lists pages
they'd like search engines to index. On a news site, this will usually
include a lot of article links.

Use `sitemapwalker` to grab all the URLs in a site map.

This is a nice generic method for backfilling - if the articles you
want are in the sitemap, you don't need to do anything site-specific.

Some sites only have recent articles in their sitemap. This is great
for filling in recent gaps (eg within the last week).
But if the gap is further back, you'll have to resort to other
methods - most likely some site-specific hackery.

Some sites have _really_ comprehensive sitemaps. For example, the
Independent seems to list it's entire archive of articles back to 2012
or so. In these cases, the list of URLs can be overwhelmingly large.

TODO: document any progress in filtering sitemaps by rough date ranges

## wp-json

TODO


## Site-specific Hackery

If the generic sitemap.xml scanning doesn't cover the articles you're
looking for, you'll probably have to write some custom coding to cover
the site you want.

Such site-specifc hacks are being collected in the `backfill` tool.

Most sites have a search facility which can be used to generate a list
of older articles.

Other sites have good archive sections which can be iterated through.

Either way... coding.

## Scraping articles from backfill lists

Once you've got a list of article URLs, you can scrape them using
the `scrapeomat` tool with the `-i` flag. This flag skips the usual
article-discovery phase for the scraper and instead reads a list of
URLs from a file.

In this mode, only a single scraper can be invoked
- it's assumed that all the URLs in the list file are from the same
publication.

The usual scraper-specific article URL patterns are still applied
to the URLs before scraping, so non-article links will be filtered
out of the list.



