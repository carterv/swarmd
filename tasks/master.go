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

type commonStruct struct {
	Input         chan packets.PeerPacket
	Broadcast     chan packets.Packet
	Output        chan packets.PeerPacket
	FileShare     chan packets.PeerPacket
	ModuleControl chan moduleCommand
	Peers         chan node.Node
	PeerMap       *sync.Map
	KillFlag      *bool
	Key           [32]byte
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
	config := new(commonStruct)
	config.Input = make(chan packets.PeerPacket)
	config.Broadcast = make(chan packets.Packet)
	config.Output = make(chan packets.PeerPacket)
	config.FileShare = make(chan packets.PeerPacket)
	config.ModuleControl = make(chan moduleCommand)
	config.Peers = make(chan node.Node)
	config.PeerMap = new(sync.Map)
	config.KillFlag = killFlag
	config.Key = authentication.MakeKey(seed)

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

	go Listener(conn, config)
	go Talker(conn, config)
	go FileShare(config, self)
	go PeerManager(config, bootstrapper)
	go ModuleManager(config)

	for !*killFlag {
		select {
		case nodePkt := <-config.Input:
			//print(nodePkt.Packet.ToString())
			switch nodePkt.Packet.PacketType() {
			// Generic message packet
			case packets.PacketTypeMessageHeader:
				HandleMessage(config, nodePkt)
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
				config.FileShare <- nodePkt
			case packets.PacketTypeConnectionRequest:
				HandleConnectionRequest(config, nodePkt, self)
			case packets.PacketTypeConnectionShare:
				HandleConnectionShare(config, self, *nodePkt.Packet.(*packets.ConnectionShareHeader))
			case packets.PacketTypeConnectionAck:
				config.Peers <- nodePkt.Source
			}
		}
	}
}

func HandleMessage(config *commonStruct, pkt packets.PeerPacket) {
	msg := pkt.Packet.(*packets.MessageHeader).Message
	if msg == "__PING_REQ" { // Ping request -- respond with ack
		response := new(packets.MessageHeader)
		response.Initialize("__PING_ACK")
		nodePkt := packets.PeerPacket{Packet: response, Source: pkt.Source}
		config.Output <- nodePkt
	} else if msg == "__PING_ACK" { // Ping ack, mark peer as live
		config.PeerMap.Store(pkt.Source, 0)
	} else if msg == "__LIST_PEERS" {
		response := new(packets.MessageHeader)
		peers := ""
		config.PeerMap.Range(func(key, value interface{}) bool {
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
		config.Output <- nodePkt
	} else if strings.HasPrefix(msg, "__DEPLOY ") {
		if !createDeployment(msg, pkt.Source, config.Broadcast, config.Output) {
			response := new(packets.MessageHeader)
			response.Initialize("__DEPLOY_ERROR")
			nodePkt := packets.PeerPacket{Packet: response, Source: pkt.Source}
			config.Output <- nodePkt
		}
	} else if strings.HasPrefix(msg, "__MODULE") {
		handleModuleCommand(msg, config.ModuleControl)
		config.Broadcast <- pkt.Packet
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
	case "__MODULE_DELETE":
		command.Command = "delete"
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
