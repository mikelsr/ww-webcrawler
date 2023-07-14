package main

import (
	"context"
	"fmt"
	"time"

	"capnproto.org/go/capnp/v3"
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

	t1 := time.Now()
	self, err := ww.Init(ctx)
	if err != nil {
		panic(err)
	}
	defer self.Close()
	t2 := time.Since(t1)
	fmt.Printf("Took %f seconds to init process.\n", t2.Seconds())

	if len(self.Args) < 4 {
		panic("usage: ww cluster run crawler.wasm <neo4j user> <neo4j password> <neo4j url> <urls...>")
	}

	urls := self.Args[3:]
	fmt.Printf("(%x) will crawl urls: %s\n", self.Pid, urls)

	// The host points to its executor.
	host := cluster.Host(self.Caps[0])
	executor, err := executorFromHost(ctx, host)
	if err != nil {
		panic(err)
	}
	defer executor.Release()

	// The executor points to the experimental tools.
	tools, err := toolsFromExecutor(ctx, executor)
	if err != nil {
		panic(err)
	}
	defer tools.Release()

	// The experimental tools have an http client.
	r, err := httpFromTools(ctx, tools)
	if err != nil {
		panic(err)
	}
	defer r.Release()

	requester := http.Requester(r)

	neo4jLogin := LoginInfo{
		Username: self.Args[0],
		Password: self.Args[1],
		Endpoint: self.Args[2],
	}
	neo4jSession := Neo4jSession{
		Http:  requester,
		Login: neo4jLogin,
	}

	srcUrl := urls[0]
	res, err := requester.Get(ctx, srcUrl)
	if err != nil {
		panic(err)
	}

	fromLink, toLinks := extractLinks(srcUrl, string(res.Body))
	pendingProcs := make([]csp.Proc, 0)
	for _, link := range toLinks {
		fmt.Printf("- Found %s\n", link)
		if !neo4jSession.PageExists(ctx, link) {
			fmt.Printf("Spawn crawler for %s\n", link)
			proc, release := csp.Executor(executor).ExecFromCache(
				ctx,
				[]byte(self.Md5Sum),
				self.Pid,
				capnp.Client(csp.NewArgs(self.Args[0], self.Args[1], self.Args[2], link.String())),
				capnp.Client(host.AddRef()),
			)
			defer release()
			defer proc.Kill(ctx)
			pendingProcs = append(pendingProcs, proc)
		} else {
			fmt.Printf("Skip page %s\n", link)
		}
		// if err = neo4jSession.RegisterRef(ctx, fromLink, link); err != nil {
		// 	panic(err)
		// }
		_ = fromLink
		break
	}
	for _, proc := range pendingProcs {
		if err = proc.Wait(ctx); err != nil {
			panic(err)
		}
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

func httpFromTools(ctx context.Context, tools tools.Tools) (http_api.Requester, error) {
	f, _ := tools.Http(ctx, nil)
	<-f.Done()

	res, err := f.Struct()
	if err != nil {
		return http_api.Requester{}, err
	}

	return res.Http(), nil
}
