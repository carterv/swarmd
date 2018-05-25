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
	"strings"
	"crypto/md5"
	"io"
	"path/filepath"
	"sync"
)

type moduleCommand struct {
	ModuleName string
	Command    string
}

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
	moduleCommands := make(chan moduleCommand)
	peerChan := make(chan node.Node)
	//peerMap := make(map[node.Node]int)
	peerMap := new(sync.Map)
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
	go ModuleManager(killFlag, moduleCommands)

	for !*killFlag {
		select {
		case nodePkt := <-inputGeneral:
			//print(nodePkt.Packet.ToString())
			switch nodePkt.Packet.PacketType() {
			// Generic message packet
			case packets.PacketTypeMessageHeader:
				HandleMessage(nodePkt, outputGeneral, outputDirected, moduleCommands, peerMap)
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
				HandleConnectionShare(self, *nodePkt.Packet.(*packets.ConnectionShareHeader), peerChan, peerMap, outputDirected, outputGeneral)
			case packets.PacketTypeConnectionAck:
				peerChan <- nodePkt.Source
			}
		}
	}
}

func HandleMessage(pkt packets.PeerPacket, outputGeneral chan packets.Packet, outputDirected chan packets.PeerPacket,
	moduleCommands chan moduleCommand, peerMap *sync.Map) {
	msg := pkt.Packet.(*packets.MessageHeader).Message
	if msg == "__PING_REQ" { // Ping request -- respond with ack
		response := new(packets.MessageHeader)
		response.Initialize("__PING_ACK")
		nodePkt := packets.PeerPacket{Packet: response, Source: pkt.Source}
		outputDirected <- nodePkt
	} else if msg == "__PING_ACK" { // Ping ack, mark peer as live
		peerMap.Store(pkt.Source, 0)
	} else if msg == "__LIST_PEERS" {
		response := new(packets.MessageHeader)
		peers := ""
		peerMap.Range(func(key, value interface{}) bool {
			peer := key.(node.Node)
			if peers != "" {
				peers = fmt.Sprintf("%s,%s:%d", peers, peer.Address, peer.Port)
			} else {
				peers = fmt.Sprintf("%s:%d", peer.Address, peer.Port)
			}
			return true
		})
		response.Initialize(fmt.Sprintf("__LIST_RSP%s", peers))
		nodePkt := packets.PeerPacket{Packet: response, Source: pkt.Source}
		outputDirected <- nodePkt
	} else if strings.HasPrefix(msg, "__DEPLOY ") {
		if !createDeployment(msg, pkt.Source, outputGeneral, outputDirected) {
			response := new(packets.MessageHeader)
			response.Initialize("__DEPLOY_ERROR")
			nodePkt := packets.PeerPacket{Packet: response, Source: pkt.Source}
			outputDirected <- nodePkt
		}
	} else if strings.HasPrefix(msg, "__MODULE") {
		handleModuleCommand(msg, moduleCommands)
		outputGeneral <- pkt.Packet
	} else { // Other message, print it
		log.Print(pkt.Packet.ToString())
	}
}

func handleModuleCommand(msg string, moduleCommands chan moduleCommand) {
	words := strings.Split(msg, " ")
	if len(words) != 2 {
		return
	}
	command := moduleCommand{
		ModuleName: words[1],
		Command:    "",
	}
	switch words[0] {
	case "__MODULE_INSTALL":
		command.Command = "install"
	case "__MODULE_START":
		command.Command = "start"
	case "__MODULE_STOP":
		command.Command = "stop"
	case "__MODULE_UNINSTALL":
		command.Command = "uninstall"
	default:
		return
	}
	moduleCommands <- command
}

func createDeployment(msg string, source node.Node, outputGeneral chan packets.Packet,
	outputDirected chan packets.PeerPacket) bool {
	words := strings.Split(msg, " ")
	if len(words) != 2 {
		return false
	}
	log.Printf("Starting deployment for %s", words[1])
	targetPath := filepath.Join(GetSharePath(), fmt.Sprintf("%s.swm", words[1]))
	file, err := os.Open(targetPath)
	if err != nil {
		log.Printf("Error opening target module: %v\n", err)
		return false
	}
	defer file.Close()
	// Generate the checksum
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		log.Printf("Error getting file hash: %v\n", err)
		return false
	}
	// Kick off the deployment
	var fileHash [16]uint8
	copy(fileHash[:], hash.Sum(nil)[:16])
	deploymentPacket := new(packets.DeploymentHeader)
	deploymentPacket.Initialize(fileHash)
	outputGeneral <- deploymentPacket
	// Send the response to the console
	response := new(packets.MessageHeader)
	response.Initialize("__DEPLOY_ACK")
	nodePkt := packets.PeerPacket{Packet: response, Source: source}
	outputDirected <- nodePkt
	return true
}
