package main

import (
	"context"
	"fmt"
	"regexp"

	http_api "github.com/wetware/ww/experiments/api/http"
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

// response contains the most basic attributes of an HTTP GET response
type response struct {
	Body   []byte
	Status uint32
	Error  string
}

func (r response) String() string {
	bodyLen := 15
	if len(r.Body) < bodyLen {
		bodyLen = len(r.Body)
	}

	return fmt.Sprintf("status: %d, error: %s, body: %s", r.Status, r.Error, string(r.Body)[:bodyLen])
}

// get uses the getter capability to perform HTTP GET requests
func get(ctx context.Context, getter http_api.HttpGetter, url string) (response, error) {
	f, release := getter.Get(ctx, func(hg http_api.HttpGetter_get_Params) error {
		return hg.SetUrl(url)
	})
	defer release()
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return response{}, err
	}

	status := res.Status()

	resErr, err := res.Error()
	if err != nil {
		return response{}, err
	}

	buf, err := res.Body()
	if err != nil {
		return response{}, err
	}
	body := make([]byte, len(buf)) // avoid garbage-collecting the body
	copy(body, buf)

	return response{
		Body:   body,
		Status: status,
		Error:  resErr,
	}, nil
}
