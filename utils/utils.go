package utils

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
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

func CheckAndOpenRead(dir string, filename string) (bool, *os.File, int64) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, nil, 0
	}

	filePath := filepath.Join(cwd, dir, filename) // Get filePath
	f, err := os.Open(filePath)                   // Open file

	if os.IsNotExist(err) { // Check existence
		return false, nil, 0
	}
	info, err := f.Stat()

	if err != nil {
		return false, nil, 0
	}

	return !info.IsDir(), f, info.Size()
}

func CheckAndOpenWrite(dir string, filename string) (bool, *os.File, int64) {
	cwd, err := os.Getwd()
	if err != nil {
		return false, nil, 0
	}

	filePath := filepath.Join(cwd, dir, filename) // Get filePath
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend)                   // Open file

	if os.IsNotExist(err) { // Check existence
		return false, nil, 0
	}
	info, err := f.Stat()

	if err != nil {
		return false, nil, 0
	}

	return !info.IsDir(), f, info.Size()
}

func CreateEmptyFile(dir string, filename string) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	filePath := filepath.Join(cwd, dir, filename)
	if _, err := os.Create(filePath); err != nil{
		return
	}
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
