package tasks

import (
	"swarmd/packets"
	"net"
	"swarmd/node"
	"log"
	"fmt"
	"encoding/hex"
	"sync"
	"time"
)

func historyMaintainer(history *sync.Map, period time.Duration) {
	for {
		select {
		case <-time.After(period/10):
			now := time.Now()
			history.Range(func (key, value interface{}) bool {
				if now.Sub(value.(time.Time)) > period {
					history.Delete(key)
				}
				return true
			})
		}
	}
}

func Listener(conn net.PacketConn, output chan packets.PeerPacket) {
	history := new(sync.Map)
	go historyMaintainer(history, 10 * time.Second)
	for {
		// Read the raw byte stream
		data := make(packets.SerializedPacket, 2048)
		_, addr, err := conn.ReadFrom(data)
		if err != nil {
			log.Fatal(err)
		}
		sourceNode, err := node.BuildNode(addr)
		if err != nil {
			fmt.Printf("Error occurred while attempting to parse packet source, discarding\n")
			continue
		}
		nodePkt := packets.PeerPacket{Packet: nil, Source: sourceNode}
		// Deserialize the data based off the data type
		switch data[2] {
		case packets.PacketTypeMessageHeader:
			nodePkt.Packet = new(packets.MessageHeader)
		case packets.PacketTypeManifestHeader:
			nodePkt.Packet = new(packets.ManifestHeader)
		case packets.PacketTypeFileDigestHeader:
			nodePkt.Packet = new(packets.FileDigestHeader)
		case packets.PacketTypeFilePartHeader:
			nodePkt.Packet = new(packets.FilePartHeader)
		case packets.PacketTypeFilePartRequestHeader:
			nodePkt.Packet = new(packets.FilePartRequestHeader)
		case packets.PacketTypeFileRequestHeader:
			nodePkt.Packet = new(packets.FileRequestHeader)
		case packets.PacketTypeDeployment:
			nodePkt.Packet = new(packets.DeploymentHeader)
		default:
			fmt.Printf("Unknown packet: \n%s\n", hex.Dump(data))
		}
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

func Talker(conn net.PacketConn, input chan packets.Packet, directInput chan packets.PeerPacket, peerChan chan node.Node) {
	peers := make([]node.Node, 0)
	for {
		select {
		case pkt := <-input:
			SendToAll(conn, pkt, peers)
		case nodePkt := <-directInput:
			Talk(conn, nodePkt.Packet, nodePkt.Source)
		case peer := <-peerChan:
			peers = append(peers, peer)
		}
	}
}

func SendToAll(conn net.PacketConn, pkt packets.Packet, peers []node.Node) {
	for _, peer := range peers {
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.Address, peer.Port))
		if err != nil {
			panic(err)
			continue
		}
		conn.WriteTo(pkt.Serialize(), addr)
	}
}

func Talk(conn net.PacketConn, pkt packets.Packet, peer node.Node) bool {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", peer.Address, peer.Port))
	if err != nil {
		return false
	}
	conn.WriteTo(pkt.Serialize(), addr)
	return true
}
