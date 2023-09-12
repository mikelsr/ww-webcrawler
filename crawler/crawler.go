package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	raft_api "github.com/mikelsr/raft-capnp/proto/api"
	"github.com/mikelsr/raft-capnp/raft"
	"github.com/wetware/pkg/api/core"
	"github.com/wetware/pkg/auth"
	"github.com/wetware/pkg/cap/capstore"
	"github.com/wetware/pkg/cap/csp"
	ww "github.com/wetware/pkg/guest/system"

	api "github.com/mikelsr/ww-webcrawler/crawler/proto/pkg"
	http "github.com/mikelsr/ww-webcrawler/services/http/pkg"
)

type Crawler struct {
	Http
	Raft
	Ww

	Prefix string
	Cancel context.CancelFunc
}

type Ww struct {
	capstore.CapStore
	csp.Executor
	auth.Session
}

type Http struct {
	Key string // Key pointing to the HTTP capability in the capstore.
	http.Requester
}

type Raft struct {
	*raft.Node               // Raft node implementation.
	Cap        raft_api.Raft // Raft node capability.
}

// Retrieve a Raft Node capability from the CapStore.
func (c *Crawler) retrieveRaftNode(ctx context.Context, id uint64) (raft_api.Raft, error) {
	r, err := c.CapStore.Get(ctx, c.idToKey(id))
	if err != nil {
		return raft_api.Raft{}, nil
	}
	return raft_api.Raft(r.AddRef()).AddRef(), nil // TODO mikel
}

func (c *Crawler) onNewValue(item raft.Item) error {
	log.Warningf("NEW VALUE: {'%s': '%s'}\n", string(item.Key), string(item.Value))
	return nil
}

// Start a raft node.
func (c *Crawler) startRaftNode(ctx context.Context) {
	// the coordinator will spawn the rest of the crawlers.
	if isCoordinator() {
		c.Init()
		go c.Node.Start(ctx)
		for c.Raft.Raft.Status().Lead != c.ID {
			time.Sleep(10 * time.Millisecond)
		}
		n := parseUint64(ww.Args()[NODES], 10)
		c.spawnCrawlers(ctx, n)
	} else { // the rest will join the coordinator.
		go c.Node.Start(ctx)
		// Join the Raft by joining the coordinator.
		if err := c.join(ctx); err != nil {
			panic(err)
		}
	}
}

// Spawn n crawler processes with raft nodes of this cluster.
func (c *Crawler) spawnCrawlers(ctx context.Context, n uint64) error {
	log.Infof("[%x] spawn %d crawlers\n", c.ID, n)
	for i := uint64(0); i < uint64(n); i++ {
		log.Infof("[%x] spawn crawler %x\n", c.ID, i)
		// p, release := c.Executor.ExecCached(
		// Won't keep track of the other processes.
		_, release := c.Executor.ExecCached(
			ctx,
			core.Session(c.Session),
			ww.Cid(),
			ww.Pid(),
			append(ww.Args(), []string{
				c.Prefix,
				strconv.FormatUint(c.Node.ID, ID_BASE),
			}...)...,
		)
		defer release()
		log.Infof("[%x] spawn crawler %x done\n", c.ID, i)
	}
	return nil
}

// Join the coordinator Raft node.
func (c *Crawler) join(ctx context.Context) error {
	// Find the coordinator.
	remoteRaftId := parseUint64(ww.Args()[RAFT_LINK], ID_BASE)
	// Get the coordinator Raft capability.
	coord, err := c.CapStore.Get(ctx, c.idToKey(remoteRaftId))
	if err != nil {
		return err
	}
	// Add oneself to the Raft.
	f, release := raft_api.Raft(coord).Add(ctx, func(r raft_api.Raft_add_Params) error {
		return r.SetNode(c.Cap.AddRef())
	})
	defer release()
	// Wait until the RPC is done and check that it has no errors.
	<-f.Done()
	s, err := f.Struct()
	if err != nil {
		return err
	}
	if s.HasError() {
		e, err := s.Error()
		if err != nil {
			return err
		}
		return errors.New(e)
	}
	// The coordinator replied with known nodes.
	nodes, err := s.Nodes()
	if err != nil {
		return err
	}
	// Add them to the known nodes list.
	for i := 0; i < nodes.Len(); i++ {
		peer, err := nodes.At(i)
		if err != nil {
			return err
		}
		c.Raft.View.AddPeer(ctx, peer.AddRef())
	}

	return nil
}

// Combine the prefix and id to form the CapStore key for a node.
func (c *Crawler) idToKey(id uint64) string {
	return fmt.Sprintf("%s-%x", c.Prefix, id)
}

func (c *Crawler) Crawl(ctx context.Context, call api.Crawler_crawl) error {
	return nil
}

func (c *Crawler) testPutValue(ctx context.Context) error {
	c.Raft.DirectPut(ctx, raft.Item{
		Key:   []byte(fmt.Sprintf("hi from %d", c.ID)),
		Value: []byte(fmt.Sprintf("there from %d", c.ID)),
	})
	return nil
}

func (c *Crawler) testGetValue(ctx context.Context, k string) error {
	if _, ok := c.Raft.Map.Load(k); ok {
		fmt.Println("found")
		return nil
	}
	fmt.Println("not found")
	return nil
}
