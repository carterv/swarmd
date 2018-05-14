package tasks

import (
	"swarmd/packets"
	"fmt"
	"swarmd/node"
	"net"
	"log"
	"swarmd/authentication"
	"os"
	"strconv"
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

func Run(killFlag *bool, bootstrapHost string, bootstrapPort int, seed string) {
	inputGeneral := make(chan packets.PeerPacket)
	outputGeneral := make(chan packets.Packet)
	outputDirected := make(chan packets.PeerPacket)
	outputFileShare := make(chan packets.PeerPacket)
	peerChan := make(chan node.Node)
	peerMap := make(map[node.Node]int)
	key := authentication.MakeKey(seed)

	// Setup the port for connections
	var bootstrapper *node.Node
	if bootstrapHost != "" {
		bootstrapper = new(node.Node)
		bootstrapper.Address = bootstrapHost
		bootstrapper.Port = uint16(bootstrapPort)
		log.Printf("Configured bootstrap node: %s:%d", bootstrapper.Address, bootstrapper.Port)
	}
	localAddress := GetOutboundIP()

	portStr, present := os.LookupEnv("SWARMD_LOCAL_PORT")
	myPort := uint16(51234)
	if present {
		tempPort, _ := strconv.ParseInt(portStr, 10, 32)
		if tempPort > 0 && tempPort < 65536 {
			myPort = uint16(tempPort)
			log.Printf("Using alternative local port: %d", myPort)
		} else {
			log.Fatalf("Environment variable SWARMD_LOCAL_PORT has bad value: %s", portStr)
		}
	}

	self := node.Node{Address: localAddress.String(), Port: myPort}
	address := fmt.Sprintf("[::]:%d", self.Port)

	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	go Listener(killFlag, conn, key, inputGeneral)
	go Talker(killFlag, conn, key, outputGeneral, outputDirected, peerMap)
	go FileShare(killFlag, outputGeneral, outputDirected, outputFileShare, self)
	go PeerManager(killFlag, bootstrapper, outputDirected, peerMap, peerChan)

	for !*killFlag {
		select {
		case nodePkt := <-inputGeneral:
			//print(nodePkt.Packet.ToString())
			switch nodePkt.Packet.PacketType() {
			// Generic message packet
			case packets.PacketTypeMessageHeader:
				HandleMessage(nodePkt, outputGeneral, outputDirected, peerChan, peerMap)
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
				HandleConnectionRequest(nodePkt, outputGeneral, outputDirected, self)
			case packets.PacketTypeConnectionShare:
				HandleConnectionShare(*nodePkt.Packet.(*packets.ConnectionShareHeader), peerChan, peerMap, outputDirected, outputGeneral)
			case packets.PacketTypeConnectionAck:
				peerChan <- nodePkt.Source
			}
		}
	}
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
		log.Print(pkt.Packet.ToString())
	}
}
