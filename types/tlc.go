package types

import (
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"sync"
	"time"
)

type TLCRound struct {
	clientMessage *TLCMessage
	confirmedMessages map[uint32]*TLCMessage
}

func initTLCRound() *TLCRound {
	return &TLCRound{
		clientMessage:     nil,
		confirmedMessages: make(map[uint32]*TLCMessage),
	}
}

type TLC struct {
	majority uint64
	nodeName string
	acks     map[uint32]map[string]struct{} // Set of all peers who acknowledged TLCMessage with this ID
	tickers  map[uint32]chan bool
	rounds   map[uint32]*TLCRound
	currentRound uint32
	sync.RWMutex
}

func InitTLCStruct(numNodes uint64, name string) *TLC {
	/*Initializes a structure of TLC, which takes care  of ACKS*/
	majority := uint64(numNodes/2) + 1
	log.Println(majority)
	return &TLC{
		majority: majority,
		nodeName: name,
		acks:     make(map[uint32]map[string]struct{}),
		tickers:  make(map[uint32]chan bool),
	}
}

func (tlc *TLC) AddUnconfirmed(uid uint32) {
	tlc.Lock()
	defer tlc.Unlock()
	if _, ok := tlc.acks[uid]; ok {
		log.Println("Message already recorded")
	}
	tlc.acks[uid] = map[string]struct{}{tlc.nodeName: struct{}{}} // Count the node as first ACK
}

func (tlc *TLC) RegisterTicker(uid uint32, seconds time.Duration, callback func()) {
	tlc.Lock()
	defer tlc.Unlock()
	if t, ok := tlc.tickers[uid]; ok { // Check if no running tickers
		t <- true
		delete(tlc.tickers, uid)
	}
	ticker := utils.NewTicker(callback, seconds)
	tlc.tickers[uid] = ticker
}

func (tlc *TLC) ClearAcks(uid uint32) []string {
	tlc.Lock()
	defer tlc.Unlock()
	if w, ok := tlc.acks[uid]; ok {
		witnesses := utils.MapToSlice(w)
		delete(tlc.acks, uid)
		return witnesses
	}
	return nil
}
func (tlc *TLC) StopTicker(uid uint32) {
	tlc.Lock()
	defer tlc.Unlock()
	if t, ok := tlc.tickers[uid]; ok {
		t <- true
		delete(tlc.tickers, uid)
	}
}

func (tlc *TLC) AddAck(ack *TLCAck) bool {
	/*Adds an ack to the structure and returns a flag indicating whether we were waiting for this ACK ID*/
	tlc.Lock()
	defer tlc.Unlock()
	if _, ok := tlc.acks[ack.ID]; ok {
		tlc.acks[ack.ID][ack.Origin] = struct{}{} // Record this ack
		return true
	}
	return false
}

func (tlc *TLC) HasMajority(uid uint32) bool {
	tlc.RLock()
	defer tlc.RUnlock()
	if acks, ok := tlc.acks[uid]; ok {
		return uint64(len(acks)) >= tlc.majority
	}
	return false
}
