package types

import (
	"crypto/sha256"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"net"
	"sort"
	"sync"
)

type Jam struct {
	Lock          sync.RWMutex
	Jammers       map[string]string
	ACKS          map[string]bool
	Order         []string
	NumberOfNodes uint64
	PROPOSED      bool
	MASTER        string
}

type DiscussionMessage struct {
	From    *string
	Players *map[string]string
	Order   *[]string
}

func InitJamStruct(numNodes uint64, gossiperName string) *Jam {
	js := Jam{
		Jammers:       make(map[string]string),
		NumberOfNodes: numNodes,
		PROPOSED:      false,
		ACKS:          make(map[string]bool),
		Order:         make([]string, 0),
		MASTER:        "",
	}
	//Save our local name
	hash := sha256.Sum256([]byte(gossiperName))
	uid := utils.ToHex(hash[:])
	js.Jammers[gossiperName] = uid
	return &js
}

func InitPlay(g *Gossiper) {
	//First we propagate our name
	dm := DiscussionMessage{
		Players: &g.Jam.Jammers,
	}
	g.Broadcast(&GossipPacket{DiscussionMessage: &dm}, nil) // Broadcast to all known nodes
}

func (g *Gossiper) HandleDiscussionMessage(from *net.UDPAddr, disc *DiscussionMessage) {
	if disc == nil {
		return
	}

	// If we have all Jammers
	if len(g.Jam.Jammers) == int(g.Jam.NumberOfNodes) {
		// If havent proposed, propose order
		if !g.Jam.PROPOSED {
			g.Jam.PROPOSED = true
			go g.ProposeConsensus()
		}

		// Master sent order
		if disc.Order != nil {
			g.Jam.Order = *disc.Order // Set order
			ack := &DiscussionMessage{From: &g.Name} // Prepare ACK message
			log.Println("DECIDED", g.Jam.Order, "MASTER", g.Jam.Order[0], "My order", g.Name, g.GetMyOrder())
			g.SendPacket(&GossipPacket{DiscussionMessage: ack}, from) // Ack to Sending node

		} else if disc.From != nil && g.Name == g.Jam.MASTER { // I am the master, and this is an ack
			if !g.Jam.ACKS[*disc.From] {
				g.Jam.ACKS[*disc.From] = true
			}

			//Received all acks
			if len(g.Jam.ACKS) == int(g.Jam.NumberOfNodes) {
				log.Println("RECEIVED ALL ACKS, ready to play using ptp")
				go PlayPTP(g)
				//TODO : reset all fields to prepare for next round
			}
		}

	} else {
		//Merge if we don't have all players yet
		for k, v := range *disc.Players {
			if _, ok := g.Jam.Jammers[k]; ok {
				g.Jam.Jammers[k] = v
			}
		}

		//Broadcast our new table
		dm := DiscussionMessage{
			Players: &g.Jam.Jammers,
		}
		g.Broadcast(&GossipPacket{DiscussionMessage: &dm}, nil)
	}
}

func (g *Gossiper) ProposeConsensus() {
	//Use the League of Entropy to elect the master clock
	//In the normal PTP protocol normally the leader is the one with the most accurate clock. Since we will have the same hardware for each Jamster, we take a random leader
	r, err := getRandomness()
	if err != nil {
		log.Println("Error getting the randomness")
	}

	values := make([]int, 0)
	for _, v := range g.Jam.Jammers {
		values = append(values, int(HashToNumber(v)%HashToNumber(r.Point)))
	}

	sort.Ints(values)

	order := make([]string, 0)
	for _, v := range values {
		for name, hash := range g.Jam.Jammers {
			if v == int(HashToNumber(hash)%HashToNumber(r.Point)) {
				order = append(order, name)
			}
		}
	}

	//If I am the master
	if order[0] == g.Name {
		g.Jam.MASTER = g.Name // Set Jam master
		g.PTP.MASTER = g.Name // Set PTP master
		g.Jam.Order = order  // Set Jam Order
		g.Jam.ACKS[g.Name] = true
		log.Println("AT", g.Name, "DECIDED:", order)
		proposition := &DiscussionMessage{Order: &order} // Order proposition
		go g.Broadcast(&GossipPacket{DiscussionMessage: proposition}, nil)
	} else {
		log.Println("WAITING FOR ORDERING")
	}

}

func (g *Gossiper) GetMyOrder() int {
	//TODO : improve or add error
	if g.Jam.Order == nil {
		log.Println("Jam order not set")
		return int(g.Jam.NumberOfNodes) + 1
	}
	for k, v := range g.Jam.Order {
		if v == g.Name {
			return k
		}
	}
	return int(g.Jam.NumberOfNodes) + 1
}
