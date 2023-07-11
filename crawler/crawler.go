package main

import (
	"context"
	"time"

	"github.com/wetware/ww/api/cluster"
	"github.com/wetware/ww/api/process"
	"github.com/wetware/ww/api/pubsub"
)

func main() {
	ctx := context.Background()
	client, closer, err := BootstrapClient(ctx)
	defer closer.Close()
	if err != nil {
		panic(err)
	}
	inbox := process.Inbox(client)
	open, release := inbox.Open(context.TODO(), nil)
	defer release()
	host := cluster.Host(open.Content().AddRef())
	if err = pub(ctx, host.AddRef()); err != nil {
		panic(err)
	}
}

func pub(ctx context.Context, host cluster.Host) error {
	psFuture, release := host.PubSub(ctx, nil)
	defer release()

	<-psFuture.Done()
	ps, err := psFuture.Struct()
	if err != nil {
		return err
	}

	topicFuture, release := ps.PubSub().Join(ctx, func(r pubsub.Router_join_Params) error {
		return r.SetName("topica")
	})
	defer release()
	<-topicFuture.Done()

	topic := topicFuture.Topic()
	err = topic.Publish(ctx, func(t pubsub.Topic_publish_Params) error {
		return t.SetMsg([]byte("Hello!"))
	})
	time.Sleep(5 * time.Second)
	return err
}

// stub function to write the visited url

// stub function to check whether a url has been visited

// stub function to process a webpage
