package tasks

import (
	"swarmd/packets"
	"net"
	"swarmd/node"
	"log"
	"fmt"
	"encoding/hex"
)

func Listener(output chan packets.Packet) {
	conn, err := net.ListenPacket("udp", "0.0.0.0:51234")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	for {
		data := make(packets.SerializedPacket, 2048)
		_, _, err := conn.ReadFrom(data)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print("Attempting to parse packet\n")
		switch data[2] {
		case packets.MessageType:
			var pkt packets.MessageHeader
			pkt.Deserialize(data)
			if !pkt.Common.ValidChecksum {
				fmt.Print("Invalid checksum\n")
				continue
			}
			output <- &pkt
		default:
			fmt.Printf("Unknown packet: \n%s\n", hex.Dump(data))
		}
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
