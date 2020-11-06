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
	format   string
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

formats (using https://example.com:8080/foo/bar?id=1#wibble as example):

host: "example.com"
site: "https://example.com"

-c can filter out:
s  scheme
u  username/password
h  hostname (&port)
n  port
p  path
q  query
f  fragment

options:
`)
		flag.PrintDefaults()
	}

	opts := Options{}

	flag.StringVar(&opts.format, "f", "", "output format (host,site)")
	flag.StringVar(&opts.cutParts, "c", "", "remove the specified parts (any of 'suhnpqf')")
	flag.BoolVar(&opts.site, "s", false, "(DEPRECATED!) just the site url (equivalent to -f site) eg http://example.com/foo/bar?id=20#wibble -> http://example.com")
	flag.Parse()

	if opts.site {
		// -s is deprecated
		if opts.format != "" {
			fmt.Fprintf(os.Stderr, "ERROR: -f and -s are mutually exclusive (use -f site instead)\n")
			os.Exit(1)
		}
		opts.format = "site"
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

		switch opts.format {
		case "host":
			fmt.Println(u.Host)
			break
		case "site":
			u.Path = ""
			u.RawPath = ""
			u.RawQuery = ""
			u.ForceQuery = false
			u.Fragment = ""
			fmt.Println(u.String())
			break
		case "":
			fmt.Println(u.String())
		default:
			return fmt.Errorf("Unknown -f: %s", opts.format)
		}

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
		// strip host:port
		u.Host = ""
	}
	if strings.Contains(parts, "n") {
		// just strip the port
		u.Host = u.Hostname()
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
