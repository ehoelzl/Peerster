package types

import (
	"fmt"
	"sync"
)

type RoutingTable struct {
	Table map[string]string
	sync.Mutex
}

func NewRoutingTable() *RoutingTable {
	return &RoutingTable{
		Table: make(map[string]string),
	}
}

func (rt *RoutingTable) UpdateRoute(origin, address string, isRouteRumor bool) {
	rt.Lock()
	defer rt.Unlock()
	if elem, ok := rt.Table[origin]; ok && elem == address { // Check if need to update/print
		return
	}
	rt.Table[origin] = address
	if !isRouteRumor {
		fmt.Printf("DSDV %v %v\n", origin, address)
	}
}
