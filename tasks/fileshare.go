package tasks

import (
	"swarmd/util"
	"swarmd/packets"
	"swarmd/node"
	"path/filepath"
	"os"
	"crypto/md5"
	"io"
	"fmt"
	"math"
	"log"
	"time"
	"encoding/hex"
)

func GetSharePath() string {
	sharePath := filepath.Join(util.GetBasePath(), "share/")

	// Make the share directory if it doesn't exist
	os.MkdirAll(sharePath, 0700)

	return sharePath
}

func GetPartsPath(filehash string) string {
	partsPath := filepath.Join(util.GetBasePath(), "parts/", filehash)

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

func FileShare(killFlag *bool, output chan packets.Packet, outputDirected chan packets.PeerPacket, input chan packets.PeerPacket,
	self node.Node) {
	manifest := GetFileManifest()
	downloaders := make(map[[16]uint8]chan packets.PeerPacket, 10)
	downloaderPeers := make(map[[16]uint8]chan node.Node)
	downloadStarted := make(map[[16]uint8]bool)
	downloaderFinished := make(chan [16]uint8)
	for !*killFlag {
		select {
		case nodePkt := <-input:
			switch nodePkt.Packet.PacketType() {
			case packets.PacketTypeDeployment:
				// Check to make sure the file hasn't already been downloaded
				fileHash := nodePkt.Packet.(*packets.DeploymentHeader).FileHash
				manifest = GetFileManifest()
				if _, ok := manifest[fileHash]; !ok {
					if _, ok := downloaders[fileHash]; !ok {
						downloaders[fileHash] = make(chan packets.PeerPacket)
						downloaderPeers[fileHash] = make(chan node.Node)
						downloadStarted[fileHash] = false
						// Request the file
						fileRequest := new(packets.FileRequestHeader)
						fileRequest.Initialize(fileHash, self)
						output <- fileRequest
					}
				}
				output <- nodePkt.Packet
			case packets.PacketTypeManifestHeader:
				// TODO: Compare the manifest to the local manifest and request digest headers from peers
				// This may be unneeded
			case packets.PacketTypeFileRequestHeader:
				fileHash := nodePkt.Packet.(*packets.FileRequestHeader).FileHash
				requester := nodePkt.Packet.(*packets.FileRequestHeader).GetRequester()
				// Check to see if we have a copy of the requested file
				manifest = GetFileManifest()
				if digest, ok := manifest[fileHash]; ok {
					// Respond that we have a copy of the packet
					fileDigest := new(packets.FileDigestHeader)
					fileDigest.Initialize(fileHash, digest.FileSize, digest.RelativeFilePath)
					outputDirected <- packets.PeerPacket{Packet: fileDigest, Source: requester}
				}
				// Broadcast the file request to all peers
				output <- nodePkt.Packet
			case packets.PacketTypeFileDigestHeader:
				// Create/update the file downloader for this file to use the sender as a peer
				header := *nodePkt.Packet.(*packets.FileDigestHeader)
				fileHash := header.FileHash
				// If a downloader for this file doesn't already exist, ignore the packet
				if started, ok := downloadStarted[fileHash]; ok {
					if !started {
						go FileDownloader(outputDirected, output, self, header, downloaders[fileHash], downloaderPeers[fileHash],
							downloaderFinished)
						downloadStarted[fileHash] = true
					}
					// Provide the sender as a peer
					downloaderPeers[fileHash] <- nodePkt.Source
				}
			case packets.PacketTypeFilePartHeader:
				// Pass the file part to the appropriate downloader if it exists
				header := *nodePkt.Packet.(*packets.FilePartHeader)
				fileHash := header.FileHash
				if downloader, ok := downloaders[fileHash]; ok {
					// Non-block write to the downloader to fix the case where the downloader is finished
					select {
					case downloader <- nodePkt:
					default:
					}
				}
			case packets.PacketTypeFilePartRequestHeader:
				header := *nodePkt.Packet.(*packets.FilePartRequestHeader)
				fileDigest, ok := manifest[header.FileHash]
				// Check to make sure the file exists. TODO: download the file if it doesn't exist
				if !ok {
					continue
				}
				buffer := make([]uint8, 1024)
				bytesRead := getFilePart(fileDigest.RelativeFilePath, header.PartNumber, buffer)
				// Send the file part
				filePart := new(packets.FilePartHeader)
				filePart.Initialize(header.FileHash, header.PartNumber, buffer[:bytesRead])
				outputDirected <- packets.PeerPacket{filePart, nodePkt.Source}
			}
		case fileHash := <-downloaderFinished:
			// Refresh the manifest and cleanup
			manifest = GetFileManifest()
			close(downloaders[fileHash])
			delete(downloaders, fileHash)
			close(downloaderPeers[fileHash])
			delete(downloaderPeers, fileHash)
			delete(downloadStarted, fileHash)
		}
	}
}

func FileDownloader(outputDirected chan packets.PeerPacket, outputGeneral chan packets.Packet, self node.Node,
	fileInfo packets.FileDigestHeader, input chan packets.PeerPacket, newPeers chan node.Node,
	eventStream chan [16]uint8) {
	// Download finished notification
	defer (func() { log.Print("Signalling"); eventStream <- fileInfo.FileHash; log.Print("Signalled") })()
	// Determine the temp directory for the part to be stored in
	fileID := hex.EncodeToString(fileInfo.FileHash[:])
	tempDir := GetPartsPath(fileID)
	// Set up state variables for downloader
	numParts := uint16(math.Ceil(float64(fileInfo.FileSize) / 1024))
	partsNeeded := make(map[uint16]bool)
	peers := make(map[node.Node]bool)
	sequenceNumber := numParts - 1
	packetCount := 0

	// Initialize the parts needed
	for i := uint16(0); i < numParts; i++ {
		partsNeeded[i] = true
	}
	log.Printf("[%s] Starting to download parts...", fileInfo.FileName)
	done := false
	for !done {
		select {
		case nodePkt := <-input:
			if packetCount%50 == 0 {
				log.Printf("[%s] %.2f%%\n", fileInfo.FileName, 100*(1-float32(len(partsNeeded))/float32(numParts)))
			}

			packetCount += 1
			filePartHeader := *nodePkt.Packet.(*packets.FilePartHeader)
			partNum := filePartHeader.PartNumber
			peer := nodePkt.Source
			if _, ok := peers[peer]; !ok {
				peers[peer] = true
			}
			if _, ok := partsNeeded[partNum]; ok {
				delete(partsNeeded, partNum)
				writeFilePart(tempDir, partNum, filePartHeader)
			}
			// Request the next file part
			if getNextKey(partsNeeded, numParts, &sequenceNumber) {
				getNextPart(sequenceNumber, fileInfo, outputDirected, peer)
			} else {
				done = true
			}
		case newPeer := <-newPeers:
			// Start downloading from the new peer if they don't already exist.
			if _, ok := peers[newPeer]; !ok {
				peers[newPeer] = true
				if getNextKey(partsNeeded, numParts, &sequenceNumber) {
					getNextPart(sequenceNumber, fileInfo, outputDirected, newPeer)
				} else {
					done = true
				}
			}
		case <-time.After(10 * time.Second):
			// Time out after 10 seconds of no packets/new peers, send out a file request to get new peers
			fileRequest := new(packets.FileRequestHeader)
			fileRequest.Initialize(fileInfo.FileHash, self)
			outputGeneral <- fileRequest
		}
	}
	log.Printf("[%s] Parts downloaded", fileInfo.FileName)

	outputFile, err := os.OpenFile(filepath.Join(GetSharePath(), fileInfo.FileName), os.O_RDWR|os.O_CREATE|os.O_TRUNC,
		0700)
	if err != nil {
		log.Printf("[%s] Unable to download file: %v", fileInfo.FileName, err)
		return
	}
	defer outputFile.Close()

	partBuffer := make([]uint8, 1024)
	assemblePart := func(i uint16) bool {
		partFile, err := os.OpenFile(filepath.Join(tempDir, fmt.Sprintf("%d.part", i)), os.O_RDONLY, 0700)
		if err != nil {
			log.Printf("[%s] Unable to write parts: %v", fileInfo.FileName, err)
			return false
		}
		defer partFile.Close()
		bytesRead, err := partFile.Read(partBuffer)
		if err != nil {
			log.Printf("[%s] Unable to write parts: %v", fileInfo.FileName, err)
			return false
		}
		outputFile.Write(partBuffer[:bytesRead])
		return true
	}
	for i := uint16(0); i < numParts; i++ {
		if !assemblePart(i) {
			return
		}
	}
	// Clean up the temporary files
	os.RemoveAll(tempDir)

	log.Printf("[%s] File assembled", fileInfo.FileName)
}

func getFilePart(relativeFilePath string, partNumber uint16, buffer []uint8) uint16 {
	file, err := os.OpenFile(filepath.Join(GetSharePath(), relativeFilePath), os.O_RDONLY, 0700)
	if err != nil {
		return 0
	}
	defer file.Close()
	offset := 1024 * uint32(partNumber)
	// Read the part from the file
	bytesRead, err := file.ReadAt(buffer, int64(offset))
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	return uint16(bytesRead)
}

func writeFilePart(tempDir string, partNum uint16, filePartHeader packets.FilePartHeader) {
	// Write the file partNum to disk
	partPath := filepath.Join(tempDir, fmt.Sprintf("%d.part", partNum))
	partFile, err := os.OpenFile(partPath, os.O_RDWR|os.O_CREATE, 0700)
	if err != nil {
		log.Fatal(err)
	}
	defer partFile.Close()
	partFile.Write(filePartHeader.Data[:1024-filePartHeader.Padding])
}

func getNextPart(sequenceNumber uint16, fileInfo packets.FileDigestHeader, outputDirected chan packets.PeerPacket, newPeer node.Node) {
	partRequest := new(packets.FilePartRequestHeader)
	partRequest.Initialize(fileInfo.FileHash, sequenceNumber)
	outputDirected <- packets.PeerPacket{partRequest, newPeer}
}

func getNextKey(partsNeeded map[uint16]bool, numParts uint16, currentKey *uint16) bool {
	*currentKey %= numParts
	if len(partsNeeded) == 0 {
		return false
	}
	for nextKey := (*currentKey + 1) % numParts; nextKey != *currentKey; nextKey = (nextKey + 1) % numParts {
		if _, ok := partsNeeded[nextKey]; ok {
			*currentKey = nextKey
			return true
		}
	}
	for key := range partsNeeded {
		*currentKey = key
		return true
	}
	return false
}
