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
	"crypto/md5"
	"io"
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

	localAddr := getAddr(localNode)

	conn := setupConnection(key, self, localAddr)
	defer conn.Close()

	startPrompt(conn, key, localAddr)
}

func getAddr(n node.Node) net.Addr {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", n.Address, n.Port))
	if err != nil {
		log.Fatal(err)
	}
	return addr
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
		case "quit":
			return
		default:
			fmt.Printf("Invalid command: %s\n", cmd)
		}
	}
}

func createDeployment(conn net.PacketConn, key [32]uint8, localAddr net.Addr, words []string, targetRegex *regexp.Regexp) {
	if len(words) != 3 {
		fmt.Printf("Usage: deploy target source\n")
		return
	}
	target := words[1]
	if !targetRegex.MatchString(target) {
		fmt.Printf("Invalid target: must match %s\n", targetRegex.String())
	}
	sourcePath := words[2]
	targetPath := filepath.Join(util.GetBasePath(), "share", fmt.Sprintf("%s.swm", target))
	fmt.Printf("Searching for files in source directory...\n")

	packageFiles := []string{
		filepath.Join(sourcePath, "install.sh"),
		filepath.Join(sourcePath, "uninstall.sh"),
		filepath.Join(sourcePath, "start.sh"),
		filepath.Join(sourcePath, "stop.sh"),
		filepath.Join(sourcePath, "payload.zip"),
	}

	for _, filePath := range packageFiles {
		if _, err := os.Stat(filePath); err != nil {
			fmt.Printf("Unable to find file: %s\n", filePath)
		}
	}

	os.RemoveAll(targetPath)

	fmt.Printf("Zipping files...\nDeployment target: %s\n", targetPath)
	util.ZipFiles(targetPath, packageFiles)

	fmt.Printf("Hashing file...\n")
	// Open the file for hashing
	file, err := os.Open(targetPath)
	if err != nil {
		fmt.Printf("Error opening target module: %v\n", err)
		return
	}
	defer file.Close()
	// Generate the checksum
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		fmt.Printf("Error getting file hash: %v\n", err)
	}

	var fileHash [16]uint8
	copy(fileHash[:], hash.Sum(nil)[:16])
	deploymentPacket := new(packets.DeploymentHeader)
	deploymentPacket.Initialize(fileHash)
	sendPacket(conn, localAddr, key, deploymentPacket)

	fmt.Printf("Deployment initiated\n")
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
	sendPacket(conn, localAddr, key, pingPkt)
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

func sendPacket(conn net.PacketConn, addr net.Addr, key [32]uint8, pkt packets.Packet) {
	data := authentication.EncryptPacket(pkt.Serialize(), key)
	_, err := conn.WriteTo(data, addr)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}