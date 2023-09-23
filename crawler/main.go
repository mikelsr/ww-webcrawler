package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/google/uuid"
	"github.com/mikelsr/raft-capnp/raft"
	ww "github.com/wetware/pkg/guest/system"

	http "github.com/mikelsr/ww-webcrawler/services/http/pkg"
)

const (
	HTTP      = iota // ID used in the capstore for the HTTP capability.
	NODES            // Number of nodes (coord).
	URL              // Entrypoint URL (coord).
	PREFIX           // Prefix used to store raft node capabilities of this Raft cluster.
	RAFT_LINK        // ID of a known raft node, mainly the coordinator.

	COORD_ARGS = 3  // Number of arguments the first process will get.
	ID_BASE    = 16 // ID encoding base.
	USAGE      = "ww cluster run ./wasm/crawler.wasm <http capstore key> <nodes> <url>"

	QUEUE_CAP          = 20              // Maximum size of the local URL queue.
	CLAIM_CHECK_PERIOD = 1 * time.Minute // Period in between claim eviction cheks.
	CLAIM_TIMEOUT      = 5 * time.Minute // Claim timeout.
	URL_ITER_PERIOD    = 1 * time.Second
)

var log = raft.DefaultLogger(true)

func main() {
	if len(ww.Args()) < COORD_ARGS {
		log.Error("not enough arguments")
		log.Error(USAGE)
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Log into wetware cluster.
	sess, err := ww.Bootstrap(ctx)
	if err != nil {
		panic(err)
	}

	// Retrieve the HTTP Requester capability.
	cs := sess.CapStore()
	httpCli, err := cs.Get(ctx, ww.Args()[HTTP])
	if err != nil {
		panic(err)
	}

	// Use a unique prefix to store and retrieve raft capabilities
	// to avoid conflicts.
	var prefix string
	if isCoordinator() {
		prefix = uuid.NewString()
	} else {
		prefix = ww.Args()[PREFIX]
	}

	// Build the crawler.
	crawler := Crawler{
		Cancel: cancel,
		Prefix: prefix,

		Urls: Urls{
			LocalQueue: NewUniqueQueue[string](QUEUE_CAP),
			GlobalPool: NewSet[string](),
			Claimed:    NewTimedSet[string](),
			Visited:    NewSet[string](),
		},

		Http: Http{
			Key:       ww.Args()[HTTP],
			Requester: http.Requester(httpCli),
		},
		Ww: Ww{
			Session:  sess,
			Executor: sess.Exec(),
			CapStore: cs,
		},
	}

	// Build a raft node.
	node := raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithRaftNodeRetrieval(crawler.retrieveRaftNode).
		WithOnNewValue(crawler.onNewValue).
		WithLogger(log)

	crawler.Raft = Raft{
		Node: node,
		Cap:  node.Cap().AddRef(),
	}

	// Register raft node in capstore.
	crawler.CapStore.Set(ctx, crawler.idToKey(crawler.Node.ID), capnp.Client(crawler.Cap.AddRef()))
	// Start the Raft node for this crawler.
	crawler.startRaftNode(ctx)

	// first process will put the arg url on its local queue.
	if isCoordinator() {
		crawler.LocalQueue.Put(ww.Args()[URL])
	}
	crawler.CrawlForever(ctx)

	// ---

	// if len(self.Args) < 4 {
	// 	panic("usage: ww cluster run crawler.wasm <neo4j user> <neo4j password> <neo4j url> <urls...>")
	// }

	// urls := self.Args[3:]
	// fmt.Printf("(%d) will crawl urls: %s\n", self.PID, urls)

	// // The host points to its executor.
	// host := cluster.Host(self.Caps[0])
	// executor, err := executorFromHost(ctx, host)
	// if err != nil {
	// 	panic(err)
	// }
	// defer executor.Release()

	// // TODO register http service
	// r := http_api.Requester{}
	// defer r.Release()

	// requester := http.Requester(r)

	// neo4jLogin := LoginInfo{
	// 	Username: self.Args[0],
	// 	Password: self.Args[1],
	// 	Endpoint: self.Args[2],
	// }
	// neo4jSession := Neo4jSession{
	// 	Http:  requester,
	// 	Login: neo4jLogin,
	// }

	// srcUrl := urls[0]
	// res, err := requester.Get(ctx, srcUrl)
	// if err != nil {
	// 	panic(err)
	// }

	// fromLink, toLinks := extractLinks(srcUrl, string(res.Body))
	// if len(toLinks) == 0 {
	// 	fmt.Printf("(%d) found no new links.\n", self.PID)
	// }

	// pendingProcs := make([]csp.Proc, 0)
	// for _, link := range toLinks {
	// 	time.Sleep(2 * time.Second)
	// 	prefix := fmt.Sprintf("(%d) found %s ...", self.PID, link)
	// 	if !neo4jSession.PageExists(ctx, link) {
	// 		fmt.Printf("%s crawl.\n", prefix)
	// 		bCtx, err := csp.NewBootContext().
	// 			WithArgs(self.Args[0], self.Args[1], self.Args[2], link.String()).
	// 			WithCaps(capnp.Client(host.AddRef()))
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 		proc, release := csp.Executor(executor).ExecFromCache(
	// 			ctx, self.CID, self.PID, bCtx.Cap(),
	// 		)
	// 		defer release()
	// 		defer proc.Kill(ctx)
	// 		pendingProcs = append(pendingProcs, proc)
	// 	} else {
	// 		fmt.Printf("%s skip.\n", prefix)
	// 	}
	// 	// if err = neo4jSession.RegisterRef(ctx, fromLink, link); err != nil {
	// 	// 	panic(err)
	// 	// }
	// 	_ = fromLink
	// }
	// for _, proc := range pendingProcs {
	// 	if err = proc.Wait(ctx); err != nil {
	// 		fmt.Printf("Error waiting for subprocess: %s\n", err)
	// 	}
	// }
}

func isCoordinator() bool {
	return len(ww.Args()) == COORD_ARGS
}

func parseUint64(s string, base int) uint64 {
	n, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		panic(err)
	}
	return n
}
