package utils

import (
	"log"
	"os"
	"path/filepath"
)

func CheckAndOpenRead(filePath string) (bool, *os.File, int64) {
	f, err := os.Open(filePath) // Open file

	if os.IsNotExist(err) { // Check existence
		return false, nil, 0
	}
	info, err := f.Stat()

	if err != nil {
		return false, nil, 0
	}

	return !info.IsDir(), f, info.Size()
}

func CheckAndOpenWrite(filePath string) (*os.File, bool) {
	f, err := os.OpenFile(filePath, os.O_WRONLY, os.ModeAppend) // Open file

	if os.IsNotExist(err) { // Check existence
		return nil, false
	}
	info, err := f.Stat()

	if err != nil {
		return nil, false
	}
	return f, !info.IsDir()
}

func CreateEmptyFile(filePath string, size int64) {
	if f, err := os.Create(filePath); err == nil {
		if err = f.Truncate(size); err != nil {
			log.Printf("Could not create file %v of size %v\n", filePath, size)
		}
		if err := f.Close(); err != nil {
			log.Printf("Could not create file %v of size %v\n", filePath, size)
		}
	}
}

func GetAbsolutePath(directory string, filename string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	filePath := filepath.Join(cwd, directory, filename)
	return filePath
}

