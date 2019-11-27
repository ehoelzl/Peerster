package types

import (
	"fmt"
	"github.com/ehoelzl/Peerster/utils"
	"net"
	"strings"
)

type Message struct {
	Text        string
	Destination *string
	File        *string
	Request     *[]byte
	Budget      *uint64
	Keywords    *[]string
}

type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

type RumorMessage struct { // No need to add RelayPeerAddr as it is deduced from sending node
	Origin string
	ID     uint32
	Text   string
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

type StatusPacket struct {
	Want []PeerStatus
}

type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}

type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
}

type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
}

type GossipPacket struct {
	Simple        *SimpleMessage
	Rumor         *RumorMessage
	Status        *StatusPacket
	Private       *PrivateMessage
	DataRequest   *DataRequest
	DataReply     *DataReply
	SearchRequest *SearchRequest
	SearchReply   *SearchReply
}

func (sp *StatusPacket) PrintStatusMessage(sender *net.UDPAddr) {
	peerStatus := sp.Want
	var statusString []string
	for _, p := range peerStatus {
		statusString = append(statusString, fmt.Sprintf("peer %v nextID %v", p.Identifier, p.NextID))
	}
	fmt.Printf("STATUS from %v %v\n", sender.String(), strings.Join(statusString, " "))
}

func (sp *StatusPacket) ToMap() map[string]uint32 {
	statusMap := make(map[string]uint32)
	for _, ps := range sp.Want {
		statusMap[ps.Identifier] = ps.NextID
	}
	return statusMap
}

func (sr *SearchReply) Print() {
	if sr.Results == nil {
		return
	}
	for _, res := range sr.Results {
		chunkString := strings.Trim(strings.Replace(fmt.Sprint(res.ChunkMap), " ", ",", -1), "[]")
		fmt.Printf("FOUND match %v at %v metafile=%v chunks=%v\n", res.FileName, sr.Origin, utils.ToHex(res.MetafileHash), chunkString)
	}
}
