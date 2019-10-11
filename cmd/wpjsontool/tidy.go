package main

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"regexp"
	"strings"
)

func SanitiseHTMLString(h string) (string, error) {
	bod := &html.Node{
		Type:     html.ElementNode,
		Data:     "body",
		DataAtom: atom.Body,
	}

	nodes, err := html.ParseFragment(strings.NewReader(h), bod)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, n := range nodes {
		TidyNode(n)
		err = html.Render(&b, n)
		if err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

func SingleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// a list of allowed elements and their allowed attrs
// all missing elements or attrs should be stripped
var elementWhitelist = map[atom.Atom][]atom.Atom{
	// basing on list at  https://developer.mozilla.org/en-US/docs/Web/Guide/HTML/HTML5/HTML5_element_list

	//Sections
	atom.Section: {},
	// atom.Nav?
	atom.Article: {},
	atom.Aside:   {},
	atom.H1:      {},
	atom.H2:      {},
	atom.H3:      {},
	atom.H4:      {},
	atom.H5:      {},
	atom.H6:      {},
	atom.Header:  {}, // should disallow?
	atom.Footer:  {}, // should disallow?
	atom.Address: {},
	//atom.Main?

	// Grouping content
	atom.P:          {},
	atom.Hr:         {},
	atom.Pre:        {},
	atom.Blockquote: {},
	atom.Ol:         {},
	atom.Ul:         {},
	atom.Li:         {},
	atom.Dl:         {},
	atom.Dt:         {},
	atom.Dd:         {},
	atom.Figure:     {},
	atom.Figcaption: {},
	atom.Div:        {},

	// Text-level semantics
	atom.A:      {atom.Href},
	atom.Em:     {},
	atom.Font:   {},
	atom.Strong: {},
	atom.Small:  {},
	atom.S:      {},
	atom.Cite:   {},
	atom.Q:      {},
	atom.Dfn:    {},
	atom.Abbr:   {atom.Title},
	// atom.Data
	atom.Time: {atom.Datetime},
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
	atom.Br:   {},
	atom.Wbr:  {},

	// Edits
	atom.Ins: {},
	atom.Del: {},

	//Embedded content
	atom.Img: {atom.Src, atom.Alt},
	// atom.Video?
	// atom.Audio?
	// atom.Map?
	// atom.Area?
	// atom.Svg?
	// atom.Math?

	// Tabular data
	atom.Table:    {},
	atom.Caption:  {},
	atom.Colgroup: {},
	atom.Col:      {},
	atom.Tbody:    {},
	atom.Thead:    {},
	atom.Tfoot:    {},
	atom.Tr:       {},
	atom.Td:       {},
	atom.Th:       {},

	// Forms

	// Interactive elements

}

func filterAttrs(n *html.Node, fn func(*html.Attribute) bool) {
	var out = make([]html.Attribute, 0)
	for _, a := range n.Attr {
		if fn(&a) {
			out = append(out, a)
		}
	}
	n.Attr = out
}

// getAttr retrieved the value of an attribute on a node.
// Returns empty string if attribute doesn't exist.
func getAttr(n *html.Node, attr string) string {
	for _, a := range n.Attr {
		if a.Key == attr {
			return a.Val
		}
	}
	return ""
}

// Tidy up extracted content into something that'll produce reasonable html when
// rendered
// - remove comments
// - trim empty text nodes
// - TODO make links absolute
func TidyNode(n *html.Node) {

	// kill comments
	if n.Type == html.CommentNode {
		if n.Parent != nil {
			n.Parent.RemoveChild(n)
			return
		}
	}

	// trim excessive leading/trailing space in text nodes, and cull empty ones
	if n.Type == html.TextNode {
		leadingSpace := regexp.MustCompile(`^\s+`)
		trailingSpace := regexp.MustCompile(`\s+$`)
		txt := leadingSpace.ReplaceAllStringFunc(n.Data, func(in string) string {
			if strings.Contains(in, "\n") {
				return "\n"
			} else {
				return " "
			}
		})
		txt = trailingSpace.ReplaceAllStringFunc(n.Data, func(in string) string {
			if strings.Contains(in, "\n") {
				return "\n"
			} else {
				return " "
			}
		})
		txt = strings.TrimSpace(txt)
		if len(txt) == 0 && n.Parent != nil {
			n.Parent.RemoveChild(n)
			return
		} else {
			n.Data = txt
		}
	}

	// remove any elements not on the whitelist
	if n.Type == html.ElementNode {
		allowedAttrs, whiteListed := elementWhitelist[n.DataAtom]
		if !whiteListed {
			if n.Parent != nil {
				n.Parent.RemoveChild(n)
			}
			return
		}

		// remove attrs not on whitelist
		filterAttrs(n, func(attr *html.Attribute) bool {
			for _, allowed := range allowedAttrs {
				if attr.Key == allowed.String() {
					return true
				}
			}
			return false
		})

		// special logic for images - strip out ones with huge URIs (eg embedded
		// 'data:' + base64 encoded images)
		if n.DataAtom == atom.Img {
			const maxSrcURI = 1024
			src := getAttr(n, "src")
			if len(src) > maxSrcURI && n.Parent != nil {
				n.Parent.RemoveChild(n)
				return
			}
		}
	}

	// recurse
	for child := n.FirstChild; child != nil; {
		c := child
		// fetch next one in advance (because current one be removed)
		child = child.NextSibling
		TidyNode(c)
	}
}
