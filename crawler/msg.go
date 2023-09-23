package main

import (
	"encoding/binary"
	"encoding/json"

	"github.com/mikelsr/raft-capnp/raft"
)

type MessageType int

const (
	Visit  MessageType = iota // Visited URL
	Claim                     // Found and claimed URL
	Report                    // Found URL but did not claim it
)

type Message struct {
	MessageType `json:"type"`
	Urls        []string `json:"urls"`
}

func (m Message) AsItem(raftId uint64) (raft.Item, error) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, raftId)
	mb, err := json.Marshal(m)
	if err != nil {
		return raft.Item{}, err
	}
	return raft.Item{
		Key:   b,
		Value: mb,
	}, nil
}

func (m *Message) FromItem(item raft.Item) error {
	return json.Unmarshal(item.Value, m)
}
