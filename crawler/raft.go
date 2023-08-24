package main

// import (
// 	"context"

// 	"capnproto.org/go/capnp/v3"
// 	raft_api "github.com/mikelsr/raft-capnp/proto/api"
// 	"github.com/wetware/pkg/cap/csp"
// )

// func retrieveRaftNode(context.Context, uint64) (raft_api.Raft, error) {
// 	return raft_api.Raft{}, nil
// }

// func serveRaftRequests(ctx context.Context, bc csp.BootContext, raftNode raft_api.Raft) {
// 	var attrReq csp.AttrRequest
// 	var err error
// 	for {
// 		attrReqC, errC := bc.GoWaitForAttrReq(ctx)
// 		select {
// 		case attrReq = <-attrReqC:
// 			// TODO type check
// 			bc.SendAttrResp(ctx, capnp.Client(raftNode), attrReq.ID)
// 		case err = <-errC:
// 			bc.SendAttrResp(ctx, capnp.Client{}, attrReq.ID)
// 			_ = err // TODO log error
// 			return
// 		case <-ctx.Done():
// 			return
// 		}
// 	}
// }
