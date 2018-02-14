package tasks

import (
	"swarmd/packets"
	"fmt"
	"math/rand"
	"swarmd/node"
)

func Run() {
	inputGeneral := make(chan node.PeerPacket)
	outputGeneral := make(chan packets.Packet)
	outputFileShare := make(chan node.PeerPacket)
	peers := make(chan node.Node)
	i := 0

	go Listener(inputGeneral)
	go Talker(outputGeneral, peers)
	go FileShare(outputGeneral, outputFileShare)

	// Use self as peer for now
	peers <- node.Node{"localhost", 51234}

	for {
		select {
		case nodePkt := <-inputGeneral:
			switch nodePkt.Packet.PacketType() {
			// Generic message packet
			case packets.PacketTypeMessageHeader:
				fmt.Print(nodePkt.Packet.ToString())
			// File Share packets
			case packets.PacketTypeFileDigestHeader:
				fallthrough
			case packets.PacketTypeFilePartRequestHeader:
				fallthrough
			case packets.PacketTypeFilePartHeader:
				fallthrough
			case packets.PacketTypeManifestHeader:
				outputFileShare <- nodePkt
			}
		default:
			if i == 0 {
				r := rand.Int() % 100
				if r == 0 {
					var pkt packets.MessageHeader
					pkt.Initialize(fmt.Sprintf("Test message %d", i))
					outputGeneral <- &pkt
					i += 1
				}
			}
		}
	}
}
