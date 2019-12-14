package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
)

type Options struct {
	cutFragment bool
	cutQuery    bool
}

func main() {
	flag.Usage = func() {

		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "%s [OPTIONS] FILES(s)...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, `

Performs operations on URLs read from FILES(s).
Writes the resulting URLs to stdout.

options:
`)
		flag.PrintDefaults()
	}

	opts := Options{}

	flag.BoolVar(&opts.cutFragment, "f", false, "remove fragments http://example.com/#wibble -> http://example.com/")
	flag.BoolVar(&opts.cutQuery, "q", false, "remove query http://example.com/?id=1&page=2 -> http://example.com/")
	flag.Parse()

	var err error
	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "ERROR: missing input files(s)\n")
		flag.Usage()
		os.Exit(1)
	}

	err = doit(&opts, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func doit(opts *Options, filenames []string) error {
	for _, filename := range filenames {
		err := doFile(filename, opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func doFile(filename string, opts *Options) error {

	var infile io.Reader = os.Stdin
	if filename != "-" {
		infile, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer infile.Close()
	}

	scanner := bufio.NewScanner(infile)
	for scanner.Scan() {
		raw := scanner.Text()

		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if opts.cutFragment {
			u.Fragment = ""
		}
		if opts.cutQuery {
			u.RawQuery = ""
		}
		fmt.Println(u.String())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
