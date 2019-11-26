package utils

import (
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

func CheckAndOpenWrite(filePath string) (bool, *os.File) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, os.ModeAppend) // Open file

	if os.IsNotExist(err) { // Check existence
		return false, nil
	}
	info, err := f.Stat()

	if err != nil {
		return false, nil
	}

	return !info.IsDir(), f
}

func CreateEmptyFile(filePath string) {
	if _, err := os.Create(filePath); err != nil {
		return
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

