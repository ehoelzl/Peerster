package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
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
