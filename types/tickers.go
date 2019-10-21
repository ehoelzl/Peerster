package types

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Tickers struct {
	tickers map[string]*time.Ticker
	Lock    sync.Mutex
}

func NewPeerTimer(address *net.UDPAddr, callback func(), seconds time.Duration) *time.Ticker {
	ticker := time.NewTicker(seconds * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Printf("TIMEMOUT for %v\n", address.String())
				if callback != nil {
					go callback()
				}
				ticker.Stop()
			}
		}
	}()
	return ticker
}

func NewTickers() *Tickers {
	return &Tickers{
		tickers: make(map[string]*time.Ticker),
		Lock:    sync.Mutex{},
	}
}

func (ticks *Tickers) DeleteTicker(address *net.UDPAddr) bool {
	ticks.Lock.Lock()
	defer ticks.Lock.Unlock()
	deleted := false
	if elem, ok := ticks.tickers[address.String()]; ok {
		elem.Stop()
		delete(ticks.tickers, address.String())
		deleted = true
	}
	return deleted
}

func (ticks *Tickers) AddTicker(address *net.UDPAddr, callback func(), seconds time.Duration) {
	ticks.Lock.Lock()
	defer ticks.Lock.Unlock()
	if elem, ok := ticks.tickers[address.String()]; ok {
		elem.Stop()
		delete(ticks.tickers, address.String())
	}
	ticks.tickers[address.String()] = NewPeerTimer(address, callback, seconds)
}




