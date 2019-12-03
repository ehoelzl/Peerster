package types

import (
	"fmt"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"strings"
	"sync"
)

type QSC struct {
	currentRound        uint32
	contestant          *TLCMessage
	secondConfirmations []BlockPublish
	chain               []BlockPublish
	sync.RWMutex
}

func InitQSCStruct() *QSC {
	return &QSC{
		currentRound: 0,
	}
}

func (qsc *QSC) IsValidBlock(block BlockPublish) bool {
	qsc.RLock()
	defer qsc.RUnlock()

	if len(qsc.chain) == 0 { // Block always valid for first
		return true
	} else {
		// Check that no other block contains the filename
		for _, b := range qsc.chain {
			if b.Transaction.Name == block.Transaction.Name {
				return false
			}
		}
		lastBlock := qsc.chain[len(qsc.chain)-1] // Get the last block
		lastHash := lastBlock.Hash()
		return lastHash == lastBlock.PrevHash
	}
}

func (qsc *QSC) GenerateBlock(tx TxPublish) BlockPublish {
	qsc.RLock()
	defer qsc.RUnlock()
	var prevHash [32]byte
	if len(qsc.chain) > 0 {
		prevHash = qsc.chain[len(qsc.chain)-1].Hash()
	}
	return BlockPublish{
		PrevHash:    prevHash,
		Transaction: tx,
	}
}


func (qsc *QSC) SetContestant(tlc *TLCMessage) {
	qsc.Lock()
	defer qsc.Unlock()
	qsc.contestant = tlc
}

func GetHighestFitness(messages []*TLCMessage) *TLCMessage {
	var best *TLCMessage
	for _, m := range messages {
		if best == nil || best.Fitness < m.Fitness {
			best = m
		}
	}
	return best
}


func (qsc *QSC) AddSecondConfirmations(messages []*TLCMessage) {
	qsc.Lock()
	defer qsc.Unlock()
	for _, m := range messages {
		qsc.secondConfirmations = append(qsc.secondConfirmations, m.TxBlock)
	}
}

func (qsc *QSC) CheckConsensus(messages []*TLCMessage, majority uint64) {
	qsc.RLock()
	defer qsc.RUnlock()
	confirmedBy := uint64(0)
	contestantBlock := qsc.contestant.TxBlock
	for _, m := range qsc.secondConfirmations {
		if m.Hash() == contestantBlock.Hash() {
			confirmedBy += 1
		}
	}
	// Condition 1: verify that it was confirmed at s+2
	if confirmedBy < majority {
		log.Println("Message was not confirmed by majority at s+2")
		return
	}
	// Condition 2: Verify that no other TLCMessage had a better Fitness at round s
	for _, m := range messages {
		if m.Fitness > qsc.contestant.Fitness {
			log.Println("Found message with better fitness")
			return
		}
	}
	qsc.chain = append(qsc.chain, qsc.contestant.TxBlock)
	var fileStrings []string
	for _, b := range qsc.chain {
		tx := b.Transaction
		fileStrings = append(fileStrings, fmt.Sprintf("%v size %v metahash %v", tx.Name, tx.Size, utils.ToHex(tx.MetafileHash)))
	}

	fmt.Printf("CONSENSUS ON QSC round %v message origin %v ID %v file names %v\n", qsc.currentRound, qsc.contestant.Origin, qsc.contestant.ID, strings.Join(fileStrings, ","))
	qsc.contestant = nil
	qsc.currentRound += 1
	qsc.secondConfirmations = nil

}
