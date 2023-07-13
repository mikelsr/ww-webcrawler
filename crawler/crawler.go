package main

import (
	"context"
	"fmt"

	"github.com/wetware/ww/api/cluster"
	"github.com/wetware/ww/api/process"
	http_api "github.com/wetware/ww/experiments/api/http"
	"github.com/wetware/ww/experiments/api/tools"
	"github.com/wetware/ww/experiments/pkg/http"
	"github.com/wetware/ww/pkg/csp"
	ww "github.com/wetware/ww/wasm"
)

func main() {
	ctx := context.Background()

	clients, closers, err := ww.Init(ctx)
	if err != nil {
		panic(err)
	}
	defer closers.Close()

	host := cluster.Host(clients[ww.CAPS_INDEX])
	args, err := csp.Args(clients[ww.ARGS_INDEX]).Args(ctx)
	if err != nil {
		panic(err)
	}
	if len(args) < 2 {
		panic("usage: ww cluster run crawler.wasm <md5sum> <urls...>")
	}
	self := []byte(args[0])
	urls := args[1:]
	fmt.Printf("(%x) will crawl urls: %s\n", self, urls)

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
	res, err := http.Get(ctx, getter, srcUrl)
	if err != nil {
		panic(err)
	}

	links := extractLinks(srcUrl, string(res.Body))
	fmt.Println("Found http(s) urls:")
	for _, url := range links {
		fmt.Printf("- Found %s\n", url)
	}
	// for _, url := range newUrls {
	// 	fmt.Printf("- Exploring %s\n", url)
	// 	proc, release := csp.Executor(executor).ExecFromCache(
	// 		ctx,
	// 		[]byte(self),
	// 		capnp.Client(host.AddRef()),
	// 		capnp.Client(csp.NewArgs(string(self), url)),
	// 	)
	// 	defer release()
	// 	if err = proc.Wait(ctx); err != nil {
	// 		panic(err)
	// 	}
	// 	break
	// }
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

func getterFromTools(ctx context.Context, tools tools.Tools) (http_api.HttpGetter, error) {
	f, _ := tools.Http(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return http_api.HttpGetter{}, err
	}

	return res.Getter(), nil
}
