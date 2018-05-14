package tasks

import (
	"swarmd/node"
	"swarmd/packets"
	"time"
	"log"
	"math/rand"
)

func PeerManager(killFlag *bool, bootstrapper *node.Node, outputDirected chan packets.PeerPacket, peerMap map[node.Node]int, peerChan chan node.Node) {
	threshold := uint8(3)
	bootstrapAfter := time.After(0 * time.Second)
	initialBootstrap := true
	pingAfter := time.After(120 * time.Second)
	statusAfter := time.After(15 * time.Second)
	for !*killFlag {
		select {
		case peer := <-peerChan:
			peerMap[peer] = 0
		case <-statusAfter:
			log.Printf("Number of peers: %d", len(peerMap))
		case <-bootstrapAfter:
			// Periodically send out a connection request with an increasing threshold if there are no peers
			if len(peerMap) == 0 && bootstrapper != nil {
				log.Print("Sending connection request to bootstrapper")
				pkt := new(packets.ConnectionRequestHeader)
				pkt.Initialize(threshold)
				threshold += 1
				outputDirected <- packets.PeerPacket{Packet: pkt, Source: *bootstrapper}
			} else {
				threshold = 3
			}
			if initialBootstrap {
				bootstrapAfter = time.After(30 * time.Second)
				initialBootstrap = false
			}
		case <-pingAfter:
			// Ping routine
			deadPeers := make([]node.Node, 0)
			pkt := new(packets.MessageHeader)
			pkt.Initialize("__PING_REQ")
			// Ping peers that have responded recently
			for peer, pings := range peerMap {
				if pings == 5 {
					deadPeers = append(deadPeers, peer)
				} else {
					outputDirected <- packets.PeerPacket{Packet: pkt, Source: peer}
					peerMap[peer] = pings + 1
				}
			}
			// Remove dead peers (failed to respond to five pings)
			for _, peer := range deadPeers {
				delete(peerMap, peer)
			}
			duration := time.Duration(90 + rand.Int()%60) // 120 +/- 25%
			pingAfter = time.After(duration * time.Second)
		}
	}
}

func HandleConnectionShare(pkt packets.ConnectionShareHeader, peerChan chan node.Node, peerMap map[node.Node]int, outputDirected chan packets.PeerPacket, outputGeneral chan packets.Packet) {
	outputGeneral <- &pkt
	if len(peerMap) < int(pkt.Threshold) {
		ack := new(packets.ConnectionAckHeader)
		ack.Initialize()
		peer := node.Node{Address: pkt.Requester, Port: pkt.RequesterPort}
		outputDirected <- packets.PeerPacket{Packet: ack, Source: peer}
		peerChan <- peer
	}
}

func HandleConnectionRequest(request packets.PeerPacket, outputGeneral chan packets.Packet, outputDirected chan packets.PeerPacket, self node.Node) {
	log.Printf("Recieved connection request")
	sharePkt := new(packets.ConnectionShareHeader)
	sharePkt.Initialize(request.Source, request.Packet.(*packets.ConnectionRequestHeader).Threshold)
	outputGeneral <- sharePkt
	outputDirected <- packets.PeerPacket{Source: self, Packet: sharePkt}
}
