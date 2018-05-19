package main

import (
	"flag"
	"time"
	"swarmd/tasks"
	"swarmd/authentication"
	"swarmd/node"
	"net"
	"swarmd/util"
	"fmt"
	"log"
	"swarmd/packets"
	"strings"
	"encoding/json"
	"net/http"
	"bytes"
)

func main() {
	hostPtr := flag.String("host", "", "The host to check into")
	portPtr := flag.Int("port", 0, "The port on which the check-in service is running")
	keyPtr := flag.String("key", "", "The key to use when communicating with the local node")
	localPortPtr := flag.Int("localPort", 51234, "The port on which the local service is running")

	flag.Parse()

	localAddress := tasks.GetOutboundIP()
	key := authentication.MakeKey(*keyPtr)

	localNode := node.Node{
		Address: localAddress.String(),
		Port:    uint16(*localPortPtr),
	}

	checkInServer := node.Node{
		Address: *hostPtr,
		Port:    uint16(*portPtr),
	}

	self := node.Node{
		Address: localAddress.String(),
		Port:    0,
	}

	localAddr := util.GetAddr(localNode)
	//checkInAddr := util.GetAddr(checkInServer)

	conn, err := net.ListenPacket("udp", fmt.Sprintf("[::]:%d", self.Port))
	if err != nil {
		log.Fatal(err)
	}
	loop(conn, key, localNode, localAddr, fmt.Sprintf("%s:%d", checkInServer.Address, checkInServer.Port))
}

func loop(conn net.PacketConn, key [32]uint8, localNode node.Node, localAddr net.Addr, checkInAddr string) {
	timer := time.After(0 * time.Second)
	for {
		select {
		case <-timer:
			// Get the list of peers
			listPkt := new(packets.MessageHeader)
			listPkt.Initialize("__LIST_PEERS")
			util.SendPacket(conn, localAddr, key, listPkt)
			responseBody := getResponse(conn, key)
			if !strings.HasPrefix(responseBody, "__LIST_RSP") {
				continue
			}
			peers := strings.TrimPrefix(responseBody, "__LIST_RSP")
			peerList := strings.Split(peers, ",")
			myAddress := fmt.Sprintf("%s:%d", localNode.Address, localNode.Port)
			values := map[string]interface{}{
				"peers": peerList,
				"self":  myAddress,
			}
			jsonValue, _ := json.Marshal(values)

			response, err := http.Post(fmt.Sprintf("http://%s/checkIn", checkInAddr), "application/json",
				bytes.NewBuffer(jsonValue))
			log.Print(response)
			if err != nil {
				log.Fatal(err)
			}
			timer = time.After(30 * time.Second)
		}
	}
}

func getResponse(conn net.PacketConn, key [32]uint8) string {
	// Wait for the response
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
	respPkt := (*pkt).(*packets.MessageHeader)
	return respPkt.Message
}
