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

func NewTicker(callback func(), seconds time.Duration) {
	go func() {
		ticker := time.NewTicker(seconds * time.Second)
		for {
			select {
			case <-ticker.C:
				callback()
			}
		}
	}()
}

func CheckAndOpen(dir string, filename string) (bool, *os.File, int64) {
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
