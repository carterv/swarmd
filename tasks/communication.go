package tasks

import (
	"swarmd/packets"
	"net"
	"swarmd/node"
	"log"
	"fmt"
	"sync"
	"time"
	"swarmd/authentication"
)

func historyMaintainer(history *sync.Map, period time.Duration) {
	for {
		select {
		case <-time.After(period / 10):
			now := time.Now()
			history.Range(func(key, value interface{}) bool {
				if now.Sub(value.(time.Time)) > period {
					history.Delete(key)
				}
				return true
			})
		}
	}
}

func Listener(conn net.PacketConn, key [32]byte, output chan packets.PeerPacket) {
	history := new(sync.Map)
	go historyMaintainer(history, 10*time.Second)
	for {
		// Read the raw byte stream
		buffer := make(packets.SerializedPacket, 2048)
		_, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			log.Fatal(err)
		}
		sourceNode, err := node.BuildNode(addr)
		if err != nil {
			fmt.Printf("Error occurred while attempting to parse packet source, discarding\n")
			continue
		}
		nodePkt := packets.PeerPacket{Packet: nil, Source: sourceNode}
		// Decrypt the packet
		data := authentication.DecryptPacket(buffer, key)
		// Deserialize the data based off the data type
		packets.InitializePacket(&nodePkt.Packet, data[2])
		// Error handling
		if !nodePkt.Packet.Deserialize(data) {
			fmt.Print("Packet format does not match packet number\n")
			continue
		}
		if !nodePkt.Packet.IsValid() {
			fmt.Print("Invalid checksum\n")
			continue
		}
		// Ensure that this isn't a duplicate packet
		checksum := data.GetChecksum()
		if _, ok := history.Load(checksum); ok {
			continue
		}
		history.Store(checksum, time.Now())
		// Group the source and packet
		output <- nodePkt
	}
}

func Talker(conn net.PacketConn, key [32]byte, input chan packets.Packet, directInput chan packets.PeerPacket, peerMap map[node.Node]int) {
	for {
		select {
		case pkt := <-input:
			// Broadcast a message to all peers
			SendToAll(conn, key, pkt, peerMap)
		case nodePkt := <-directInput:
			// Send a message to a single peer
			Talk(conn, key, nodePkt.Packet, nodePkt.Source)
		}
	}
}

func SendToAll(conn net.PacketConn, key [32]byte, pkt packets.Packet, peers map[node.Node]int) {
	// Encrypt the packet
	data := authentication.EncryptPacket(pkt.Serialize(), key)
	for peer := range peers {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.Address, peer.Port))
		if err != nil {
			log.Fatal(err)
			continue
		}
		conn.WriteTo(data, addr)
	}
}

func Talk(conn net.PacketConn, key [32]byte, pkt packets.Packet, peer node.Node) bool {
	// Encrypt the packet
	data := authentication.EncryptPacket(pkt.Serialize(), key)
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.Address, peer.Port))
	if err != nil {
		return false
	}
	conn.WriteTo(data, addr)
	return true
}
