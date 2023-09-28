package main

import (
	"regexp"
)

// Special characters found in path extracted from https://datatracker.ietf.org/doc/html/rfc3986#section-3.3
// this abobinations are used to find links instead of using the wasm-limited native parser
const (
	urlPattern = `(?P<Link>` +
		`(?P<Proto>http[s]?:\/\/?)?` +
		`(?P<Domain>([0-9A-Za-z]+\.)*[0-9A-Za-z]+(:[0-9]+))?` +
		`(?P<Path>\/[\/0-9A-Za-z\.\-_\~\!$&'\(\)\*\+,;=:@]+)?` +
		`[^"]*)`
	hrefPattern = `<a\s+(?:[^>]*?\s+)?href="` + urlPattern + `"`
)

var (
	urlExp  = regexp.MustCompile(urlPattern)
	hrefExp = regexp.MustCompile(hrefPattern)
)

var nilLink = link{}

type link struct {
	Proto  string
	Domain string
	Path   string
}

func (l link) String() string {
	return l.Proto + l.Domain + l.Path
}

// create a link from a match retrieved with one of the patterns above
func linkFromMatch(exp *regexp.Regexp, match []string) link {
	names := make(map[string]string)
	for i, name := range exp.SubexpNames() {
		if i != 0 && name != "" {
			names[name] = match[i]
		}
	}
	return link{
		Proto:  names["Proto"],
		Domain: names["Domain"],
		Path:   names["Path"],
	}
}

// extract every crawleable link from a given page,
// excluding the link the page was retrieved from
func extractLinks(fromUrl string, html string) (from link, to []link) {
	match := urlExp.FindStringSubmatch(fromUrl)
	from = linkFromMatch(urlExp, match)

	matches := hrefExp.FindAllStringSubmatch(html, -1)

	// avoid repetition
	linkSet := make(map[link]bool)

	for _, match = range matches {
		link := linkFromMatch(hrefExp, match)

		// mailto, magnets or similar links
		if link.Path == "" && link.Proto != "" && link.Proto != "https://" && link.Proto != "http://" {
			continue
		}

		// relative paths
		if link.Domain == "" {
			link.Proto = from.Proto
			link.Domain = from.Domain
		}

		// same link
		if link.String() == from.String() {
			continue
		}
		linkSet[link] = true
	}

	i := 0
	to = make([]link, len(linkSet))
	for k := range linkSet {
		to[i] = k
		i++
	}

	return from, to
}
