package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/wetware/ww/api/cluster"
	"github.com/wetware/ww/api/process"
	"github.com/wetware/ww/experiments/api/http"
	"github.com/wetware/ww/experiments/api/tools"
	"github.com/wetware/ww/pkg/csp"
	ww "github.com/wetware/ww/wasm"
)

const hrefPattern = `<a\s+(?:[^>]*?\s+)?href="(?P<Link>[^"]*)"`

func main() {
	ctx := context.Background()

	clients, closers, err := ww.Init(ctx)
	if err != nil {
		panic(err)
	}
	defer closers.Close()

	host := cluster.Host(clients[ww.HOST_INDEX])
	urls, err := csp.Args(clients[ww.ARGS_INDEX]).Args(ctx)
	if err != nil {
		panic(err)
	}

	if len(urls) < 1 {
		panic("missing argument url")
	}

	// The host points to its executor
	executor, err := executorFromHost(ctx, host)
	if err != nil {
		panic(err)
	}
	defer executor.Release()

	// The executor points to the experimental tools
	tools, err := toolsFromExecutor(ctx, executor)
	if err != nil {
		panic(err)
	}
	defer tools.Release()

	// The experimental tools have an http getter
	getter, err := getterFromTools(ctx, tools)
	if err != nil {
		panic(err)
	}
	defer getter.Release()

	srcUrl := urls[0]
	res, err := get(ctx, getter, srcUrl)
	if err != nil {
		panic(err)
	}

	// The output will appear in the executor!
	r := regexp.MustCompile(hrefPattern)
	links := r.FindAllStringSubmatch(string(res.Body), -1)
	newUrls := make([]string, 0)
	for _, link := range links {
		url := link[len(link)-1]
		// Skip all non-http urls
		if !strings.HasPrefix(url, "http:") && !strings.HasPrefix(url, "https:") && !strings.HasPrefix(url, "/") {
			continue
		}
		// Add missing prefix to relative paths
		if strings.HasPrefix(url, "/") {
			url = srcUrl + url
		}
		newUrls = append(newUrls, url)
	}
	fmt.Println("Found http(s) urls:")
	for _, url := range newUrls {
		fmt.Printf("- %s\n", url)
	}
}

func executorFromHost(ctx context.Context, host cluster.Host) (process.Executor, error) {
	f, _ := host.Executor(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return process.Executor{}, err
	}

	return res.Executor(), nil
}

func toolsFromExecutor(ctx context.Context, executor process.Executor) (tools.Tools, error) {
	f, _ := executor.Tools(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return tools.Tools{}, err
	}

	return res.Tools(), nil
}

func getterFromTools(ctx context.Context, tools tools.Tools) (http.HttpGetter, error) {
	f, _ := tools.Http(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return http.HttpGetter{}, err
	}

	return res.Getter(), nil
}
