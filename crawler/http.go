package main

import (
	"regexp"
)

// Special characters found in path extracted from https://datatracker.ietf.org/doc/html/rfc3986#section-3.3
// this abobinations are used to find links instead of using the wasm-limited native parser
const quotedUrlPattern = `"(?P<Link>` +
	`(?P<Proto>http[s]:\/\/?)?` +
	`(?P<Domain>([0-9A-Za-z]+\.)*[A-Za-z]+)?` +
	`(?P<Path>\/[0-9A-Za-z\.\-_\~\!$&'\(\)\*\+,;=:@]+)?` +
	`[^"]*)"`
const hrefPattern = `<a\s+(?:[^>]*?\s+)?href=` + quotedUrlPattern

type link struct {
	Full   string
	Proto  string
	Domain string
	Path   string
}

func (l link) String() string {
	return l.Proto + l.Domain + l.Path
}

// create a link from a match retrieved with one of the patterns above
func linkFromMatch(match []string) link {
	return link{
		Proto:  match[2],
		Domain: match[3],
		Path:   match[5],
	}
}

// extract every crawleable link from a given page,
// excluding the link the page was retrieved from
func extractLinks(fromUrl string, html string) []link {
	r := regexp.MustCompile(quotedUrlPattern)
	match := r.FindStringSubmatch("\"" + fromUrl + "\"")
	srcLink := linkFromMatch(match)

	r = regexp.MustCompile(hrefPattern)
	matches := r.FindAllStringSubmatch(html, -1)

	// avoid repetition
	linkSet := make(map[link]bool)

	for _, match = range matches {
		link := linkFromMatch(match)

		// mailto, magnets or similar links
		if link.Path == "" && link.Proto != "https://" && link.Proto != "http://" {
			continue
		}

		// relative paths
		if link.Domain == "" {
			link.Proto = srcLink.Proto
			link.Domain = srcLink.Domain
		}

		// same link
		if link.String() == srcLink.String() {
			continue
		}
		linkSet[link] = true
	}

	i := 0
	links := make([]link, len(linkSet))
	for k := range linkSet {
		links[i] = k
		i++
	}

	return links
}
