package main

import (
	"flag"
	"swarmd/node"
	"swarmd/tasks"
	"net"
	"fmt"
	"log"
	"swarmd/packets"
	"swarmd/authentication"
	"bufio"
	"os"
	"strings"
	"regexp"
	"path/filepath"
	"swarmd/util"
	"runtime"
)

func main() {
	// Parse arguments
	portPtr := flag.Int("port", 51234, "The port on which the local instance is running")
	keyPtr := flag.String("key", "", "The encryption key to use for communications")

	flag.Parse()
	localAddress := tasks.GetOutboundIP()
	key := authentication.MakeKey(*keyPtr)

	localNode := node.Node{
		Address: localAddress.String(),
		Port:    uint16(*portPtr),
	}

	self := node.Node{
		Address: localAddress.String(),
		Port:    0,
	}

	localAddr := util.GetAddr(localNode)

	conn := setupConnection(key, self, localAddr)
	defer conn.Close()

	startPrompt(conn, key, localAddr)
}



func startPrompt(conn net.PacketConn, key [32]uint8, localAddr net.Addr) {
	reader := bufio.NewReader(os.Stdin)
	targetRegex, err := regexp.Compile("^[a-zA-Z0-9][-_a-zA-Z0-9]*$")
	if err != nil {
		log.Fatal(err)
	}

	for {
		fmt.Print("> ")
		text, _ := reader.ReadString('\n')
		words := strings.Fields(text)
		if len(words) == 0 {
			continue
		}
		cmd := words[0]
		switch cmd {
		case "deploy":
			createDeployment(conn, key, localAddr, words, targetRegex)
		case "signal":
			handleSignal(conn, key, localAddr, words, targetRegex)
		case "quit":
			return
		default:
			fmt.Printf("Invalid command: %s\n", cmd)
		}
	}
}

func handleSignal(conn net.PacketConn, key [32]uint8, localAddr net.Addr, words []string, targetRegex *regexp.Regexp) {
	if len(words) != 3 {
		fmt.Printf("Usage: signal target command\n")
		return
	}
	target := words[1]
	if !targetRegex.MatchString(target) {
		fmt.Printf("Invalid target: must match %s\n", targetRegex.String())
		return
	}
	command := words[2]
	if command != "start" && command != "stop" && command != "install" && command != "uninstall" {
		fmt.Printf("Invalid command, must be in (start, stop, install, uninstall)\n")
		return
	}

	signalPacket := new(packets.MessageHeader)
	signalPacket.Initialize(fmt.Sprintf("__MODULE_%s %s", strings.ToUpper(command), target))
	util.SendPacket(conn, localAddr, key, signalPacket)
}

func createDeployment(conn net.PacketConn, key [32]uint8, localAddr net.Addr, words []string, targetRegex *regexp.Regexp) {
	if len(words) != 3 {
		fmt.Printf("Usage: deploy target source\n")
		return
	}
	target := words[1]
	if !targetRegex.MatchString(target) {
		fmt.Printf("Invalid target: must match %s\n", targetRegex.String())
		return
	}
	sourcePath := words[2]
	targetPath := filepath.Join(util.GetBasePath(), "share", fmt.Sprintf("%s.swm", target))
	fmt.Printf("Searching for files in source directory...\n")

	extension := ""
	if runtime.GOOS == "windows" {
		extension = "ps1"
	} else {
		extension = "sh"
	}

	packageFiles := []string{
		filepath.Join(sourcePath, fmt.Sprintf("install.%s", extension)),
		filepath.Join(sourcePath, fmt.Sprintf("uninstall.%s", extension)),
		filepath.Join(sourcePath, fmt.Sprintf("start.%s", extension)),
		filepath.Join(sourcePath, fmt.Sprintf("stop.%s", extension)),
		filepath.Join(sourcePath, "payload.zip"),
	}

	for _, filePath := range packageFiles {
		if _, err := os.Stat(filePath); err != nil {
			fmt.Printf("Unable to find file: %s\n", filePath)
			return
		}
	}

	os.RemoveAll(targetPath)

	fmt.Printf("Archiving files...\nDeployment target: %s\n", targetPath)
	err := util.ZipFiles(targetPath, packageFiles)
	if err != nil {
		fmt.Printf("Unable to create archive: %v\n", err)
		return
	}

	deploymentPacket := new(packets.MessageHeader)
	deploymentPacket.Initialize(fmt.Sprintf("__DEPLOY %s", target))
	util.SendPacket(conn, localAddr, key, deploymentPacket)

	// Wait for response
	buffer := make(packets.SerializedPacket, 2048)
	length, _, err := conn.ReadFrom(buffer)
	if err != nil {
		log.Fatal(err)
	}
	// Deserialize/validate the response packet
	data := authentication.DecryptPacket(buffer[:length], key)
	pkt := new(packets.Packet)
	packets.InitializePacket(pkt, data[2])
	if !(*pkt).Deserialize(data) || !(*pkt).IsValid() || (*pkt).PacketType() != packets.PacketTypeMessageHeader {
		log.Fatal("Received bad packet from node")
	}
	// Check the response body
	respPkt := (*pkt).(*packets.MessageHeader)
	if respPkt.Message == "__DEPLOY_ACK" {
		fmt.Println("Deployment initiated successfully")
	} else {
		fmt.Println("Error occurred while starting deployment")
	}
}

func setupConnection(key [32]byte, self node.Node, localAddr net.Addr) net.PacketConn {
	// Create a listening udp socket
	conn, err := net.ListenPacket("udp", fmt.Sprintf("[::]:%d", self.Port))
	if err != nil {
		log.Fatal(err)
	}
	// Ping the local node
	pingPkt := new(packets.MessageHeader)
	pingPkt.Initialize("__PING_REQ")
	fmt.Println("Pinging local node...")
	util.SendPacket(conn, localAddr, key, pingPkt)
	// Wait for response
	buffer := make(packets.SerializedPacket, 2048)
	length, _, err := conn.ReadFrom(buffer)
	if err != nil {
		log.Fatal(err)
	}
	// Deserialize/validate the response packet
	data := authentication.DecryptPacket(buffer[:length], key)
	pkt := new(packets.Packet)
	packets.InitializePacket(pkt, data[2])
	if !(*pkt).Deserialize(data) || !(*pkt).IsValid() || (*pkt).PacketType() != packets.PacketTypeMessageHeader {
		log.Fatal("Received bad packet from node")
	}
	// Check the response body
	respPkt := (*pkt).(*packets.MessageHeader)
	if respPkt.Message != "__PING_ACK" {
		log.Fatal("Unable to connect to node")
	} else {
		fmt.Println("Ping ack received")
	}
	return conn
}

