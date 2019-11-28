package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

func CheckError(err error, msg string) {
	if err != nil {
		log.Println(msg)
	}
}

func CheckFatalError(error error, msg string) {
	if error != nil {
		panic(msg)
	}
}

func CoinFlip() bool {
	rand.Seed(time.Now().UnixNano())
	return rand.Int()%2 == 0
}

func NewTicker(callback func(), seconds time.Duration) chan bool {
	/*Ticker used for AntiEntropy and Route rumors*/
	stop := make(chan bool)
	ticker := time.NewTicker(seconds * time.Second)

	go func(tick *time.Ticker, stopChan chan bool) {
		for {
			select {
			case <-stopChan:
				ticker.Stop()
				return
			case <-ticker.C:
				callback()
			}
		}
	}(ticker, stop)
	return stop
}

func ToHex(hash []byte) string {
	return fmt.Sprintf("%x", hash)
}
func ToBytes(hash string) []byte {
	b, err := hex.DecodeString(hash)
	if err != nil {
		return nil
	}
	return b
}

func CheckDataHash(data []byte, hashString string) bool {
	/*Checks that SHA256 of the given []byte matches the given hash in hex format*/
	dataHash := sha256.Sum256(data)
	dataHashString := ToHex(dataHash[:])
	return dataHashString == hashString
}

func NewTimoutTicker(callback func(), seconds time.Duration) chan bool {
	/*Creates a ticker that ticks only once and calls the callback function*/
	ticker := time.NewTicker(seconds * time.Second)
	stop := make(chan bool)
	go func(tick *time.Ticker, stopChan chan bool) {
		for {
			select {
			case <-stopChan:
				tick.Stop()
				return
			case <-tick.C:
				tick.Stop()
				callback()
				return
			}
		}
	}(ticker, stop)
	return stop
}

func stringSliceToMap(slice []string) map[string]struct{} {
	newMap := make(map[string]struct{})
	for _, k := range slice {
		if k != "" {
			newMap[k] = struct{}{}
		}
	}
	return newMap
}

func StringSliceEqual(slice1 []string, slice2 []string) bool {

	map1 := stringSliceToMap(slice1)
	map2 := stringSliceToMap(slice2)

	if len(map1) != len(map2) {
		return false
	}
	for s, _ := range map1 {
		if _, ok := map2[s]; !ok {
			return false
		}
	}

	for s, _ := range map2 {
		if _, ok := map1[s]; !ok {
			return false
		}
	}
	return true
}

func ParseKeyWords(words string) []string {
	keywords := strings.Split(words, ",")
	var filteredKeywords []string
	for _, k := range keywords {
		if len(k) > 0 {
			filteredKeywords = append(filteredKeywords, k)
		}
	}
	return filteredKeywords
}

func MapToSlice(m map[string]struct{}) []string {
	var ret []string
	for k, _ := range m {
		ret = append(ret, k)
	}
	return ret
}

func ChooseRandom(locations []string) (string, bool) {
	n := len(locations)
	if n <= 0 {
		return "", false
	}
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(n)
	return locations[index], true
}