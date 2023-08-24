package main

import (
	"context"
	"fmt"
	"time"

	"capnproto.org/go/capnp/v3"
	http "github.com/mikelsr/ww-webcrawler/services/http/pkg"
	http_api "github.com/mikelsr/ww-webcrawler/services/http/proto/pkg"
	"github.com/wetware/pkg/api/cluster"
	"github.com/wetware/pkg/api/process"
	"github.com/wetware/pkg/cap/csp"
	ww "github.com/wetware/pkg/guest/system"
)

func main() {
	ctx := context.Background()

	// t1 := time.Now()
	self, err := ww.Init(ctx)
	if err != nil {
		panic(err)
	}
	defer func() {
		for _, cap := range self.Caps {
			cap.Release()
		}
	}()
	// t2 := time.Since(t1)
	// fmt.Printf("Took %f seconds to init process.\n", t2.Seconds())

	if len(self.Args) < 4 {
		panic("usage: ww cluster run crawler.wasm <neo4j user> <neo4j password> <neo4j url> <urls...>")
	}

	urls := self.Args[3:]
	fmt.Printf("(%d) will crawl urls: %s\n", self.PID, urls)

	// The host points to its executor.
	host := cluster.Host(self.Caps[0])
	executor, err := executorFromHost(ctx, host)
	if err != nil {
		panic(err)
	}
	defer executor.Release()

	// TODO register http service
	r := http_api.Requester{}
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
	if len(toLinks) == 0 {
		fmt.Printf("(%d) found no new links.\n", self.PID)
	}

	pendingProcs := make([]csp.Proc, 0)
	for _, link := range toLinks {
		time.Sleep(2 * time.Second)
		prefix := fmt.Sprintf("(%d) found %s ...", self.PID, link)
		if !neo4jSession.PageExists(ctx, link) {
			fmt.Printf("%s crawl.\n", prefix)
			bCtx, err := csp.NewBootContext().
				WithArgs(self.Args[0], self.Args[1], self.Args[2], link.String()).
				WithCaps(capnp.Client(host.AddRef()))
			if err != nil {
				panic(err)
			}
			proc, release := csp.Executor(executor).ExecFromCache(
				ctx, self.CID, self.PID, bCtx.Cap(),
			)
			defer release()
			defer proc.Kill(ctx)
			pendingProcs = append(pendingProcs, proc)
		} else {
			fmt.Printf("%s skip.\n", prefix)
		}
		// if err = neo4jSession.RegisterRef(ctx, fromLink, link); err != nil {
		// 	panic(err)
		// }
		_ = fromLink
	}
	for _, proc := range pendingProcs {
		if err = proc.Wait(ctx); err != nil {
			fmt.Printf("Error waiting for subprocess: %s\n", err)
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
