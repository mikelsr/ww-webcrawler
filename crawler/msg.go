package main

import (
	"encoding/binary"
	"encoding/json"

	"github.com/mikelsr/raft-capnp/raft"
)

type MessageType int

const (
	Report MessageType = iota
	Claim
)

type Message struct {
	MessageType `json:"type"`
	Origin      string   `json:"origin"`
	Claimed     []string `json:"claimed"`
	Unclaimed   []string `json:"unclaimed"`
}

func MakeItem(raftId uint64, msg Message) (raft.Item, error) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, raftId)
	m, err := json.Marshal(msg)
	if err != nil {
		return raft.Item{}, err
	}
	return raft.Item{
		Key:   b,
		Value: m,
	}, nil
}
