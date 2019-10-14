package types

import (
	"log"
	"net"
	"sync"
	"time"
)

type Tickers struct {
	tickers map[string]chan bool
	Lock    sync.Mutex
}

func NewPeerTimer(address *net.UDPAddr, callback func(), seconds time.Duration) chan bool {
	receivedAck := make(chan bool) // Create chanel for bool
	go func() {
		ticker := time.NewTicker(seconds * time.Second)
		for {
			select {
			case <-receivedAck:
				ticker.Stop()
				return
			case <-ticker.C:
				log.Printf("TIMEMOUT for %v\n", address.String())
				if callback != nil {
					callback()
				} else {
					ticker.Stop()
				}
				return
			}
		}
	}()
	return receivedAck
}

func NewTickers() *Tickers {
	return &Tickers{
		tickers: make(map[string]chan bool),
		Lock:    sync.Mutex{},
	}
}

func (ticks *Tickers) DeleteTicker(address *net.UDPAddr) bool {
	ticks.Lock.Lock()
	ticks.Lock.Unlock()
	deleted := false
	if elem, ok := ticks.tickers[address.String()]; ok {
		close(elem)
		delete(ticks.tickers, address.String())
		deleted = true
	}
	return deleted
}

func (ticks *Tickers) AddTicker(address *net.UDPAddr, callback func(), seconds time.Duration) {
	ticks.Lock.Lock()
	defer ticks.Lock.Unlock()
	ticks.tickers[address.String()] = NewPeerTimer(address, callback, seconds)
}




