package types

import (
	"encoding/json"
	"github.com/ehoelzl/Peerster/utils"
	"log"
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

type UIMatch struct {
	Origin       string
	MetafileHash string
	FileName     string
}
type FullMatches struct {
	matches map[string]map[string]map[string]struct{} // hash => Filename => origin
	orderedMatches []*UIMatch
	sync.RWMutex
}

func InitFullMatches() *FullMatches {
	return &FullMatches{
		matches: make(map[string]map[string]map[string]struct{}),
	}
}

func (fm *FullMatches) Add(sr []*SearchResult, origin string) {
	fm.Lock()
	defer fm.Unlock()
	for _, s := range sr {
		hash := utils.ToHex(s.MetafileHash)
		if s.ChunkCount > 0 && uint64(len(s.ChunkMap)) == s.ChunkCount { // Full match
			if _, ok := fm.matches[hash]; !ok { // No matches for this hash
				fm.matches[hash] = make(map[string]map[string]struct{})
			}
			if _, ok := fm.matches[hash][s.FileName]; !ok { // No matches for this (hash, Filename)
				fm.matches[hash][s.FileName] = make(map[string]struct{})
			}
			if _, ok := fm.matches[hash][s.FileName][origin]; !ok {
				fm.matches[hash][s.FileName][origin] = struct{}{}
				fm.orderedMatches = append(fm.orderedMatches, &UIMatch{
					Origin:       origin,
					MetafileHash: utils.ToHex(s.MetafileHash),
					FileName:     s.FileName,
				})
			}
		}
	}
}

func (fm *FullMatches) Reset() {
	fm.Lock()
	defer fm.Unlock()
	fm.matches = make(map[string]map[string]map[string]struct{})
	fm.orderedMatches = nil
}

func (fm *FullMatches) AboveThreshold(threshold int) bool {
	fm.RLock()
	defer fm.RUnlock()
	if len(fm.matches) >= threshold {
		return true
	}
	for _, filenames := range fm.matches {
		if len(filenames) >= threshold {
			return true
		}
		for _, origins := range filenames {
			if len(origins) >= threshold {
				return true
			}
		}
	}
	return false
}

func (fm *FullMatches) GetJsonString() []byte {
	fm.RLock()
	defer fm.RUnlock()
	jsonString, err := json.Marshal(fm.orderedMatches)
	if err != nil {
		log.Println("Could not marshall matches")
		return nil
	}
	return jsonString
}
