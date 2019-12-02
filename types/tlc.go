package types

import (
	"fmt"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"strings"
	"sync"
	"time"
)

type PeerTLC struct {
	currentRound uint32            // The current round it is in
	rounds       map[uint32]uint32 // Maps the round to message ID
}

func initPeerTLC() *PeerTLC {
	return &PeerTLC{
		currentRound: 0,
		rounds:       make(map[uint32]uint32),
	}
}

type TLC struct {
	majority          uint64
	nodeName          string
	acks              map[uint32]map[string]struct{} // Set of all peers who acknowledged TLCMessage with this ID
	tickers           map[uint32]chan bool
	peerTLC           map[string]*PeerTLC // Keeps state of the Current round for all nodes
	confirmedMessages map[string]uint32   // Keeps state of all confirmed messages for this round
	sync.RWMutex
}

func InitTLCStruct(numNodes uint64, name string) *TLC {
	/*Initializes a structure of TLC, which takes care  of ACKS*/
	majority := uint64(numNodes/2) + 1

	peerTLC := make(map[string]*PeerTLC)
	peerTLC[name] = initPeerTLC() // Create a PeerTLC for the given
	return &TLC{
		majority:          majority,
		nodeName:          name,
		acks:              make(map[uint32]map[string]struct{}),
		tickers:           make(map[uint32]chan bool),
		peerTLC:           peerTLC,
		confirmedMessages: make(map[string]uint32),
	}
}

/*============================================= EX 2 ==========================================================*/

func (tlc *TLC) WaitForAcks(uid uint32) {
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

/*============================================= EX 3 ==========================================================*/

func (tlc *TLC) AddUnconfirmed(uid uint32, origin string) {
	// Adds an unconfirmed message to the peer's round
	tlc.Lock()
	defer tlc.Unlock()
	if _, ok := tlc.peerTLC[origin]; !ok { // We don't know about this peer yet
		tlc.peerTLC[origin] = initPeerTLC()
	}
	peer := tlc.peerTLC[origin]
	if _, ok := peer.rounds[peer.currentRound]; !ok {
		peer.rounds[peer.currentRound] = uid
	}
}

func (tlc *TLC) GetMyRound(origin string) uint32 {
	tlc.RLock()
	defer tlc.RUnlock()
	if peer, ok := tlc.peerTLC[origin]; ok {
		return peer.currentRound
	}
	return 0
}

func (tlc *TLC) GetUnconfirmedMessageRound(message *TLCMessage) (uint32, bool) {
	// Returns the round of the given Unconfirmed message, and a flag whether we should increment the flag
	tlc.RLock()
	defer tlc.RUnlock()
	if _, ok := tlc.peerTLC[message.Origin]; !ok {
		// In this case, first unconfirmed from this peer
		return 0, false
	}
	peer := tlc.peerTLC[message.Origin]

	// First check if the message is not in a previous round
	for round, messageID := range peer.rounds {
		if messageID == message.ID {
			return round, false
		}
	}
	// Check that the current round has a message
	if _, ok := peer.rounds[peer.currentRound]; !ok {
		return peer.currentRound, false
	}
	// Otherwise, check if the ID is > than the last round's ID => Then peer has advanced to next round
	if peer.rounds[peer.currentRound] < message.ID { // Means the message ID is in the next round
		return peer.currentRound + 1, true
	}
	return 0, false
}

func (tlc *TLC) IncrementRound(origin string) {
	tlc.Lock()
	defer tlc.Unlock()
	peer := tlc.peerTLC[origin]
	peer.currentRound += 1 // Increment the round
}

func (tlc *TLC) GetConfirmedMessageRound(message *TLCMessage) uint32 {
	tlc.RLock()
	defer tlc.RUnlock()
	if _, ok := tlc.peerTLC[message.Origin]; !ok {
		log.Printf("Confirmed message for unkown peer")
		return 0
	}
	peer := tlc.peerTLC[message.Origin]
	for round, messageID := range peer.rounds {
		if uint32(message.Confirmed) == messageID {
			return round
		}
	}
	return 0
}

func (tlc *TLC) AddConfirmed(uid uint32, origin string) {
	tlc.Lock()
	defer tlc.Unlock()
	if _, ok := tlc.confirmedMessages[origin]; !ok {
		tlc.confirmedMessages[origin] = uid
	}
}

func (tlc *TLC) HasUnconfirmed(origin string) bool {
	tlc.RLock()
	defer tlc.RUnlock()
	if peer, ok := tlc.peerTLC[origin]; ok {
		_, ok := peer.rounds[peer.currentRound]
		return ok
	}
	return false
}

func (tlc *TLC) stopAllRunningTickers() {
	for _, ticker := range tlc.tickers {
		ticker <- true
	}
	tlc.tickers = make(map[uint32]chan bool) // Reset the tickers
}

func (tlc *TLC) IncrementMyRound(origin string) (uint32, []*TLCMessage) {
	/*Increments the round of the current gossiper. Assums all checks have been done prior*/
	tlc.Lock()
	defer tlc.Unlock()
	var currRound uint32
	var confirmedMessages []*TLCMessage
	if peer, ok := tlc.peerTLC[origin]; ok {
		peer.currentRound += 1
		currRound = peer.currentRound
	}
	for origin, id := range tlc.confirmedMessages {
		confirmedMessages = append(confirmedMessages, &TLCMessage{
			Origin: origin,
			ID:     id,
		})
	}
	tlc.confirmedMessages = make(map[string]uint32) // Re-initialize the round
	tlc.stopAllRunningTickers()                     // Stop waiting for all acks
	tlc.acks = make(map[uint32]map[string]struct{}) // Reset the acks
	return currRound, confirmedMessages
}

func (tlc *TLC) ShouldIncrementRound(origin string) bool {
	tlc.RLock()
	defer tlc.RUnlock()
	hasMajority := uint64(len(tlc.confirmedMessages)) >= tlc.majority
	peer := tlc.peerTLC[origin]
	_, hasRoundMessage := peer.rounds[peer.currentRound]
	return hasMajority && hasRoundMessage
}

func PrintTLCMessages(messages []*TLCMessage) string {
	counter := 1
	var prints []string
	for _, m := range messages {
		prints = append(prints, fmt.Sprintf("origin%v %v ID%v %v", counter, m.Origin, counter, m.ID))
		counter += 1
	}
	return strings.Join(prints, ",")
}
