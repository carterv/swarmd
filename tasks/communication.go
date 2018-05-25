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
		case <-time.After((period / 10) * time.Second):
			now := time.Now().Unix()
			history.Range(func(key, value interface{}) bool {
				if now - value.(int64) > int64(period.Seconds()) {
					history.Delete(key)
				}
				return true
			})
		}
	}
}

func Listener(killFlag *bool, conn net.PacketConn, key [32]byte, output chan packets.PeerPacket) {
	history := new(sync.Map)
	go historyMaintainer(history, 10*time.Second)
	for !*killFlag {
		// Read the raw byte stream
		buffer := make(packets.SerializedPacket, 2048)
		length, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			log.Fatal(err)
		}
		sourceNode, err := node.BuildNode(addr)
		if err != nil {
			log.Print("Error occurred while attempting to parse packet source, discarding")
			continue
		}
		nodePkt := packets.PeerPacket{Packet: nil, Source: sourceNode}
		// Decrypt the packet
		data := authentication.DecryptPacket(buffer[:length], key)
		if data == nil {
			log.Print("Error decrypting packet, discarding")
			continue
		}
		// Deserialize the data based off the data type
		//log.Printf("Recieved packet type: %d from %s:%d", data[2], sourceNode.Address, sourceNode.Port)
		packets.InitializePacket(&nodePkt.Packet, data[2])
		// Error handling
		if !nodePkt.Packet.Deserialize(data) {
			log.Print("Packet format does not match packet number")
			continue
		}
		if !nodePkt.Packet.IsValid() {
			log.Print("Invalid checksum")
			continue
		}
		// Ensure that this isn't a duplicate packet
		checksum := data.GetChecksum()
		now := time.Now().Unix()
		if _, ok := history.LoadOrStore(checksum, now); ok {
			continue
		}
		// Send to the master
		output <- nodePkt
	}
}

func Talker(killFlag *bool, conn net.PacketConn, key [32]byte, input chan packets.Packet,
	directInput chan packets.PeerPacket, peerMap *sync.Map) {
	for !*killFlag {
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

func SendToAll(conn net.PacketConn, key [32]byte, pkt packets.Packet, peers *sync.Map) {
	// Encrypt the packet
	data := authentication.EncryptPacket(pkt.Serialize(), key)
	peers.Range(func (key, value interface{}) bool {
		peer := key.(node.Node)
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.Address, peer.Port))
		if err != nil {
			log.Print(err)
			return true
		}
		conn.WriteTo(data, addr)
		return true
	})
}

func Talk(conn net.PacketConn, key [32]byte, pkt packets.Packet, peer node.Node) bool {
	// Encrypt the packet
	//log.Printf("Sending packet type %d to %s:%d", pkt.PacketType(), peer.Address, peer.Port)
	data := authentication.EncryptPacket(pkt.Serialize(), key)
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.Address, peer.Port))
	if err != nil {
		log.Print(err)
		return false
	}
	conn.WriteTo(data, addr)
	return true
}
