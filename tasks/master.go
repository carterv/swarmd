package tasks

import (
	"swarmd/packets"
	"fmt"
	"swarmd/node"
	"net"
	"log"
	"time"
)

// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func Run(bootstrapHost string, bootstrapPort int) {
	inputGeneral := make(chan packets.PeerPacket)
	outputGeneral := make(chan packets.Packet)
	outputDirected := make(chan packets.PeerPacket)
	outputFileShare := make(chan packets.PeerPacket)
	peerChan := make(chan node.Node)
	peerMap := make(map[node.Node]int)

	// Setup the port for connections
	var bootstrapper *node.Node
	if bootstrapHost != "" {
		bootstrapper := new(node.Node)
		bootstrapper.Address = bootstrapHost
		bootstrapper.Port = uint16(bootstrapPort)
	}
	localAddress := GetOutboundIP()
	self := node.Node{Address: localAddress.String(), Port: 51234}
	address := fmt.Sprintf("[::]:%d", self.Port)

	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	go Listener(conn, inputGeneral)
	go Talker(conn, outputGeneral, outputDirected, peerMap)
	go FileShare(outputGeneral, outputDirected, outputFileShare, self)
	go PeerManager(bootstrapper, outputDirected, peerMap, peerChan)

	for {
		select {
		case nodePkt := <-inputGeneral:
			//print(nodePkt.Packet.ToString())
			switch nodePkt.Packet.PacketType() {
			// Generic message packet
			case packets.PacketTypeMessageHeader:
				HandleMessage(nodePkt, outputGeneral, outputDirected, peerChan, peerMap)
				fmt.Print(nodePkt.Packet.ToString())
				// File Share packets
			case packets.PacketTypeFileDigestHeader:
				fallthrough
			case packets.PacketTypeFilePartRequestHeader:
				fallthrough
			case packets.PacketTypeFilePartHeader:
				fallthrough
			case packets.PacketTypeFileRequestHeader:
				fallthrough
			case packets.PacketTypeDeployment:
				fallthrough
			case packets.PacketTypeManifestHeader:
				outputFileShare <- nodePkt
			case packets.PacketTypeConnectionRequest:
				HandleConnectionRequest(nodePkt, outputGeneral)
			case packets.PacketTypeConnectionShare:
				HandleConnectionShare(*nodePkt.Packet.(*packets.ConnectionShareHeader), peerChan, peerMap, outputDirected, outputGeneral)
			case packets.PacketTypeConnectionAck:
				peerChan <- nodePkt.Source
			}
		}
	}
}

func PeerManager(bootstrapper *node.Node, outputDirected chan packets.PeerPacket, peerMap map[node.Node]int, peerChan chan node.Node) {
	threshold := uint8(3)
	for {
		select {
		case peer := <-peerChan:
			peerMap[peer] = 0
		case <-time.After(60 * time.Second):
			// Periodically send out a connection request with an increasing threshold if there are no peers
			if len(peerMap) == 0 && bootstrapper != nil {
				pkt := new(packets.ConnectionRequestHeader)
				pkt.Initialize(threshold)
				threshold += 1
				outputDirected <- packets.PeerPacket{Packet: pkt, Source: *bootstrapper}
			} else {
				threshold = 3
			}
		case <-time.After(120 * time.Second):
			// Ping routine
			deadPeers := make([]node.Node, 0)
			pkt := new(packets.MessageHeader)
			pkt.Initialize("__PING_REQ")
			// Ping peers that have responded recently
			for peer, pings := range peerMap {
				if pings == 5 {
					deadPeers = append(deadPeers, peer)
				} else {
					outputDirected <- packets.PeerPacket{Packet: pkt, Source: peer}
					peerMap[peer] = pings + 1
				}
			}
			// Remove dead peers (failed to respond to five pings)
			for _, peer := range deadPeers {
				delete(peerMap, peer)
			}
		}
	}
}

func HandleConnectionShare(pkt packets.ConnectionShareHeader, peerChan chan node.Node, peerMap map[node.Node]int, outputDirected chan packets.PeerPacket, outputGeneral chan packets.Packet) {
	outputGeneral <- &pkt
	if len(peerMap) < int(pkt.Threshold) {
		ack := new(packets.ConnectionAckHeader)
		ack.Initialize()
		peer := node.Node{Address: pkt.Requester, Port: pkt.RequesterPort}
		outputDirected <- packets.PeerPacket{Packet: ack, Source: peer}
		peerChan <- peer
	}
}

func HandleConnectionRequest(request packets.PeerPacket, outputGeneral chan packets.Packet) {
	pkt := new(packets.ConnectionShareHeader)
	pkt.Initialize(request.Source, request.Packet.(*packets.ConnectionRequestHeader).Threshold)
	outputGeneral <- pkt
}

func HandleMessage(pkt packets.PeerPacket, outputGeneral chan packets.Packet, outputDirected chan packets.PeerPacket, peerChan chan node.Node, peerMap map[node.Node]int) {
	msg := pkt.Packet.(*packets.MessageHeader).Message
	if msg == "__PING_REQ" { // Ping request -- respond with ack
		response := packets.MessageHeader{}
		response.Initialize("__PING_ACK")
		nodePkt := packets.PeerPacket{Packet: &response, Source: pkt.Source}
		outputDirected <- nodePkt
	} else if msg == "__PING_ACK" { // Ping ack, mark peer as live
		peerMap[pkt.Source] = 0
	} else { // Other message, print it
		fmt.Print(pkt.Packet.ToString())
	}
}
