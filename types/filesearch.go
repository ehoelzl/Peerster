package types

import (
	"github.com/ehoelzl/Peerster/utils"
	"sync"
	"time"
)

var duplicateThreshold int64 = 500

type SearchRequests struct {
	requests map[int64]*SearchRequest
	sync.RWMutex
}

func InitSearchRequests() *SearchRequests {
	sr := &SearchRequests{
		requests: make(map[int64]*SearchRequest),
	}
	return sr
}

func (sr *SearchRequests) AddRequest(request *SearchRequest) bool {
	/*Adds the given request to the Map, and deletes requests that are older than 500ms*/
	sr.Lock()
	defer sr.Unlock()

	now := time.Now().UnixNano() / int64(time.Millisecond)
	var todelete []int64
	duplicated := false

	for t, req := range sr.requests {
		if now-t < duplicateThreshold {
			if req.IsDuplicate(request) {
				duplicated = true
			}
		} else {
			todelete = append(todelete, t)
		}
	}

	for _, t := range todelete {
		delete(sr.requests, t)
	}
	if !duplicated {
		sr.requests[now] = request
	}
	return !duplicated
}

type FullMatches struct {
	matches map[string]map[string]struct{}
	sync.RWMutex
}

func InitFullMatches() *FullMatches {
	return &FullMatches{
		matches: make(map[string]map[string]struct{}),
	}
}

func (fm *FullMatches) Add(sr []*SearchResult, origin string) {
	fm.Lock()
	defer fm.Unlock()
	for _, s := range sr {
		hash := utils.ToHex(s.MetafileHash)
		if uint64(len(s.ChunkMap)) == s.ChunkCount { // Full match
			if _, ok := fm.matches[hash]; ok {
				fm.matches[hash][origin] = struct{}{}
			} else {
				fm.matches[hash] = map[string]struct{}{origin: struct{}{}}
			}
		}
	}
}

func (fm *FullMatches) Reset() {
	fm.Lock()
	defer fm.Unlock()
	fm.matches = make(map[string]map[string]struct{})
}

func (fm *FullMatches) AboveThreshold(threshold int) bool {
	fm.RLock()
	defer fm.RUnlock()
	if len(fm.matches) >= threshold {
		return true
	}

	for _, m := range fm.matches {
		if len(m) >= threshold {
			return true
		}
	}
	return false
}
