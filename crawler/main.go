package main

import (
	"context"
	"strconv"

	"capnproto.org/go/capnp/v3"
	"github.com/google/uuid"
	"github.com/mikelsr/raft-capnp/raft"
	ww "github.com/wetware/pkg/guest/system"

	http "github.com/mikelsr/ww-webcrawler/services/http/pkg"
)

const (
	// Order of argv items
	HTTP        = iota // ID used in the capstore for the HTTP capability.
	WORKERS            // Number of workers (coord).
	URL                // Entrypoint URL (coord).
	COORDINATOR        // ID used in the CapStore to store the coordinator capability.
	PREFIX             // Prefix used to store raft node capabilities of this Raft cluster.
	WORKER_ID          // Worker ID of this process.
	RAFT_LINK          // ID of a known raft node, mainly the coordinator.

	MAIN = 3 // The main process will receive WORKERS, HTTP, URL, but not the rest.

	ID_BASE          = 16    // ID encoding base.
	ID_OFFSET uint64 = 0x100 // Offset for Raft IDs.
)

var log = raft.DefaultLogger

func main() {
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

	// Build the crawler.
	crawler := Crawler{
		Cancel:    cancel,
		Id:        uuid.NewString(),
		IsWorker:  len(ww.Args()) > MAIN,
		Workers:   make(map[uint64]*Worker),
		NewWorker: make(chan *Worker),

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
	var (
		id     uint64
		prefix string
	)
	if crawler.IsWorker {
		id = parseUint64(ww.Args()[WORKER_ID], ID_BASE)
		prefix = ww.Args()[PREFIX]
	} else {
		id = ID_OFFSET
		prefix = uuid.NewString()
	}

	node := raft.New().
		WithRaftConfig(raft.DefaultConfig()).
		WithRaftStore(raft.DefaultRaftStore).
		WithStorage(raft.DefaultStorage()).
		WithRaftNodeRetrieval(crawler.RetrieveRaftNode).
		WithOnNewValue(raft.NilOnNewValue).
		WithLogger(raft.DefaultLogger).
		WithID(id)

	crawler.Raft = Raft{
		Node:   node,
		Cap:    node.Cap(),
		Prefix: prefix, // We use a prefix to avoid collisions
	}

	// Register raft node in capstore.
	crawler.CapStore.Set(ctx, crawler.IdToKey(crawler.Node.ID), capnp.Client(crawler.Cap))

	// main process
	// if !crawler.IsWorker {
	// 	crawler.Coordinate(ctx)
	// } else { // worker process
	// 	crawler.Work(ctx)
	// }
	crawler.RaftTest(ctx)

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

func parseUint64(s string, base int) uint64 {
	n, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		panic(err)
	}
	return n
}
