package types

import (
	"crypto/sha256"
	"encoding/json"
	"github.com/ehoelzl/Peerster/utils"
	"log"
	"os"
	"sort"
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
	Filename       string
	path           string
	metaFile       []byte
	size           int64
	chunks         map[string]*Chunk
	chunkLocations map[uint64]map[string]struct{}
	IsDownloaded   bool
	chunkCount     uint64
	MetaHash       []byte
	sync.RWMutex
}

type requestedChunk struct { // Structure for a requested chunk that has not yet been received
	metaHash string    //Has of the associated metaFile
	ticker   chan bool // Channel for running ticker
	filename string    // Filename
}

type Files struct {
	files           map[string]*File
	requestedChunks map[string]*requestedChunk // From chunk hash to requestedChunk
	sync.RWMutex
}

func (f *File) getUnavailableChunk(index uint64) (*Chunk, bool) {
	for _, c := range f.chunks {
		if c.index == index && !c.available {
			return c, true
		}
	}
	return nil, false
}

func (f *File) getChunkData(chunk *Chunk) []byte {
	/*Returns the data associated to the given chunk (Opens the file, reads the chunk and returns it)*/
	f.Lock()
	defer f.Unlock()

	// If chunk not available, cannot send
	if !chunk.available {
		return nil
	}
	exists, file, _ := utils.CheckAndOpenRead(f.path) // Open the file
	if !exists {
		return nil
	}
	_, err := file.Seek(int64(chunk.index-1)*chunkSize, 0) // Seek to chunk index
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
	if chunk, ok := f.chunks[chunkHashString]; ok && !chunk.available { // Check that the chunk was not received already
		file, exists := utils.CheckAndOpenWrite(f.path) // Open the file
		if !exists {
			return nil, false
		}

		if _, err := file.Seek(int64(chunk.index-1)*chunkSize, 0); err != nil { // Seek to index
			return nil, false
		}
		if _, err := file.Write(chunkData); err != nil { // Write chunk at index
			return nil, false
		}
		f.chunks[chunkHashString].available = true // Mark chunk as available
		f.size += int64(len(chunkData))            // Increase the size

		nextIndex := chunk.index + 1
		nextChunk, hasNextChunk := f.getUnavailableChunk(nextIndex) // Get next chunk

		if !hasNextChunk { // If all chunks have been downloaded, truncate at size
			if err := file.Truncate(f.size); err != nil {
				log.Println("Could not truncate file")
			}
			f.IsDownloaded = true // Mark file as complete
		}
		if err := file.Close(); err != nil { // Close the file
			return nil, false
		}
		return nextChunk, hasNextChunk

	}
	return nil, false
}

func (f *File) updateChunkLocations(chunkMap []uint64, origin string) {
	for _, chunkIndex := range chunkMap {
		if _, ok := f.chunkLocations[chunkIndex]; !ok {
			f.chunkLocations[chunkIndex] = make(map[string]struct{})
		}
		f.chunkLocations[chunkIndex][origin] = struct{}{}

	}
}

func (f *File) searchResult() *SearchResult {
	/*Transforms a file into SearchResult*/
	var chunkMap []uint64
	for _, c := range f.chunks {
		if c.available {
			chunkMap = append(chunkMap, c.index)
		}
	}
	sort.SliceStable(chunkMap, func(i, j int) bool { return chunkMap[i] < chunkMap[j] })

	chunkCount := len(f.metaFile) / hashSize
	return &SearchResult{
		FileName:     f.Filename,
		MetafileHash: f.MetaHash,
		ChunkMap:     chunkMap,
		ChunkCount:   uint64(chunkCount),
	}
}

func InitFilesStruct() *Files {
	return &Files{
		files:           make(map[string]*File),
		requestedChunks: make(map[string]*requestedChunk),
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
		path:     filePath,
		metaFile: metaFile,
		size:     fileSize,
		chunks:   chunks,
		MetaHash: metaHash[:],
	}

	fs.files[hashString] = newFile
	log.Printf("File %v indexed\n", hashString)
	return true

}

func (fs *Files) GetDataChunk(hash []byte) []byte {
	/*Returns any chunk of data given it's Hash*/
	fs.RLock()
	defer fs.RUnlock()
	hashString := utils.ToHex(hash)

	// First check if it corresponds to a metafile
	if elem, ok := fs.files[hashString]; ok { // Means Hash corresponds to metafile
		return elem.metaFile
	} else { // Hash maybe corresponds to a chunk of file
		for _, file := range fs.files {
			if chunk, ok := file.chunks[hashString]; ok { // Found the chunk
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
	requested := &requestedChunk{
		metaHash: metaHashString,
		ticker:   ticker,
		filename: filename,
	}
	fs.requestedChunks[chunkHashString] = requested
}

func (fs *Files) ParseDataReply(dr *DataReply) (*File, *Chunk, bool, []string) {
	/*Parses a reply coming for a requested document. Returns a (*Chunk, bool) to indicate the next chunk*/
	// First check if data is not empty, if empty discard
	fs.Lock()
	defer fs.Unlock()

	var nextChunk *Chunk = nil
	var file *File = nil
	var locations []string

	hasNext := false
	hashValueString := utils.ToHex(dr.HashValue) // Hex hash in reply

	if requested, ok := fs.requestedChunks[hashValueString]; ok { // Was requested for this hash
		requested.ticker <- true                          // stop the running ticker
		defer delete(fs.requestedChunks, hashValueString) // Delete the requested chunks

		isMetaFile := requested.metaHash == hashValueString // Means that the reply is a MetaFile

		if dr.Data == nil && !isMetaFile { // Empty data => the origin does not have the requested chunk, delete it
			file := fs.files[requested.metaHash]
			chunk := file.chunks[hashValueString]
			if _, ok := file.chunkLocations[chunk.index][dr.Origin]; ok { // The origin is in the list of Locations for this chunk
				delete(file.chunkLocations[chunk.index], dr.Origin)
			}
			return file, chunk, true, utils.MapToSlice(file.chunkLocations[chunk.index])
		}

		if !utils.CheckDataHash(dr.Data, hashValueString) { // Drop the packet if faulty data
			return nil, nil, false, nil
		}

		if isMetaFile {
			nextChunk, hasNext = fs.createDownloadFile(requested.filename, dr.HashValue, dr.Data, dr.Origin)
		} else if file, ok := fs.files[requested.metaHash]; ok { // This is a requested chunk
			nextChunk, hasNext = file.addChunk(dr.HashValue, dr.Data)
		}
		file = fs.files[requested.metaHash]

		if nextChunk != nil && hasNext {
			locations = utils.MapToSlice(file.chunkLocations[nextChunk.index])
		}

	}

	return file, nextChunk, hasNext, locations
}

func (fs *Files) GetMetaFileLocations(request []byte) ([]string, bool) {
	/*Returns all the possible locations for the MetaFile (any Node with chunk number 1)*/
	fs.RLock()
	defer fs.RUnlock()
	hash := utils.ToHex(request)
	if _, ok := fs.files[hash]; !ok {
		return nil, false
	}
	locations := utils.MapToSlice(fs.files[hash].chunkLocations[1])
	return locations, locations != nil
}

func (fs *Files) SearchFiles(keywords []string) ([]*SearchResult, bool) {
	/*Searches for files that contain the given keywords*/
	matches := make(map[string]bool)
	var results []*SearchResult
	for _, k := range keywords {
		if len(k) == 0 { // Double check
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

func (fs *Files) AddSearchResults(sr []*SearchResult, origin string) {
	if sr == nil || len(sr) == 0 {
		return
	}
	fs.Lock()
	defer fs.Unlock()

	for _, res := range sr {
		metaFileHash := utils.ToHex(res.MetafileHash)
		if f, ok := fs.files[metaFileHash]; ok { // File already exists
			f.updateChunkLocations(res.ChunkMap, origin) // Update the locations of each chunk
		} else {
			locations := make(map[uint64]map[string]struct{})
			for _, i := range res.ChunkMap {
				locations[i] = map[string]struct{}{origin: struct{}{},}
			}

			file := &File{
				chunkLocations: locations,
				chunkCount:     res.ChunkCount,
				MetaHash:       res.MetafileHash,
			}
			fs.files[metaFileHash] = file
		}
	}

}

func (fs *Files) createDownloadFile(filename string, metaHash []byte, metaFile []byte, origin string) (*Chunk, bool) {
	/*Creates an empty File struct to start downloading, puts the path as downloadedDir*/
	hashString := utils.ToHex(metaHash)
	file, filePresent := fs.files[hashString]
	if filePresent && file.metaFile != nil { // Check if we don't have the file already
		return nil, false
	}

	chunks := parseMetaFile(metaFile) // Parse all the chunks
	filePath := utils.GetAbsolutePath(downloadedDir, filename)

	if filePresent { // Means file is indexed, and was found by Search
		file.metaFile = metaFile
		file.path = filePath
		file.chunks = chunks
		file.Filename = filename
	} else {
		chunkLocations := createChunkLocations(chunks, origin)

		newFile := &File{
			Filename:       filename,
			metaFile:       metaFile,
			path:           filePath,
			size:           0,
			chunks:         chunks,
			chunkCount:     uint64(len(chunks)),
			chunkLocations: chunkLocations,
			MetaHash:       metaHash,
		}
		fs.files[hashString] = newFile
		file = newFile
	}

	utils.CreateEmptyFile(filePath, int64(len(chunks))*chunkSize)

	nextChunk, hasNext := file.getUnavailableChunk(1) // Get chunk at index 1
	return nextChunk, hasNext
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
			index:     uint64(i + 1),
			Hash:      chunkHash,
		}
	}
	return chunks
}

func createChunkLocations(chunks map[string]*Chunk, origin string) map[uint64]map[string]struct{} {
	locations := make(map[uint64]map[string]struct{})
	for _, c := range chunks {
		locations[c.index] = map[string]struct{}{origin: struct{}{},}
	}
	return locations
}

func createMetaFile(file *os.File) ([]byte, map[string]*Chunk) {
	/*Reads a file and chunks it into 8KiB chunks, hashes them and concatenates them to an array*/
	chunks := make(map[string]*Chunk)
	buffer := make([]byte, chunkSize)

	var chunkIndex uint64 = 1
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
