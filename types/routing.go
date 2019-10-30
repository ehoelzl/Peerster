package types

import (
	"fmt"
	"net"
	"sync"
)

type RoutingTable struct {
	table map[string]*net.UDPAddr
	sync.RWMutex
}

func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		table: make(map[string]*net.UDPAddr),
	}
}

func (rt *RoutingTable) UpdateRoute(origin string, address *net.UDPAddr, isRouteRumor bool) {
	rt.Lock()
	defer rt.Unlock()
	if elem, ok := rt.table[origin]; ok && elem.String() == address.String() { // Check if need to update/print
		return
	}
	rt.table[origin] = address
	if !isRouteRumor {
		fmt.Printf("DSDV %v %v\n", origin, address.String())
	}
}

func (rt *RoutingTable) GetNextHop(destination string) (*net.UDPAddr, bool) {
	rt.RLock()
	defer rt.RUnlock()
	address, ok := rt.table[destination]
	return address, ok
}

func (rt *RoutingTable) GetAllOrigins() map[string]*net.UDPAddr {
	rt.RLock()
	defer rt.RUnlock()
	return rt.table
}