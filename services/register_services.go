package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"

	"capnproto.org/go/capnp/v3"
	"github.com/libp2p/go-libp2p"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	tcp "github.com/libp2p/go-libp2p/p2p/transport/tcp"

	"github.com/wetware/pkg/auth"
	"github.com/wetware/pkg/boot"
	"github.com/wetware/pkg/vat"

	http "github.com/mikelsr/ww-webcrawler/services/http/pkg/server"
	http_api "github.com/mikelsr/ww-webcrawler/services/http/proto/pkg"
)

const ns = "ww"
const usage = "usage: register_services <key>"

func exit(cause string) {
	fmt.Println(cause)
	os.Exit(1)
}

func key() string {
	if len(os.Args) < 2 {
		exit(usage)
	}
	return os.Args[1]
}

func main() {
	k := key()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := http_api.Requester_ServerToClient(http.HttpServer{})

	h, err := libp2p.New(
		libp2p.NoTransports,
		libp2p.NoListenAddrs,
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(quic.NewTransport))
	if err != nil {
		exit(err.Error())
	}

	bootstrap, err := boot.DialString(h, bootstrapAddr())
	if err != nil {
		exit(err.Error())
	}
	defer bootstrap.Close()

	sess, err := vat.Dialer{
		Host:    h,
		Account: auth.SignerFromHost(h),
	}.DialDiscover(ctx, bootstrap, ns)
	if err != nil {
		exit(err.Error())
	}
	defer sess.Release()

	sess.CapStore().Set(ctx, k, capnp.Client(r))

	<-ctx.Done()
	if ctx.Err() != nil {
		exit(ctx.Err().Error())
	}
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
