package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	http "github.com/mikelsr/ww-webcrawler/services/http/pkg"
)

const (
	statusOk  = 200
	noMatches = "[]"
	// TODO replace this one with a valid parsing of the JSON
	noMatchesPrefix = `{"results":[{"columns":["a"],"data":[]}],"errors":[]`
)

type LoginInfo struct {
	Endpoint string
	Username string
	Password string
}

func (li LoginInfo) Headers() map[string]string {
	return defaultHeaders(li.Username, li.Password)
}

type Neo4j struct {
	Login LoginInfo
	Http  http.Requester
}

func (s Neo4j) PageExists(ctx context.Context, page link) bool {
	pageExists := neo4jNodeExistsQuery(page)
	res, err := runQueries(ctx, s.Http, s.Login, pageExists)
	if err != nil {
		return false
	}
	return res.Error != "" || res.Status != statusOk || !strings.HasPrefix(string(res.Body), noMatchesPrefix)
}

func (s Neo4j) RegisterVisit(ctx context.Context, src, dst link) error {
	create := neo4jVisitQuery(dst)
	_, err := runQueries(ctx, s.Http, s.Login, create)
	return err
}

func (s Neo4j) RegisterRefs(ctx context.Context, src link, dsts ...link) error {
	if src == nilLink {
		return errors.New("nil src link")
	}
	stmts := make([]statement, 0, len(dsts)*2+1)
	// merge src
	createSrc := neo4jVisitQuery(src)
	stmts = append(stmts, createSrc)
	for _, dst := range dsts {
		if dst == nilLink {
			continue
		}
		// merge dst
		createDst := neo4jVisitQuery(dst)
		stmts = append(stmts, createDst)
		// point to dst from src
		reference := neo4jReferenceQuerie(src, dst)
		stmts = append(stmts, reference)
	}
	_, err := runQueries(ctx, s.Http, s.Login, stmts...)
	return err
}

func defaultHeaders(username, password string) map[string]string {
	return map[string]string{
		"Authorization": http.NewBasicAuth(username, password),
		"Accept":        "application/json;charset=UTF-8",
		"Content-Type":  "application/json",
	}
}

func pageNode(page link) string {
	return fmt.Sprintf(
		":WebPage {url: \"%s\", domain: \"%s\", path: \"%s\"}",
		page,
		page.Domain,
		page.Path,
	)
}

func neo4jNodeExistsQuery(page link) statement {
	query := fmt.Sprintf(
		"MATCH (a%s) RETURN a",
		pageNode(page),
	)
	return statement{Statement: query}
}

func neo4jVisitQuery(page link) statement {
	query := fmt.Sprintf(
		"MERGE (%s)",
		pageNode(page),
	)
	return statement{Statement: query}
}

func neo4jReferenceQuerie(src, dst link) statement {
	query := fmt.Sprintf(
		`MATCH (fromPage%s)
		MATCH (toPage%s)
		MERGE (fromPage)-[:REFERENCES]->(toPage)`,
		pageNode(src),
		pageNode(dst),
	)
	return statement{Statement: query}
}

type statements struct {
	Statements []statement `json:"statements"`
}

type statement struct {
	Statement string `json:"statement"`
}

func runQueries(ctx context.Context, requester http.Requester, li LoginInfo, stmts ...statement) (http.Response, error) {
	s := statements{
		Statements: stmts,
	}
	body, _ := json.Marshal(s)
	res, err := requester.Post(ctx, li.Endpoint, li.Headers(), body)
	if err != nil {
		return http.Response{}, err
	}
	if res.Error != "" {
		return res, errors.New(res.Error)
	}
	return res, nil
}
