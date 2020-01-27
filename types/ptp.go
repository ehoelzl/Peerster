package types

import (
	"crypto/sha256"
	"github.com/ehoelzl/Peerster/utils"
	"hash/fnv"
	"log"
	"net"
	"sync"
	"time"
)


const Ticker_PTP  = 14

type PTP struct {
	Peers         map[string]string
	NumberOfNodes uint64
	Lock          sync.RWMutex
	MASTER        string
	SECOND		  string
	Randomness	  uint32
	T1            time.Time
	T2            time.Time
	T3 			  time.Time
	T4 			  time.Time
	CurrentDelta  time.Duration
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
		CurrentDelta:  0,
	}
	return &ptp
}

//Restart the PTP at Ticker_PTP intervals
func PlayPTP(g *Gossiper){
	ticker := time.NewTicker(Ticker_PTP * time.Second)
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <- ticker.C:
				InitPTP(g)
			case <- quit:
				ticker.Stop()
				return
			}
		}
	}()
}

//Gives the time at the master clock
func (g *Gossiper) MasterTime() time.Time {
	return time.Now().Add(g.PTP.CurrentDelta)
}


func InitPTP(g *Gossiper){
	//We wait for some messages to exchange so that the ptp is initiated with all peers
	//We first send our name, which will help us decide each leader at each round
	time.Sleep(2*time.Second)
	gp := GossipPacket{PTPMessage:&PTPMessage{Name:&g.Name}}
	g.Broadcast(&gp, nil)

	//Save our local name
	hash := sha256.Sum256([]byte(g.Name))
	uid := utils.ToHex(hash[:])
	g.PTP.Lock.Lock()
	g.PTP.Peers[g.Name] = uid
	g.PTP.Lock.Unlock()

	//TODO : check for correctness here
	//Wait for the name of the other peers
	time.Sleep(3*time.Second)
	log.Println("WAITED LONG ENOUGH, ELECTING..")

	//Use the League of Entropy to elect the master clock
	//In the normal PTP protocol normally the leader is the one with the most accurate clock. Since we will have the same hardware for each Jamster, we take a random leader
	r, err := getRandomness()
	if err != nil {
		log.Println("Error getting the randomness")
	}

	//TODO : be sure that we have uniform consensus on the leader
	//We have two ways to implement the rounds :  either round by round, process1,2,3,1,2,3 etc.. or use the randomness from the league of entropy to decide the leader at each round
	var max uint32 = 0
	var MASTER string
	g.PTP.Randomness = HashToNumber(r.Point)
	log.Println("RANDOMNESS", HashToNumber(r.Point))
	for k, v := range g.PTP.Peers {
		mod := HashToNumber(v)%HashToNumber(r.Point)
		if  mod > max {
			max = mod
			MASTER = k
		}
	}

	for k, v := range g.PTP.Peers {
		mod := HashToNumber(v)%HashToNumber(r.Point)
		if k != MASTER && g.Name != k {
			if mod < HashToNumber(g.PTP.Peers[g.Name]) % HashToNumber(r.Point) {
				g.PTP.SECOND = g.Name
			}
		}
	}
	
	//The master is saved, and starts the sync process
	g.PTP.MASTER = MASTER
	if MASTER == g.Name {
		log.Println("INITIATED SYNC at", g.Name)
		g.SendSYNC()
	}
}

//Initiation at Master
func (g *Gossiper) SendSYNC() {
	g.PTP.T1 = time.Now()
	p := PTPMessage {
		Name: &g.Name,
		T1:   &g.PTP.T1,
	}
	g.Broadcast(&GossipPacket{PTPMessage:&p}, nil)
}

//Generic function to handle ptp packets both at Master and Slave
func (g *Gossiper) HandlePTP(from  *net.UDPAddr, ptp *PTPMessage) {
	//We received the name of another Jamster!
	if ptp.Name != nil && ptp.T1 == nil {
		//Check if we haven't recorded this peer already
		if g.PTP.Peers[*ptp.Name] != "" {return}
		//Lazy Broadcast
		g.Broadcast(&GossipPacket{PTPMessage: ptp}, nil)
		//Else we register a new ptp peer
		hash := sha256.Sum256([]byte(*ptp.Name))
		uid := utils.ToHex(hash[:])
		g.PTP.Lock.Lock()
		g.PTP.Peers[*ptp.Name] = uid
		g.PTP.Lock.Unlock()
	} else
	//AT SLAVE : Record time at which we received the message from the master
	if ptp.T1 != nil && ptp.Name != nil {
		g.PTP.T1 = *ptp.T1

		T2 := time.Now()
		g.PTP.T2 = T2

		nilP := PTPMessage{}
		g.SendPacket(&GossipPacket{PTPMessage: &nilP}, from)

		g.PTP.T3 = time.Now()

	} else
	//AT MASTER : we received an empty ptp message from a slave. He is simply asking us when we received this message
	if ptp.T1 == nil && ptp.Name == nil && ptp.T4 == nil{
		//Check that we are the master of this round
		if g.Name != g.PTP.MASTER {return}
		now := time.Now()
		t4Packet := PTPMessage{T4: &now}
		go g.SendPacket(&GossipPacket{PTPMessage: &t4Packet}, from)

		time.Sleep(1*time.Second)

		log.Println(g.Name, "STARTED PLAYING DRUMS")

		// THIS IS RUN TWICE, ONCE FOR EACH SLAVE
		// TODO FIX THIS
		PlayDrums()
	} else if
	//AT SLAVE sync is complete, we can compute the offset
	ptp.T4 != nil {
		//offset = (T2 - T1 - T4 + T3) ÷ 2
		g.PTP.T4 = *ptp.T4
		t2mt1 := g.PTP.T2.Sub(g.PTP.T1)
		opt3 := g.PTP.T3.Add(t2mt1)
		offset := opt3.Sub(g.PTP.T4)/2

		//Avoid time underflow
		if offset < 1*time.Microsecond {
			log.Println("CLOCK DELAY at", g.Name, "<1µs")
			g.PTP.Lock.Lock()
			g.PTP.CurrentDelta = 1*time.Microsecond
			g.PTP.Lock.Unlock()
		} else {
			log.Println("CLOCK DELAY at", g.Name, offset)
		}

		time.Sleep(1*time.Second - offset)

		if g.PTP.SECOND == g.Name {
			log.Println(g.Name, "STARTED PLAYING BASS")
			PlayBass()
		} else {
			log.Println(g.Name, "STARTED PLAYING SYNTH")
			PlaySynth()
		}

		//We reset the values for the next round
		g.PTP.T1 = time.Time{}
		g.PTP.T2 = time.Time{}
		g.PTP.T3 = time.Time{}
		g.PTP.T4 = time.Time{}
	}
}

func HashToNumber(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}












/*
	for len(g.PTP.Peers) != int(g.PTP.NumberOfNodes) {
		duration, err := time.ParseDuration("0.1s")
		if err != nil {
			log.Println("Error with timer")
		}
		ticker := time.NewTicker(duration)
		_ = ticker.C
	}
*/
