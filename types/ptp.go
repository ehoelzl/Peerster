package types

import (
	"crypto/sha256"
	"github.com/ehoelzl/Peerster/utils"
	"hash/fnv"
	"log"
	"sync"
	"time"
)




type PTP struct {
	Peers         map[string]string
	NumberOfNodes uint64
	Lock          sync.RWMutex
	MASTER        string
	T1            time.Time
	T2            time.Time
	T3 			  time.Time
	T4 			  time.Time
}


type PTPMessage struct {
	Name *string
	T1 *time.Time
	T4 *time.Time

}





func InitPTPStruct(numNodes uint64) *PTP {
	ptp := PTP {
		Peers:         make(map[string]string),
		NumberOfNodes: numNodes,
		Lock:          sync.RWMutex{},
	}
	return &ptp
}



//TODO
//Pass the gossiper as an argument instead of using it as an handler?
func (g *Gossiper) InitPTP(numNodes *uint64){
	//We wait for some messages to exchange so that the ptp is initiated with all peers
	//We first send our name, which will help us decide each leader at each round
	time.Sleep(2*time.Second)
	gp := GossipPacket{PTPMessage:&PTPMessage{Name:&g.Name}}

	hash := sha256.Sum256([]byte(g.Name))
	uid := utils.ToHex(hash[:])

	g.PTP.Lock.Lock()
	g.PTP.Peers[g.Name] = uid
	g.PTP.Lock.Unlock()
	g.Broadcast(&gp, nil)

	//TODO : is this necessary? should we give up at some point?
	//We wait here
	for len(g.PTP.Peers) != int(g.PTP.NumberOfNodes) {}

	r, err := getRandomness()
	if err != nil {
		log.Println("Error getting the randomness")
	}


	//In the normal ptp protocol normally the leader is the one with the most accurate clock. Since we will have the same hardware for each Jamster, we take a random leader
	//We have two ways to implement the rounds :  either round by round, process1,2,3,1,2,3 etc.. or use the randomness from the league of entropy to decide the leader at each round
	var max uint32
	var MASTER string
	max = 0

	for k, v := range g.PTP.Peers {
		mod := HashToNumber(v)%uint32(r.Round)
		if  mod > max {
			max = mod
			MASTER = k
		}
	}

	g.PTP.MASTER = MASTER

	if MASTER == g.Name {
		log.Println("INITIATED SYNC at", g.Name)
		g.SendSYNC()
	}

}


func (g *Gossiper) SendSYNC() {
	record := time.Now()
	g.PTP.T1 = record
	p := PTPMessage {
		Name: &g.Name,
		T1:   &g.PTP.T1,
	}
	g.Broadcast(&GossipPacket{PTPMessage:&p}, nil)
}



func HashToNumber(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
