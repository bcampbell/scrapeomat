package main

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"strings"
)

var inlineNodes = map[atom.Atom]struct{}{
	atom.A:      {},
	atom.Em:     {},
	atom.Strong: {},
	atom.Small:  {},
	atom.S:      {},
	atom.Cite:   {},
	atom.Q:      {},
	atom.Dfn:    {},
	atom.Abbr:   {},
	// atom.Data
	atom.Time: {},
	atom.Code: {},
	atom.Var:  {},
	atom.Samp: {},
	atom.Kbd:  {},
	atom.Sub:  {},
	atom.Sup:  {},
	atom.I:    {},
	atom.B:    {},
	atom.U:    {},
	atom.Mark: {},
	atom.Ruby: {},
	atom.Rt:   {},
	atom.Rp:   {},
	atom.Bdi:  {},
	atom.Bdo:  {},
	atom.Span: {},
	//	atom.Br:   {},
	atom.Wbr: {},
	atom.Ins: {},
	atom.Del: {},
}

// NodeToText renders HTML as text, using linebreaks for block elements
func NodeToText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	inline := false

	if n.Type == html.ElementNode {
		_, inline = inlineNodes[n.DataAtom]
		// special case for some structural elements
		if n.DataAtom == atom.Html || n.DataAtom == atom.Head || n.DataAtom == atom.Body {
			inline = true
		}
	}

	txt := ""
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		txt += NodeToText(child)
	}

	if !inline {
		txt += "\n"
	}

	return txt
}

// HTMLToText converts html into text, with an attempt to make it
// look nice by plonking line breaks between block elements.
func HTMLToText(h string) (string, error) {
	bod := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}

	nodes, err := html.ParseFragment(strings.NewReader(h), bod)
	if err != nil {
		return "", err
	}

	var txt string
	for _, n := range nodes {
		txt = txt + NodeToText(n)
	}

	return txt, nil
}
