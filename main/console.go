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

	conn := setupConnection(key, self, localNode)

	startPrompt(conn, self, localNode)
}

func startPrompt(conn net.PacketConn, self node.Node, localNode node.Node) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		text, _ := reader.ReadString('\n')
		words := strings.Fields(text)
		if len(words) == 0 {
			continue
		}
		cmd := words[0]
		switch cmd {
		case "quit":
			return
		default:
			fmt.Printf("Invalid command: %s\n", cmd)
		}
	}
}

func setupConnection(key [32]byte, self node.Node, localNode node.Node) net.PacketConn {
	// Create a listening udp socket
	conn, err := net.ListenPacket("udp", fmt.Sprintf("[::]:%d", self.Port))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	// Ping the local node
	pingPkt := new(packets.MessageHeader)
	pingPkt.Initialize("__PING_REQ")
	data := authentication.EncryptPacket(pingPkt.Serialize(), key)
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", localNode.Address, localNode.Port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Pinging local node...")
	conn.WriteTo(data, addr)
	// Wait for response
	buffer := make(packets.SerializedPacket, 2048)
	length, _, err := conn.ReadFrom(buffer)
	if err != nil {
		log.Fatal(err)
	}
	// Deserialize/validate the response packet
	data = authentication.DecryptPacket(buffer[:length], key)
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
