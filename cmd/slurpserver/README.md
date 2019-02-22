# slurpserver

Provides an http API for serving up articles from a scrapeomat database.

With the -browse option, also supplies an html-based browse interface
for examining articles.

## Commandline options:

  -browse
    	enable html browsing of articles
  -db string
    	database connection string (eg postgres://user:password@localhost/scrapeomat) or set $SCRAPEOMAT_DB
  -port int
    	port to run server on (default 12345)
  -prefix string
    	url prefix (eg "/ukarticles") to allow multiple servers on same port
  -v int
    	verbosity (0=errors only, 1=info, 2=debug)

TODO document:
- running multiple slurpservers on same public port using nginx proxying
- web interface address (scheme://$HOST:$PORT/$PREFIX/browse)
- link to API document with JSON format explained

## prerequisites

The web interface uses `go-bindata` to include `templates/` and `static/`
directly into the binary. So no extra files need to be installed for
deployment.

There are multiple forks of `go-bindata`, but kevinburke's one looks
like the one to pick. You can install it from source with:

  $ go get -u github.com/kevinburke/go-bindata/...


## building

  $ go generate
  $ go build

(the generate step creates `bindata.go` containing the extra files used by
the web interface)


