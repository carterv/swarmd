package tasks

import (
	"swarmd/packets"
	"fmt"
	"swarmd/node"
	"os"
	"net"
	"log"
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

func Run() {
	inputGeneral := make(chan packets.PeerPacket)
	outputGeneral := make(chan packets.Packet)
	outputDirected := make(chan packets.PeerPacket)
	outputFileShare := make(chan packets.PeerPacket)
	peers := make(chan node.Node)

	// Setup the port for connections
	var address string
	self := node.Node{Address:GetOutboundIP().String()}
	if os.Getenv("SWARMNODE") == "a" {
		address = "[::]:51234"
		self.Port = 51234
	} else {
		address = "[::]:51235"
		self.Port = 51235
	}
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	go Listener(conn, inputGeneral)
	go Talker(conn, outputGeneral, outputDirected, peers)
	go FileShare(outputGeneral, outputDirected, outputFileShare, self)

	// TODO: Proper peer discovery
	if os.Getenv("SWARMNODE") == "a" {
		peers <- node.Node{"[::1]", 51235}
		manifest := GetFileManifest()
		for k := range manifest {
			pkt := new(packets.DeploymentHeader)
			pkt.Initialize(k)
			outputGeneral <- pkt
		}
	} else {
		peers <- node.Node{"[::1]", 51234}
	}

	for {
		select {
		case nodePkt := <-inputGeneral:
			//print(nodePkt.Packet.ToString())
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
			case packets.PacketTypeFileRequestHeader:
				fallthrough
			case packets.PacketTypeDeployment:
				fallthrough
			case packets.PacketTypeManifestHeader:
				outputFileShare <- nodePkt
			}
		}
	}
}
