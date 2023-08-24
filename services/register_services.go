package main

import (
	"context"
	"log/slog"
	"path"
	"runtime"

	"github.com/libp2p/go-libp2p/core/record"
	http "github.com/mikelsr/ww-webcrawler/services/http/pkg/server"
	http_api "github.com/mikelsr/ww-webcrawler/services/http/proto/pkg"
	"github.com/wetware/pkg/cap/host"
	"github.com/wetware/pkg/client"
	"github.com/wetware/pkg/system"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := http_api.Requester_ServerToClient(http.HttpServer{})

	h, err := client.NewHost()
	if err != nil {
		panic(err)
	}

	host, err := system.Bootstrap[host.Host](ctx, h, client.Dialer{
		Logger:   slog.Default(),
		NS:       "ww",
		Peers:    []string{},
		Discover: bootstrapAddr(),
	})
	if err != nil {
		panic(err)
	}
	defer host.Release()

	reg, release := host.Registry(ctx)
	defer release()

	pk, _ := h.ID().ExtractPublicKey()
	envelope := &record.Envelope{
		PublicKey:   pk,
		PayloadType: []byte{}, // TODO
		RawPayload:  []byte{}, // TODO build from r
	}
	topic := nil // TODO
	reg.Provide(ctx, topic, envelope)
	// TODO block until cancel
}

func bootstrapAddr() string {
	return path.Join("/ip4/228.8.8.8/udp/8822/multicast", loopback())
}

func loopback() string {
	switch runtime.GOOS {
	case "darwin":
		return "lo0"
	default:
		return "lo"
	}
}
