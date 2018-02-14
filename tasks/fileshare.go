package tasks

import (
	"swarmd/node"
	"swarmd/packets"
)

func FileShare(output chan packets.Packet, input chan node.PeerPacket) {
	for {
		select {
		case nodePkt := <-input:
			switch nodePkt.Packet.PacketType() {
			case packets.PacketTypeManifestHeader:
				// TODO: Compare the manifest to the local manifest and request digest headers from peers
			case packets.PacketTypeFileDigestHeader:
				go FileDownloader(*(nodePkt.Packet.(*packets.FileDigestHeader)))
			case packets.PacketTypeFilePartHeader:
				// TODO: Extract the data and save it as a file part
			case packets.PacketTypeFilePartRequestHeader:
				// TODO: Respond with the contents of the file part.
			}
		}
	}
}

func FileDownloader(fileInfo packets.FileDigestHeader) {
	// TODO: Query peers for file parts
}