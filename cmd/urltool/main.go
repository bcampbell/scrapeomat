package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/purell"
	"io"
	"net/url"
	"os"
	"strings"
)

type Options struct {
	cutParts string
	site     bool
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] FILES(s)...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, `

Tool to manipulate URL strings.
URLs are read from read from stdin or FILES(s) (if specified).
Writes the resulting URLs to stdout.

can filter out:
s  scheme
u  username/password
h  hostname (&port)
p  path
q  query
f  fragment

options:
`)
		flag.PrintDefaults()
	}

	opts := Options{}

	flag.StringVar(&opts.cutParts, "c", "", "remove the specified parts (any of 'suhpqf')")
	flag.BoolVar(&opts.site, "s", false, "just the site url (equivalent to -c pqf) eg http://example.com/foo/bar?id=20#wibble -> http://example.com")
	flag.Parse()

	if opts.site {
		if opts.cutParts != "" {
			fmt.Fprintf(os.Stderr, "ERROR: -c and -s are mutually exclusive")
		}
		opts.cutParts = "pqf" // cut off path, query and fragment
	}

	infiles := []string{}
	if flag.NArg() == 0 {
		// default to stdin if no input files
		infiles = append(infiles, "-")
	} else {
		infiles = append(infiles, flag.Args()...)
	}

	for _, infile := range infiles {
		err := doFile(infile, &opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}

func doFile(filename string, opts *Options) error {

	var infile io.Reader
	if filename == "-" {
		infile = os.Stdin
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		infile = f
		defer f.Close()
	}

	scanner := bufio.NewScanner(infile)
	for scanner.Scan() {
		raw := scanner.Text()

		u, err := url.Parse(raw)
		if err != nil {
			return err
		}

		zeroParts(u, opts.cutParts)

		// Apply safe normalisations
		purell.NormalizeURL(u, purell.FlagsSafe)

		fmt.Println(u.String())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func zeroParts(u *url.URL, parts string) {

	if strings.Contains(parts, "s") {
		u.Scheme = ""
	}
	if strings.Contains(parts, "u") {
		u.User = nil
	}
	if strings.Contains(parts, "h") {
		u.Host = ""
	}
	if strings.Contains(parts, "p") {
		u.Path = ""
		u.RawPath = ""
	}
	if strings.Contains(parts, "q") {
		u.RawQuery = ""
		u.ForceQuery = false
	}
	if strings.Contains(parts, "f") {
		u.Fragment = ""
	}
}
