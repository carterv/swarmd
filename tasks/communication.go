package tasks

import (
	"swarmd/packets"
	"net"
	"swarmd/node"
	"log"
	"fmt"
	"encoding/hex"
)

func Listener(output chan node.PeerPacket) {
	conn, err := net.ListenPacket("udp", "0.0.0.0:51234")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	for {
		// Read the raw byte stream
		data := make(packets.SerializedPacket, 2048)
		_, addr, err := conn.ReadFrom(data)
		if err != nil {
			log.Fatal(err)
		}
		sourceNode, err := node.BuildNode(addr)
		if err != nil {
			fmt.Printf("Error occurred while attempting to parse packet source, discarding\n")
			continue
		}
		nodePkt := node.PeerPacket{Packet: nil, Source: sourceNode}
		// Deserialize the data based off the data type
		// TODO: Test this and hope like hell that it doesn't throw errors
		switch data[2] {
		case packets.PacketTypeMessageHeader:
			nodePkt.Packet = new(packets.MessageHeader)
		case packets.PacketTypeManifestHeader:
			nodePkt.Packet = new(packets.ManifestHeader)
		case packets.PacketTypeFileDigestHeader:
			nodePkt.Packet = new(packets.FileDigestHeader)
		case packets.PacketTypeFilePartHeader:
			nodePkt.Packet = new(packets.FilePartHeader)
		case packets.PacketTypeFilePartRequestHeader:
			nodePkt.Packet = new(packets.FilePartRequestHeader)
		default:
			fmt.Printf("Unknown packet: \n%s\n", hex.Dump(data))
		}
		// Error handling
		if !nodePkt.Packet.Deserialize(data) {
			fmt.Print("Packet format does not match packet number\n")
			continue
		}
		if !nodePkt.Packet.IsValid() {
			fmt.Print("Invalid checksum\n")
			continue
		}
		// Group the source and packet
		output <- nodePkt
	}
}

func Talker(input chan packets.Packet, peerChan chan node.Node) {
	peers := make([]node.Node, 0)
	for {
		select {
		case pkt := <-input:
			SendToAll(pkt, peers)
		case peer := <-peerChan:
			peers = append(peers, peer)
		}
	}
}

func SendToAll(pkt packets.Packet, peers []node.Node) {
	for _, peer := range peers {
		addr := fmt.Sprintf("%s:%d", peer.Address, peer.Port)
		conn, err := net.Dial("udp", addr)
		if err != nil {
			fmt.Printf("%v\n", err)
			continue
		}
		conn.Write(pkt.Serialize())
		conn.Close()
	}
}
