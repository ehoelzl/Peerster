package types

import (
	"crypto/sha256"
	"fmt"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"sync"
)

var sharedFilesDir = "_SharedFiles"
var downloadedDir = "_Downloads"
var chunkSize int64 = 8192
var hashSize int = 32

type Chunk struct {
	available bool
	index     int
	Hash      []byte
}

type waitingChunk struct {
	metaHash string
	ticker   chan bool
	filename string
}
type File struct {
	Filename  string
	Directory string
	Metafile  []byte
	Size      int64
	Chunks    map[string]*Chunk
	sync.RWMutex
}

type Files struct {
	files         map[string]*File
	waitingChunks map[string]*waitingChunk
	sync.RWMutex
}

func (f *File) getChunkData(chunk *Chunk) ([]byte, bool) {
	/*Returns the data associated to the given chunk*/
	f.Lock()
	defer f.Unlock()

	/*Returns the corresponding data chunk for the given file*/
	if !chunk.available {
		return nil, false
	}
	exists, file, _ := utils.CheckAndOpen(f.Directory, f.Filename) // Open the file
	if !exists {
		return nil, false
	}
	_, err := file.Seek(int64(chunk.index)*chunkSize, 0) // Seek to chunk index
	if err != nil {
		return nil, false
	}
	dataChunk := make([]byte, chunkSize)
	_, err = file.Read(dataChunk) // Read into buffer
	if err != nil {
		return nil, false
	}
	return dataChunk, true
}

func parseMetaFile(file []byte) map[string]*Chunk {
	/*Parses the given metafile to get each chunk's hash, and adds the metafile to the file*/
	numChunks := len(file) / hashSize // Get the number of chunks
	chunks := make(map[string]*Chunk)
	for i := 0; i < numChunks; i++ { // Create the chunks with their hashes
		chunkHash := file[i*hashSize : hashSize*(i+1)]
		chunkHashString := utils.ToHex(chunkHash)
		chunks[chunkHashString] = &Chunk{
			available: false,
			index:     i,
			Hash:      chunkHash,
		}
	}
	return chunks
}

func InitFilesStruct() *Files {
	return &Files{
		files:         make(map[string]*File),
		waitingChunks: make(map[string]*waitingChunk),
	}
}

func (fs *Files) IndexNewFile(filename string) bool {
	/*Indexes a new file that should be located under _SharedFiles*/
	fs.Lock()
	defer fs.Unlock()

	fileExists, file, fileSize := utils.CheckAndOpen(sharedFilesDir, filename) // Open file and check existence
	if !fileExists {
		return false
	}

	var metaFile []byte // Create metafile and buffer to read file
	buffer := make([]byte, chunkSize)

	var chunkIndex = 0
	chunks := make(map[string]*Chunk)
	for n, err := file.Read(buffer); err == nil; {
		content := buffer[:n]                   // Read content
		hash := sha256.Sum256(content)          // Hash 8KiB
		metaFile = append(metaFile, hash[:]...) // Append to metaFile

		chunks[fmt.Sprintf("%x", hash)] = &Chunk{ // Add chunk
			available: true,
			index:     chunkIndex,
			Hash:      hash[:],
		}
		chunkIndex += 1
		n, err = file.Read(buffer)
	}

	metaHash := sha256.Sum256(metaFile) // Hash the metaFile
	hashString := fmt.Sprintf("%x", metaHash)

	if _, ok := fs.files[hashString]; ok { // File is already indexed
		return false
	}

	newFile := &File{
		Filename:  filename,
		Directory: sharedFilesDir,
		Metafile:  metaFile,
		Size:      fileSize,
		Chunks:    chunks,
	}

	fs.files[hashString] = newFile
	log.Printf("File %v indexed\n", hashString)
	return true

}

func (fs *Files) IsIndexed(hash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	_, ok := fs.files[hashString]
	return ok
}

func (fs *Files) GetDataChunk(hash []byte) ([]byte, bool) {
	/*Returns any chunk of data given it's Hash*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)

	// First check if it corresponds to a metafile
	if elem, ok := fs.files[hashString]; ok { // Means Hash corresponds to metafile
		return elem.Metafile, elem.Metafile != nil
	} else { // Hash maybe corresponds to a chunk of file
		for _, file := range fs.files {
			for h, chunk := range file.Chunks {
				if hashString == h { // Found the chunk
					log.Println("Found matching hash")
					return file.getChunkData(chunk)
				}
			}
		}
	}
	return nil, false // Could not find the has
}

func (fs *Files) RegisterRequest(chunkHash []byte, metaHash []byte, filename string, callback func()) {
	/*Registers a ticker for the given request, that will periodically call the callback function until stopped*/
	fs.Lock()
	defer fs.Unlock()

	chunkHashString := utils.ToHex(chunkHash)
	metaHashString := utils.ToHex(metaHash)
	ticker := utils.NewTicker(callback, 5) // Ticker for 5 seconds
	waiting := &waitingChunk{
		metaHash: metaHashString,
		ticker:   ticker,
		filename: filename,
	}
	fs.waitingChunks[chunkHashString] = waiting
}

func (fs *Files) ParseDataReply(dr *DataReply) {
	// First check if data is not empty, if empty discard
	if dr.Data == nil {
		return
	}
	dataHash := sha256.Sum256(dr.Data) // Compute the hash of the sent data
	dataHashString := utils.ToHex(dataHash[:])
	hashValueString := utils.ToHex(dr.HashValue)

	if dataHashString != hashValueString { // Drop message if incorrect data
		log.Println("Got mismatched data and Hash")
		return
	}

	fs.Lock()
	defer fs.Unlock()
	if elem, ok := fs.waitingChunks[hashValueString]; ok { // Was waiting for this hash
		elem.ticker <- true                   // stop the running ticker
		if elem.metaHash == hashValueString { // This is a requested metaFile
			fs.createFile(elem.filename, dr.HashValue, dr.Data)
			// TODO: now start requesting chunks
		} else { // This is a requested chunk
			//TODO: when receiving a requested chunk
		}
		delete(fs.waitingChunks, hashValueString)
	}

}

func (fs *Files) createFile(filename string, metaHash []byte, metaFile []byte) {
	/*Creates an empty File struct to start downloading, puts the Directory as downloadedDir*/
	fs.Lock()
	defer fs.Unlock()

	hashString := utils.ToHex(metaHash)
	if _, ok := fs.files[hashString]; ok { // Check if we don't have the file already
		return
	}

	chunks := parseMetaFile(metaFile)
	newFile := &File{
		Filename:  filename,
		Metafile:  metaFile,
		Directory: downloadedDir,
		Size:      0,
		Chunks:    chunks,
	}
	fs.files[hashString] = newFile
}

func (fs *Files) HasMetaFile(hash []byte) bool {
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	if elem, ok := fs.files[hashString]; ok {
		return elem.Metafile != nil
	}
	return false
}

func (fs *Files) GetFileChunks(hash []byte) (map[string]*Chunk, bool) {
	/*Returns all the chunks of the file*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	if elem, ok := fs.files[hashString]; ok {
		return elem.Chunks, true
	}
	return nil, false
}

func (fs *Files) FileHasChunk(fileHash []byte, chunkHash []byte) bool {
	/*Returns True of the given chunk of the given file is available*/
	fs.RLock()
	defer fs.RUnlock()
	fileHashString := utils.ToHex(fileHash)
	chunkHashString := utils.ToHex(chunkHash)
	if file, ok := fs.files[fileHashString]; ok {
		if chunk, ok := file.Chunks[chunkHashString]; ok {
			return chunk.available
		}
	}

	return false
}
