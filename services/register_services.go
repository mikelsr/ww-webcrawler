package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"

	"capnproto.org/go/capnp/v3"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	tcp "github.com/libp2p/go-libp2p/p2p/transport/tcp"

	cs_api "github.com/wetware/pkg/api/capstore"
	"github.com/wetware/pkg/api/core"
	"github.com/wetware/pkg/auth"
	"github.com/wetware/pkg/boot"
	"github.com/wetware/pkg/cap/view"
	"github.com/wetware/pkg/vat"

	http "github.com/mikelsr/ww-webcrawler/services/http/pkg/server"
	http_api "github.com/mikelsr/ww-webcrawler/services/http/proto/pkg"
)

const ns = "ww"
const usage = "usage: register_services <key>"

func main() {

	// Setup the environment.
	k := key()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := http_api.Requester_ServerToClient(http.HttpServer{})
	defer r.Release()
	h := libp2pHost()
	defer h.Close()

	// Bootstrap p2p connection.
	log("p2p bootstrap...")
	bootstrap, err := boot.DialString(h, bootstrapAddr())
	if err != nil {
		exit(err.Error())
	}
	defer bootstrap.Close()

	// Request a session from a Wetware node.
	log("wetware login...")
	sess, err := vat.Dialer{
		Host:    h,
		Account: auth.SignerFromHost(h),
	}.DialDiscover(ctx, bootstrap, ns)
	if err != nil {
		exit(err.Error())
	}
	defer sess.Release()

	// Register the HTTP requester on the Wetware node.
	log("register http provider...")
	// err = sess.CapStore().Set(ctx, k, capnp.Client(r))
	// if err != nil {
	// 	exit(err.Error())
	// }
	err = registerServices(ctx, sess, k, capnp.Client(r))
	if err != nil {
		exit(err.Error())
	}

	// Provide until the context is cancelled.
	log("providing...")
	<-ctx.Done()
	if ctx.Err() != nil {
		exit(ctx.Err().Error())
	}
	log("graceful exit")
}

// register services in every available capstore.
func registerServices(ctx context.Context, sess auth.Session, key string, service capnp.Client) error {
	e := sess.Exec()
	it, release := sess.View().Iter(ctx, view.NewQuery(view.All()))
	defer release()
	for r := it.Next(); r != nil; r = it.Next() {
		t, _ := r.Peer().MarshalBinary()
		s, self, err := e.DialPeer(ctx, t)
		if err != nil {
			panic(err)
		}
		if self {
			s = core.Session(sess)
		}
		s.CapStore().Set(ctx, func(cs cs_api.CapStore_set_Params) error {
			if err := cs.SetId(key); err != nil {
				return err
			}
			return cs.SetCap(service)
		})
	}
	return nil
}

func key() string {
	if len(os.Args) < 2 {
		exit(usage)
	}
	return os.Args[1]
}

func exit(cause string) {
	log("exit with cause: %s", cause)
	os.Exit(1)
}

func log(t string, v ...interface{}) {
	s := fmt.Sprintf(t, v...)
	fmt.Printf("[provider] %s\n", s)
}

// outbound libp2p host.
func libp2pHost() host.Host {
	h, err := libp2p.New(
		libp2p.NoTransports,
		libp2p.NoListenAddrs,
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(quic.NewTransport))
	if err != nil {
		exit(err.Error())
	}
	return h
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
