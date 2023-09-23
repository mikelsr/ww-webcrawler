package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/mikelsr/raft-capnp/raft"
	ww "github.com/wetware/pkg/guest/system"

	http "github.com/mikelsr/ww-webcrawler/services/http/pkg"
)

const (
	HTTP   = iota // ID used in the capstore for the HTTP capability.
	PREFIX        // Prefix used to store raft node capabilities of this Raft cluster.
	NODES         // Number of nodes (coord).
	URL           // Entrypoint URL (coord).
	// Optional NEO4J parameters.
	DB_ENPOINT
	DB_USER
	DB_PASS
	RAFT_LINK /* ID of a known raft node, mainly the coordinator.
	Coordinator will not receive raft link. */

	COORD_ARGS       = 4  // Number of arguments the first process will get.
	COORD_ARGS_NEO4J = 7  // Number of arguments the first process will get, if Neo4j info is provided.
	ID_BASE          = 16 // ID encoding base.
	USAGE            = "ww cluster run ./wasm/crawler.wasm <http capstore key> <nodes> <url> <?neo4j endpoint> <?neo4j user> <?neo4j password>"

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

	requester := http.Requester(httpCli)

	// Build the crawler.
	crawler := Crawler{
		Cancel: cancel,
		Prefix: ww.Args()[PREFIX],

		Urls: Urls{
			LocalQueue: NewUniqueQueue[string](QUEUE_CAP),
			GlobalPool: NewSet[string](),
			Claimed:    NewTimedSet[string](),
			Visited:    NewSet[string](),
		},

		Http: Http{
			Key:       ww.Args()[HTTP],
			Requester: requester,
		},
		Ww: Ww{
			Session:  sess,
			Executor: sess.Exec(),
			CapStore: cs,
		},
	}

	if dbEnabled() {
		neo4jSession := Neo4j{
			Http: requester,
			Login: LoginInfo{
				Endpoint: ww.Args()[DB_ENPOINT],
				Username: ww.Args()[DB_USER],
				Password: ww.Args()[DB_PASS],
			},
		}
		crawler.DBWrite = neo4jSession.RegisterRefs
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
}

func isCoordinator() bool {
	return len(ww.Args()) == COORD_ARGS || len(ww.Args()) == COORD_ARGS_NEO4J
}

func dbEnabled() bool {
	return len(ww.Args()) >= COORD_ARGS_NEO4J &&
		ww.Args()[DB_ENPOINT] != "" &&
		ww.Args()[DB_USER] != "" &&
		ww.Args()[DB_PASS] != ""
}

func parseUint64(s string, base int) uint64 {
	n, err := strconv.ParseUint(s, base, 64)
	if err != nil {
		panic(err)
	}
	return n
}
