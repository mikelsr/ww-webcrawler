package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"capnproto.org/go/capnp/v3"
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

	Cancel    context.CancelFunc
	Id        string
	IsWorker  bool
	Workers   map[uint64]*Worker
	NewWorker chan *Worker
}

type Worker struct {
	Id uint64
	csp.Proc
	api.Crawler
	capnp.ReleaseFunc
}

type Ww struct {
	capstore.CapStore
	csp.Executor
	auth.Session
}

type Http struct {
	Key string
	http.Requester
}

type Raft struct {
	*raft.Node               // implementation
	Cap        raft_api.Raft // capability

	Prefix string
}

// Retrieve a Raft Node capability from the CapStore.
func (c *Crawler) RetrieveRaftNode(ctx context.Context, id uint64) (raft_api.Raft, error) {
	r, err := c.CapStore.Get(ctx, c.IdToKey(id))
	if err != nil {
		return raft_api.Raft{}, nil
	}
	return raft_api.Raft(r.AddRef()).AddRef(), nil // TODO mikel
}

func (c *Crawler) IdToKey(id uint64) string {
	return fmt.Sprintf("%s-%x", c.Prefix, id)
}

func (c *Crawler) RaftTest(ctx context.Context) {
	if c.IsWorker {
		go c.Node.Start(ctx)
		// Find the raft node of the coordinator process
		if err := c.join(ctx); err != nil {
			panic(err)
		}
	} else {
		c.Init()
		go c.Node.Start(ctx)
		for c.Raft.Raft.Status().Lead != c.ID {
			time.Sleep(10 * time.Millisecond)
		}
		c.spawnRaft(ctx)
	}
	for {
		log.Infof("[%x] peers: %v\n", c.ID, c.Raft.Node.Peers())
		time.Sleep(1 * time.Second)
	}
	<-ctx.Done()
}

func (c *Crawler) join(ctx context.Context) error {
	remoteRaftId := parseUint64(ww.Args()[RAFT_LINK], ID_BASE)
	coord, err := c.CapStore.Get(ctx, c.IdToKey(remoteRaftId))
	if err != nil {
		return err
	}
	f, release := raft_api.Raft(coord).Add(ctx, func(r raft_api.Raft_add_Params) error {
		return r.SetNode(c.Cap.AddRef())
	})
	defer release()
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
	nodes, err := s.Nodes()
	if err != nil {
		return err
	}

	peers := make([]raft_api.Raft, nodes.Len())
	for i := 0; i < nodes.Len(); i++ {
		peers[i], err = nodes.At(i)
		if err != nil {
			return err
		}
	}
	for _, peer := range peers {
		c.Raft.View.AddPeer(ctx, peer.AddRef())
	}

	return nil
}

func (c *Crawler) spawnRaft(ctx context.Context) {
	workers := parseUint64(ww.Args()[WORKERS], 10)
	log.Infof("[%x] spawn %d workers\n", c.ID, workers)
	for i := ID_OFFSET + 1; i < ID_OFFSET+workers+1; i++ {
		log.Infof("[%x] spawn worker %x\n", c.ID, i)
		p, release := c.Executor.ExecCached(
			ctx,
			core.Session(c.Session), // TODO if it fails, AddRef
			ww.Cid(),
			ww.Pid(),
			append(ww.Args(), []string{
				c.Id,
				c.Prefix,
				strconv.FormatUint(i, ID_BASE),
				strconv.FormatUint(c.Node.ID, ID_BASE),
			}...)...,
		)
		c.Workers[i] = &Worker{
			Id:          i,
			Proc:        p,
			ReleaseFunc: release,
		}
		log.Infof("[%x] spawn worker %x done\n", c.ID, i)
	}
}

// func (c *Crawler) getCoordinator(ctx context.Context, id string) (api.Crawler, error) {
// 	cli, err := c.CapStore.Get(ctx, id)
// 	return api.Crawler(cli), err
// }

func (c *Crawler) AddWorker(ctx context.Context, call api.Crawler_addWorker) error {
	// if c.IsWorker {
	// 	return errors.New("crawler is a worker")
	// }
	// select {
	// case <-ctx.Done():
	// 	return ctx.Err()
	// case c.NewWorker <- &Worker{
	// 	Crawler: call.Args().Worker(),
	// 	Id:      call.Args().Id(),
	// }:
	// }
	return nil
}

func (c *Crawler) Crawl(ctx context.Context, call api.Crawler_crawl) error {
	return nil
}

func (c *Crawler) addSelf(ctx context.Context, coord api.Crawler) error {
	// f, release := coord.AddWorker(ctx, func(p api.Crawler_addWorker_Params) error {
	// 	return p.SetWorker(api.Crawler_ServerToClient(c))
	// })
	// defer release()
	// <-f.Done()
	return nil
}

func (c *Crawler) Coordinate(ctx context.Context) error {
	// if err := c.SpawnWorkers(ctx, parseUint64(ww.Args()[WORKERS])); err != nil {
	// 	return err
	// }
	return nil
}

func (c *Crawler) SpawnWorkers(ctx context.Context, n uint64) error {
	// if c.IsWorker {
	// 	return errors.New("cannot spawn workers as a worker")
	// }
	// for i := uint64(0); i < n; i++ {
	// 	c.Executor.ExecCached(
	// 		ctx,
	// 		core.Session(c.Session), // TODO if it fails, AddRef
	// 		ww.Cid(),
	// 		ww.Pid(),
	// 		[]string{}..., // TODO
	// 	)
	// }
	// c.waitForWorkers(ctx, n)
	return nil
}

// wait for every worker to be created
func (c *Crawler) waitForWorkers(ctx context.Context, n uint64) error {
	// i := uint64(0)
	// for i < n {
	// 	select {
	// 	case <-ctx.Done():
	// 		return ctx.Err()
	// 	case newWorker := <-c.NewWorker:
	// 		worker := c.Workers[newWorker.Id]
	// 		worker.Crawler = newWorker.Crawler // TODO addref?
	// 	}
	// 	i++
	// }
	return nil
}

func (c *Crawler) assignWork(ctx context.Context, w Worker) error {
	return nil
}

func (c *Crawler) Work(ctx context.Context) error {
	// c.Prefix = ww.Args()[PREFIX]
	// mainRaftId := ww.Args()[RAFT_LINK]
	// id, err := strconv.ParseUint(mainRaftId, 10, 64)
	// if err != nil {
	// 	panic(err)
	// }
	// err = c.Node.Register(ctx, id)
	// if err != nil {
	// 	panic(err)
	// }
	// coord, err := c.getCoordinator(ctx, ww.Args()[COORDINATOR])
	// if err != nil {
	// 	panic(err)
	// }
	// if err = c.addSelf(ctx, coord); err != nil {
	// 	panic(err)
	// }
	// // Block until the context is cancelled.
	// <-ctx.Done()
	return nil
}
