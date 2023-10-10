package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"time"

	raft_api "github.com/mikelsr/raft-capnp/proto/api"
	"github.com/mikelsr/raft-capnp/raft"
	"github.com/wetware/pkg/api/core"
	"github.com/wetware/pkg/auth"
	"github.com/wetware/pkg/cap/capstore"
	"github.com/wetware/pkg/cap/csp"
	ww "github.com/wetware/pkg/guest/system"

	http "github.com/mikelsr/ww-webcrawler/services/http/pkg"
)

type DBWrite func(ctx context.Context, src link, dsts ...link) error

type Crawler struct {
	DBWrite
	Http
	Raft
	Ww

	ReqPool

	Urls
	CrawCtrl // TODO: cancel current crawl if the URL in progress is received as report.
	// Prefix, shared by all members of the crawling cluster.
	Prefix string
	Cancel context.CancelFunc
}

type CrawCtrl struct {
	// "go.uber.org/atomic"
	// atomic.String
	// Cancel context.CancelFunc
}

type ReqPool struct {
	Requests  chan string
	Responses chan http.Response
	Size      int
}

type Urls struct {
	// Contains the URLs to crawl next.
	LocalQueue UniqueQueue[string]
	// Global pool of unassigned URLs.
	GlobalPool Set[string]
	// Unparsed URLs claimed by other peers
	Claimed TimedSet[string]
	// URLs already crawled
	Visited Set[string]
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

// process messages from other crawlers.
func (c *Crawler) onNewValue(item raft.Item) error {
	var err error
	var msg Message
	if err := msg.FromItem(item); err != nil {
		return err
	}
	c.Logger.Debugf("[%x] received message: %v", c.ID, msg)
	switch msg.MessageType {
	case Visit:
		err = c.onUrlVisit(msg)
	case Claim:
		err = c.onUrlClaim(msg)
	case Report:
		err = c.onUrlReport(msg)
	default:
		err = fmt.Errorf("unrecognized MessageType %d", msg.MessageType)
	}
	return err
}

// register visited url and possibly remove it from claims and local queue.
func (c *Crawler) onUrlVisit(msg Message) error {
	for _, url := range msg.Urls {
		c.Visited.Put(url)
		c.Claimed.Pop(url)
		c.LocalQueue.Remove(url)
	}
	return nil
}

// register the claim on a url, which will be either visited or evicted to the global
// queue.
func (c *Crawler) onUrlClaim(msg Message) error {
	for _, url := range msg.Urls {
		if !c.Visited.Has(url) {
			c.Claimed.Put(url)
		} else {
			c.Visited.Put(url) // update last visit time
		}
	}
	return nil
}

// register an unclaimed url in the global queue.
func (c *Crawler) onUrlReport(msg Message) error {
	for _, url := range msg.Urls {
		if !c.Visited.Has(url) {
			c.GlobalPool.Put(url)
		} else {
			c.Visited.Put(url) // update last visit time
		}
	}
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

// Spawn n-1 crawler processes with raft nodes of this cluster.
func (c *Crawler) spawnCrawlers(ctx context.Context, n uint64) error {
	log.Infof("[%x] spawn %d crawlers\n", c.ID, n)
	for i := uint64(1); i < uint64(n); i++ {
		log.Infof("[%x] spawn crawler %x\n", c.ID, i)
		// p, release := c.Executor.ExecCached(
		// Won't keep track of the other processes.
		var release capnp.ReleaseFunc
		if hasDbArgs() {
			_, release = c.Executor.ExecCached(
				ctx,
				core.Session(c.Session),
				ww.Cid(),
				ww.Pid(),
				append(ww.Args(), strconv.FormatUint(c.Node.ID, ID_BASE))...,
			)
		} else {
			newArgs := []string{"", "", "", strconv.FormatUint(c.Node.ID, ID_BASE)}
			_, release = c.Executor.ExecCached(
				ctx,
				core.Session(c.Session),
				ww.Cid(),
				ww.Pid(),
				append(ww.Args(), newArgs...)...,
			)
		}
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

// Crawl until the context is canceled.
func (c *Crawler) CrawlForever(ctx context.Context) error {
	// Start page requesting goroutine.
	go c.sendRequests(ctx)
	// Start page processing goroutine.
	go c.processResponses(ctx)
	// Start claim eviction goroutine.
	go c.evictClaimsForever(ctx)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		url, err := c.NextUrl(ctx)
		if err != nil {
			// don't overload the CPU
			c.Logger.Debugf("[%x] error getting next url: %s", c.ID, err)
			runtime.Gosched()
			// TODO remove, its here to avoid DoS bans on my home network.
			time.Sleep(URL_ITER_PERIOD)
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c.Requests <- url: // pass url to the requesting routine.
		}
	}
}

// Send http requests to at most c.ReqPool.Size urls at any given time.
// URLs are received through c.Requests.
// Results are sent through c.Responses.
func (c *Crawler) sendRequests(ctx context.Context) error {
	turn := make(chan struct{}, c.ReqPool.Size)
	for i := 0; i < c.ReqPool.Size; i++ {
		turn <- struct{}{}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-turn:
			select {
			case <-ctx.Done():
				return ctx.Err()
			case url := <-c.Requests:
				go func() {
					defer func() {
						select {
						case turn <- struct{}{}:
						case <-ctx.Done():
						}
					}()
					c.Logger.Debugf("[%x] crawl %s", c.ID, url)
					res, err := c.Http.Get(ctx, url)
					if err != nil {
						c.Logger.Errorf("[%x] error crawling %s: %s", c.ID, url, err)
					}
					c.Responses <- res
				}()
			}
		}
	}
}

// Get the next url to search:
// 1. Check the local queue. If empty:
// 2. Claim a URL from the local queue. If not possible:
// 3. Run an eviction round. If any claims were evicted, the next call to
// NextUrl should claim it from the global pool.
func (c *Crawler) NextUrl(ctx context.Context) (string, error) {
	if c.LocalQueue.Size() > 0 {
		return c.LocalQueue.Get()
	} else {
		if c.GlobalPool.Size() > 0 {
			url, ok := c.GlobalPool.PopRandom()
			if !ok { // pool was emptied between Size() and PopRandom()
				return "", errors.New("error fetching url from global pool")
			}
			c.claimUrls(ctx, url)
			return url, nil
		} else {
			if c.Claimed.Size() > 0 {
				c.evictClaims(ctx)
			}
			return "", errors.New("no new urls found")
		}
	}
}

func (c *Crawler) processResponses(ctx context.Context) error {
	for {
		select {
		case r := <-c.Responses:
			c.processResponse(ctx, r)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Crawler) processResponse(ctx context.Context, r http.Response) {
	fromLink, toLinks := extractLinks(r.Origin, string(r.Body))
	c.Logger.Debugf("[%x] found %d urls crawling %s: %v", c.ID, len(toLinks), fromLink, toLinks)

	err := c.reportVisits(ctx, r.Origin)
	if err != nil {
		c.Logger.Errorf("[%x] error reporting %s: %s", c.ID, r.Origin, err)
	}

	if c.DBWrite != nil {
		go c.writeRefsToDb(ctx, fromLink, toLinks)
	}

	err = c.sortAndSend(ctx, toLinks...)
	if err != nil {
		c.Logger.Error("[%x] %s", c.ID, err)
	}
}

// Loop claim eviction.
func (c *Crawler) evictClaimsForever(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(CLAIM_CHECK_PERIOD):
			c.evictClaims(ctx)
		}
	}
}

// Moved claimed URLs whose claim has expired to the global queue.
func (c *Crawler) evictClaims(ctx context.Context) {
	c.Claimed.Range(func(key, value any) bool {
		claimDate := value.(time.Time)
		if claimDate.Add(CLAIM_TIMEOUT).Before(time.Now()) {
			url := key.(string)
			c.Claimed.Pop(url)
			c.GlobalPool.Put(url)
		}
		return true
	})
}

// Sorts urls into potential claims and unclaimed, then publicly claims and reports them
// respectively.
func (c *Crawler) sortAndSend(ctx context.Context, urls ...link) error {
	freeSlots := QUEUE_CAP - c.Urls.LocalQueue.Size()
	claimed := make([]string, 0, freeSlots)
	unclaimed := make([]string, 0, len(urls))
	for i := range urls {
		ref := urls[i].String()
		if c.Urls.Visited.Has(ref) {
			continue
		}
		// Leave 1 in 3 URLs unclaimed to better distribute load.
		if len(claimed) < freeSlots && i%3 == 0 {
			claimed = append(claimed, ref)
		} else {
			unclaimed = append(unclaimed, ref)
		}
	}
	c.claimUrls(ctx, claimed...)
	c.reportUrls(ctx, unclaimed...)
	return nil
}

// A URL was visited, other nodes will add it to Visited.
func (c *Crawler) reportVisits(ctx context.Context, urls ...string) error {
	err := c.sendMsg(ctx, Visit, urls...)
	if err != nil {
		c.Logger.Debugf("[%x] failed to report visits to %v: %s", c.ID, urls, err)
	}
	return err
}

// Claims a URL so others don't crawl it too.
func (c *Crawler) claimUrls(ctx context.Context, urls ...string) error {
	claimed := make([]string, 0, len(urls))
	unclaimed := make([]string, 0, len(urls))
	for _, url := range urls {
		if err := c.LocalQueue.Put(url); err == nil {
			claimed = append(claimed, url)
		} else {
			unclaimed = append(unclaimed, url)
		}
	}
	err := c.sendMsg(ctx, Claim, claimed...)
	if err != nil {
		c.Logger.Debugf("[%x] failed to claim %v: %s", c.ID, claimed, err)
	}
	// Report the ones it could not claim.
	c.reportUrls(ctx, unclaimed...)
	return err
}

// Reports unclaimed URLs.
func (c *Crawler) reportUrls(ctx context.Context, urls ...string) error {
	err := c.sendMsg(ctx, Report, urls...)
	if err != nil {
		c.Logger.Debugf("[%x] failed to report %v: %s", c.ID, urls, err)
	}
	return err
}

// Sends a message to other nodes.
func (c *Crawler) sendMsg(ctx context.Context, t MessageType, urls ...string) error {
	if len(urls) <= 0 {
		return nil
	}

	msg := Message{
		MessageType: t,
		Urls:        urls,
	}
	item, err := msg.AsItem(c.Raft.ID)
	if err != nil {
		return err
	}
	return c.Raft.PutItem(ctx, item)
}

// write src->dst relations to the target DB.
func (c *Crawler) writeRefsToDb(ctx context.Context, src link, dsts []link) {
	c.Logger.Debugf("[%x] write refs to db, src: %s, dst: %v", c.ID, src, dsts)
	if err := c.DBWrite(ctx, src, dsts...); err != nil {
		c.Logger.Errorf("[%x] could not write refs to DB: %s", c.ID, err)
	}
}
