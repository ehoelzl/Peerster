package types

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"os"
	"strings"
	"sync"
)

var sharedFilesDir = "_SharedFiles"
var downloadedDir = "_Downloads"
var chunkSize int64 = 8192
var hashSize int = 32

type Chunk struct {
	available bool
	index     uint64
	Hash      []byte
}

type File struct {
	Filename string
	Path     string
	Metafile []byte
	Size     int64
	Chunks   map[string]*Chunk
	MetaHash []byte
	sync.RWMutex
}

type waitingChunk struct { // Structure for a requested chunk that has not yet been received
	metaHash string    //Has of the associated metaFile
	ticker   chan bool // Channel for running ticker
	filename string    // Filename
}

type Files struct {
	files         map[string]*File
	waitingChunks map[string]*waitingChunk // From chunk hash to waitingChunk
	sync.RWMutex
}

func (f *File) getChunkData(chunk *Chunk) []byte {
	/*Returns the data associated to the given chunk (Opens the file, reads the chunk and returns it)*/
	f.Lock()
	defer f.Unlock()

	// If chunk not available, cannot send
	if !chunk.available {
		return nil
	}
	exists, file, _ := utils.CheckAndOpenRead(f.Path) // Open the file
	if !exists {
		return nil
	}
	_, err := file.Seek(int64(chunk.index)*chunkSize, 0) // Seek to chunk index
	if err != nil {
		return nil
	}
	dataChunk := make([]byte, chunkSize)
	n, err := file.Read(dataChunk) // Read into buffer
	if err != nil {
		return nil
	}

	if err := file.Close(); err != nil {
		return nil
	}

	return dataChunk[:n]
}

func (f *File) addChunk(chunkHash []byte, chunkData []byte) (*Chunk, bool) {
	/*Adds a given chunk to the file (opens and appends the bytes)*/
	f.Lock()
	defer f.Unlock()

	chunkHashString := utils.ToHex(chunkHash)
	if chunk, ok := f.Chunks[chunkHashString]; ok && !chunk.available { // Check that the chunk was not received already
		exists, file := utils.CheckAndOpenWrite(f.Path) // Open the file
		if !exists {
			return nil, false
		}

		if _, err := file.Write(chunkData); err != nil { // Append the new chunk data
			return nil, false
		}
		if err := file.Close(); err != nil { // Close the file
			return nil, false
		}
		f.Chunks[chunkHashString].available = true // Mark chunk as available
		f.Size += int64(len(chunkData))            // Increase the size

		// Get the next chunk
		nextIndex := chunk.index + 1
		for _, c := range f.Chunks {
			if c.index == nextIndex {
				return c, true
			}
		}
	}
	return nil, false
}

func (f *File) searchResult() *SearchResult {
	/*Transforms a file into SearchResult*/
	var chunkMap []uint64
	for _, c := range f.Chunks {
		if c.available {
			chunkMap = append(chunkMap, c.index)
		}
	}

	chunkCount := len(f.Metafile) / hashSize
	return &SearchResult{
		FileName:     f.Filename,
		MetafileHash: f.MetaHash,
		ChunkMap:     chunkMap,
		ChunkCount:   uint64(chunkCount),
	}
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

	filePath := utils.GetAbsolutePath(sharedFilesDir, filename)
	fileExists, file, fileSize := utils.CheckAndOpenRead(filePath) // Open file and check existence
	if !fileExists {
		return false
	}
	metaFile, chunks := createMetaFile(file)
	metaHash := sha256.Sum256(metaFile) // Hash the metaFile
	hashString := utils.ToHex(metaHash[:])

	if _, ok := fs.files[hashString]; ok { // File is already indexed
		return false
	}

	newFile := &File{
		Filename: filename,
		Path:     filePath,
		Metafile: metaFile,
		Size:     fileSize,
		Chunks:   chunks,
		MetaHash: metaHash[:],
	}

	fs.files[hashString] = newFile
	log.Printf("File %v indexed\n", hashString)
	return true

}

func (fs *Files) IsIndexed(hash []byte) bool {
	/*Checks if a hash is already indexed in the structure*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)
	_, ok := fs.files[hashString]
	return ok
}

func (fs *Files) GetDataChunk(hash []byte) []byte {
	/*Returns any chunk of data given it's Hash*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)

	// First check if it corresponds to a metafile
	if elem, ok := fs.files[hashString]; ok { // Means Hash corresponds to metafile
		return elem.Metafile
	} else { // Hash maybe corresponds to a chunk of file
		for _, file := range fs.files {
			if chunk, ok := file.Chunks[hashString]; ok { // Found the chunk
				return file.getChunkData(chunk)
			}
		}
	}
	return nil // Could not find the hash
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

func (fs *Files) ParseDataReply(dr *DataReply) (*File, *Chunk, bool) {
	/*Parses a reply coming for a requested document. Returns a (*Chunk, bool) to indicate the next chunk*/
	// First check if data is not empty, if empty discard
	fs.Lock()
	defer fs.Unlock()

	var nextChunk *Chunk = nil
	var file *File = nil
	hasNext := false
	hashValueString := utils.ToHex(dr.HashValue) // Hex hash in reply

	if waiting, ok := fs.waitingChunks[hashValueString]; ok { // Was waiting for this hash
		waiting.ticker <- true                          // stop the running ticker
		defer delete(fs.waitingChunks, hashValueString) // Delete the waiting Chunks

		if dr.Data == nil || !utils.CheckDataHash(dr.Data, hashValueString) { // Drop the packet if no Data
			return nil, nil, false
		}

		if waiting.metaHash == hashValueString { // This is a requested metaFile, need to create the file
			nextChunk, hasNext = fs.createDownloadFile(waiting.filename, dr.HashValue, dr.Data)
		} else if file, ok := fs.files[waiting.metaHash]; ok { // This is a requested chunk
			nextChunk, hasNext = file.addChunk(dr.HashValue, dr.Data)
		}
		file = fs.files[waiting.metaHash]

	}

	return file, nextChunk, hasNext
}

func (fs *Files) SearchFiles(keywords []string) ([]*SearchResult, bool) {
	/*Searches for files that contain the given keywords*/
	matches := make(map[string]bool)
	var results []*SearchResult
	for _, k := range keywords {
		if k == "" { // Double check
			continue
		}
		for _, f := range fs.files {
			if _, ok := matches[f.Filename]; !ok && strings.Contains(f.Filename, k) { // Match for this name
				matches[f.Filename] = true
				results = append(results, f.searchResult())
			}
		}
	}
	return results, len(results) > 0
}

func (fs *Files) GetJsonString() []byte {
	fs.RLock()
	defer fs.RUnlock()
	jsonString, err := json.Marshal(fs.files)
	if err != nil {
		log.Println("Could not marshall Files")
		return nil
	}
	return jsonString
}

func (fs *Files) createDownloadFile(filename string, metaHash []byte, metaFile []byte) (*Chunk, bool) {
	/*Creates an empty File struct to start downloading, puts the Path as downloadedDir*/
	hashString := utils.ToHex(metaHash)
	if _, ok := fs.files[hashString]; ok { // Check if we don't have the file already
		return nil, false
	}

	chunks := parseMetaFile(metaFile) // Parse all the chunks
	filePath := utils.GetAbsolutePath(downloadedDir, filename)

	newFile := &File{
		Filename: filename,
		Metafile: metaFile,
		Path:     filePath,
		Size:     0,
		Chunks:   chunks,
		MetaHash: metaHash,
	}
	fs.files[hashString] = newFile
	utils.CreateEmptyFile(filePath)

	for _, chunk := range chunks {
		if chunk.index == 0 {
			return chunk, true
		}
	}
	return nil, false
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
			index:     uint64(i),
			Hash:      chunkHash,
		}
	}
	return chunks
}

func createMetaFile(file *os.File) ([]byte, map[string]*Chunk) {
	/*Reads a file and chunks it into 8KiB chunks, hashes them and concatenates them to an array*/
	chunks := make(map[string]*Chunk)
	buffer := make([]byte, chunkSize)

	var chunkIndex uint64 = 0
	var metaFile []byte

	for n, err := file.Read(buffer); err == nil; {
		content := buffer[:n]                   // Read content
		hash := sha256.Sum256(content)          // Hash 8KiB
		metaFile = append(metaFile, hash[:]...) // Append to metaFile

		chunks[utils.ToHex(hash[:])] = &Chunk{ // Add chunk
			available: true,
			index:     chunkIndex,
			Hash:      hash[:],
		}
		chunkIndex += 1
		n, err = file.Read(buffer)
	}
	return metaFile, chunks
}
