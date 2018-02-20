package tasks

import (
	"swarmd/packets"
	"fmt"
	"swarmd/node"
	"os"
	"net"
	"log"
)

func Run() {
	inputGeneral := make(chan node.PeerPacket)
	outputGeneral := make(chan packets.Packet)
	outputDirected := make(chan node.PeerPacket)
	outputFileShare := make(chan node.PeerPacket)
	peers := make(chan node.Node)

	// Setup the port for connections
	var address string
	if os.Getenv("SWARMNODE") == "a" {
		address = "[::1]:51234"
	} else {
		address = "[::1]:51235"
	}
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	go Listener(conn, inputGeneral)
	go Talker(conn, outputGeneral, outputDirected, peers)
	go FileShare(outputGeneral, outputDirected, outputFileShare)

	// Use self as peer for now
	if os.Getenv("SWARMNODE") == "a" {
		peers <- node.Node{"[::1]", 51235}
		manifest := GetFileManifest()
		for k, v := range manifest {
			pkt := new(packets.FileDigestHeader)
			pkt.Initialize(k, v.FileSize, v.RelativeFilePath)
			outputGeneral <- pkt
		}

	} else {
		peers <- node.Node{"[::1]", 51234}
	}

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
		}
	}
}
