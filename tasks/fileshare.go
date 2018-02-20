package tasks

import (
	"swarmd/node"
	"swarmd/packets"
	"path/filepath"
	"os/user"
	"os"
	"crypto/md5"
	"io"
	"fmt"
	"strings"
	"math"
)

func GetBasePath() string {
	usr, _ := user.Current()
	basePath := filepath.Join(usr.HomeDir, fmt.Sprintf("%s/.swarmd/", os.Getenv("SWARMNODE")))

	// Make the directory if it doesn't exist
	os.MkdirAll(basePath, 0700)

	return basePath
}

func GetSharePath() string {
	sharePath := filepath.Join(GetBasePath(), "share/")

	// Make the share directory if it doesn't exist
	os.MkdirAll(sharePath, 0700)

	return sharePath

}

func GetPartsPath(filehash string) string {
	partsPath := filepath.Join(GetBasePath(), "parts/", filehash)

	// Make the directory if it doesn't exist
	os.MkdirAll(partsPath, 0700)

	return partsPath
}

func GetFileManifest() packets.FileManifest {
	sharePath := GetSharePath()
	files := make(map[[16]uint8]packets.FileDigest)
	// Build a function that will parse the relevant info from each file
	walkFunc := func(path string, info os.FileInfo, err error) error {
		// Check that file info could be gathered
		if err != nil {
			return nil
		}
		// Open the file for hashing
		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()
		// Generate the checksum
		hash := md5.New()
		if _, err := io.Copy(hash, file); err != nil {
			return nil
		}
		// Determine the relative file path
		relPath, err := filepath.Rel(sharePath, path)
		if err != nil {
			return nil
		}
		// Pack the FileDigest struct
		f := packets.FileDigest{
			RelativeFilePath: relPath,
			FileSize:         uint32(info.Size()),
		}
		// Add the file digest to the file map
		var checksum [16]uint8
		copy(checksum[:], hash.Sum(nil)[:16])
		files[checksum] = f
		return nil
	}
	// Walk the directory using the above function
	filepath.Walk(sharePath, walkFunc)
	return files
}

func FileShare(output chan packets.Packet, outputDirected chan node.PeerPacket, input chan node.PeerPacket) {
	manifest := GetFileManifest()
	downloaders := make(map[[16]uint8]chan node.PeerPacket)
	downloaderPeers := make(map[[16]uint8]chan node.Node)
	for {
		select {
		case nodePkt := <-input:
			switch nodePkt.Packet.PacketType() {
			case packets.PacketTypeManifestHeader:
				// TODO: Compare the manifest to the local manifest and request digest headers from peers
			case packets.PacketTypeFileDigestHeader:
				// Create/update the file downloader for this file to use the sender as a peer
				header := *nodePkt.Packet.(*packets.FileDigestHeader)
				fileId := header.FileHash
				// If a downloader for this file doesn't already exist, make one
				if _, ok := downloaders[fileId]; !ok {
					downloaders[fileId] = make(chan node.PeerPacket)
					downloaderPeers[fileId] = make(chan node.Node)
					go FileDownloader(outputDirected, header, downloaders[fileId], downloaderPeers[fileId])
				}
				// Provide the sender as a pper
				downloaderPeers[fileId] <- nodePkt.Source
			case packets.PacketTypeFilePartHeader:
				// Pass the file part to the appropriate downloader if it exists
				header := *nodePkt.Packet.(*packets.FilePartHeader)
				fileId := header.FileHash
				if downloader, ok := downloaders[fileId]; ok {
					downloader <- nodePkt
				}
			case packets.PacketTypeFilePartRequestHeader:
				header := *nodePkt.Packet.(*packets.FilePartRequestHeader)
				fileDigest, ok := manifest[header.FileHash]
				// Check to make sure the file exists. TODO: download the file if it doesn't exist
				if !ok {
					continue
				}
				// Open the file
				file, err := os.OpenFile(filepath.Join(GetSharePath(), fileDigest.RelativeFilePath), os.O_RDONLY, 0700)
				if err != nil {
					continue
				}
				offset := 1024 * header.PartNumber
				buffer := make([]uint8, 1024)
				// Read the part from the file
				bytesRead, err := file.ReadAt(buffer, int64(offset))
				if err != nil && err != io.EOF {
					panic(err)
				}
				file.Close()
				// Send the file part
				filePart := new(packets.FilePartHeader)
				filePart.Initialize(header.FileHash, header.PartNumber, buffer[:bytesRead])
				outputDirected <- node.PeerPacket{filePart, nodePkt.Source}
			}
		}
	}
}

func FileDownloader(outputDirected chan node.PeerPacket, fileInfo packets.FileDigestHeader, input chan node.PeerPacket,
	newPeers chan node.Node) {
	// Determine the temp directory for the part to be stored in
	fileHash := make([]string, 0)
	for _, elem := range fileInfo.FileHash {
		fileHash = append(fileHash, fmt.Sprintf("%x", elem))
	}
	fileID := strings.Join(fileHash, "")
	tempDir := GetPartsPath(fileID)
	// Set up state variables for downloader
	numParts := uint16(math.Ceil(float64(fileInfo.FileSize) / 1024))
	partsNeeded := make(map[uint16]bool)
	peers := make(map[node.Node]bool)
	sequenceNumber := numParts - 1

	// Initialize the parts needed
	for i := uint16(0); i < numParts; i++ {
		partsNeeded[i] = true
	}

DOWNLOADLOOP:
	for {
		done := false
		select {
		case nodePkt := <-input:
			filePartHeader := *nodePkt.Packet.(*packets.FilePartHeader)
			partNum := filePartHeader.PartNumber
			peer := nodePkt.Source
			if _, ok := peers[peer]; !ok {
				peers[peer] = true
			}
			// Write the file partNum to disk
			partPath := filepath.Join(tempDir, fmt.Sprintf("%d.part", partNum))
			partFile, err := os.OpenFile(partPath, os.O_RDWR|os.O_CREATE, 0700) // TODO: Evaluate whether or not opens should be in rb/wb instead of r/w
			if err != nil {
				panic(err)
			}
			partFile.Write(filePartHeader.Data[:1024-filePartHeader.Padding]) // TODO: Extra error handling here
			partFile.Close()
			delete(partsNeeded, partNum)
			fmt.Print(partsNeeded)
			// Request the next file partNum
			k, partsLeft := GetNextKey(partsNeeded, numParts, sequenceNumber)
			if !partsLeft {
				done = true
				break
			} else {
				fmt.Print(k)
				sequenceNumber = k
			}
			partRequest := new(packets.FilePartRequestHeader)
			partRequest.Initialize(fileInfo.FileHash, sequenceNumber)
			outputDirected <- node.PeerPacket{partRequest, peer}

		case newPeer := <-newPeers:
			// Start downloading from the new peer if they don't already exist.
			if _, ok := peers[newPeer]; !ok {
				k, partsLeft := GetNextKey(partsNeeded, numParts, sequenceNumber)
				if !partsLeft {
					done = true
					break
				} else {
					sequenceNumber = k
				}
				partRequest := new(packets.FilePartRequestHeader)
				partRequest.Initialize(fileInfo.FileHash, sequenceNumber)
				outputDirected <- node.PeerPacket{partRequest, newPeer}
			}
		}
		if done {
			break
		}
	}

	outputFile, err := os.OpenFile(filepath.Join(GetSharePath(), fileInfo.FileName), os.O_RDWR|os.O_CREATE, 0700)
	if err != nil {
		panic(err)
	}
	defer outputFile.Close()

	partBuffer := make([]uint8, 1024)
	for i := uint16(0); i < numParts; i++ {
		partFile, err := os.OpenFile(filepath.Join(tempDir, fmt.Sprintf("%d.part", i)), os.O_RDONLY, 0700)
		if err != nil {
			panic(err)
		}
		bytesRead, err := partFile.Read(partBuffer)
		if err != nil {
			panic(err)
		}
		outputFile.Write(partBuffer[:bytesRead])
		partFile.Close()
	}
}

func GetNextKey(partsNeeded map[uint16]bool, numParts uint16, currentKey uint16) (uint16, bool) {
	currentKey %= numParts
	for nextKey := (currentKey + 1) % numParts; nextKey != currentKey; nextKey = (nextKey + 1) % numParts {
		if _, ok := partsNeeded[nextKey]; ok {
			return nextKey, true
		}
	}
	return currentKey, false
}
