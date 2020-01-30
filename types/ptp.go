package types

import (
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
	MASTER        string
	SECOND		  string
	Randomness	  uint32
	T1            time.Time
	T2            time.Time
	T3 			  time.Time
	T4 			  time.Time
	CurrentDelta  time.Duration
	sync.RWMutex
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
	g.SendSYNC()
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
	//AT SLAVE : Record time at which we received the message from the master
	if ptp.T1 != nil && ptp.Name != nil {
		g.PTP.T1 = *ptp.T1
		T2 := time.Now()
		g.PTP.T2 = T2
		g.SendPacket(&GossipPacket{PTPMessage: &PTPMessage{}}, from)
		g.PTP.T3 = time.Now()
	} else
	//AT MASTER : we received an empty ptp message from a slave. He is simply asking us when we received this message
	if ptp.T1 == nil && ptp.Name == nil && ptp.T4 == nil{
		//Check that we are the master of this round
		if g.Name != g.PTP.MASTER {return}
		now := time.Now()
		t4Packet := PTPMessage{T4: &now}

		g.PTP.Lock()
		g.PTP.CurrentDelta = 0*time.Microsecond
		g.PTP.Unlock()

		g.SendPacket(&GossipPacket{PTPMessage: &t4Packet}, from)

		time.Sleep(g.sleepDuration())

		log.Println(g.Name, "STARTED PLAYING DRUMS")
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
			g.PTP.Lock()
			g.PTP.CurrentDelta = 1*time.Microsecond
			g.PTP.Unlock()
		} else {
			log.Println("CLOCK DELAY at", g.Name, offset)
			g.PTP.Lock()
			g.PTP.CurrentDelta = offset
			g.PTP.Unlock()
		}
		time.Sleep(g.sleepDuration())

		if g.GetMyOrder() == 1 {
			log.Println(g.Name, "STARTED PLAYING BASS")
			PlayBass()
		} else {
			log.Println(g.Name, "STARTED PLAYING SYNTH")
			PlaySynth()
		}
	}

	g.PTP.Lock()
	//We reset the values for the next round
	g.PTP.T1 = time.Time{}
	g.PTP.T2 = time.Time{}
	g.PTP.T3 = time.Time{}
	g.PTP.T4 = time.Time{}
	g.PTP.Unlock()
}

func (g *Gossiper) sleepDuration() time.Duration {
	//playTime := g.MasterTime().Truncate(5 * time.Second).Add(5 * time.Second)
	playTime := g.MasterTime().Add(5 * time.Second)
	log.Printf("Current master time %v\n", g.MasterTime())
	duration := playTime.Sub(g.MasterTime()) // g.MasterTime().Sub(playTime)
	log.Printf("Start playing at %v, wait %v \n", playTime, duration)
	return duration
}

func HashToNumber(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
