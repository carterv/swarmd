package tasks

import (
	"swarmd/node"
	"swarmd/packets"
	"time"
	"log"
	"math/rand"
	"sync"
)

func PeerManager(killFlag *bool, bootstrapper *node.Node, outputDirected chan packets.PeerPacket, peerMap *sync.Map, peerChan chan node.Node) {
	threshold := uint8(3)
	bootstrapAfter := time.After(0 * time.Second)
	pingAfter := time.After(120 * time.Second)
	statusAfter := time.After(5 * time.Second)
	for !*killFlag {
		select {
		case peer := <-peerChan:
			log.Printf("Accepting connection from %s:%d", peer.Address, peer.Port)
			peerMap.Store(peer, 0)
		case <-statusAfter:
			peerCount := 0
			peerMap.Range(func(key, value interface{}) bool { peerCount += 1; return true })
			log.Printf("Number of peers: %d", peerCount)
			statusAfter = time.After(15 * time.Second)
		case <-bootstrapAfter:
			hasPeers := false
			peerMap.Range(func(key, value interface{}) bool { hasPeers = true; return false })
			// Periodically send out a connection request with an increasing threshold if there are no peers
			if !hasPeers && bootstrapper != nil {
				log.Print("Sending connection request to bootstrapper")
				pkt := new(packets.ConnectionRequestHeader)
				pkt.Initialize(threshold)
				outputDirected <- packets.PeerPacket{Packet: pkt, Source: *bootstrapper}
				threshold += 1
			} else {
				threshold = 3
			}
			bootstrapAfter = time.After(30 * time.Second)
		case <-pingAfter:
			// Ping routine
			deadPeers := make([]node.Node, 0)
			pkt := new(packets.MessageHeader)
			pkt.Initialize("__PING_REQ")
			// Ping peers that have responded recently
			peerMap.Range(func(key, value interface{}) bool {
				peer := key.(node.Node)
				pings := value.(int)
				if pings == 5 {
					deadPeers = append(deadPeers, peer)
				} else {
					outputDirected <- packets.PeerPacket{Packet: pkt, Source: peer}
					peerMap.Store(peer, pings + 1)
				}
				return true
			})
			// Remove dead peers (failed to respond to five pings)
			for _, peer := range deadPeers {
				peerMap.Delete(peer)
			}
			duration := time.Duration(90 + rand.Int()%60) // 120 +/- 25%
			pingAfter = time.After(duration * time.Second)
		}
	}
}

func HandleConnectionShare(self node.Node, pkt packets.ConnectionShareHeader, peerChan chan node.Node, peerMap *sync.Map, outputDirected chan packets.PeerPacket, outputGeneral chan packets.Packet) {
	peerCount := 0
	peerMap.Range(func (key, value interface{}) bool {peerCount += 1; return true})
	if peerCount < int(pkt.Threshold) {
		ack := new(packets.ConnectionAckHeader)
		ack.Initialize()
		peer := node.Node{Address: pkt.Requester, Port: pkt.RequesterPort}
		if _, ok := peerMap.Load(peer); !ok && !(peer.Address == self.Address && peer.Port == self.Port) {
			log.Printf("Acking connection to %s:%d", peer.Address, peer.Port)
			outputDirected <- packets.PeerPacket{Packet: ack, Source: peer}
			peerChan <- peer
			if pkt.Threshold > 3 {
				pkt.Threshold = 3
			}
		} else if _, ok := peerMap.Load(peer); ok {
			log.Printf("Reestablishing connection to %s:%d", peer.Address, peer.Port)
			outputDirected <- packets.PeerPacket{Packet: ack, Source: peer}
			if pkt.Threshold > 3 {
				pkt.Threshold = 3
			}
		}
	}
	if pkt.Threshold > 0 {
		//log.Printf("Sharing packet with threshold: %d", pkt.Threshold)
		outputGeneral <- &pkt
	}
}

func HandleConnectionRequest(request packets.PeerPacket, outputGeneral chan packets.Packet, outputDirected chan packets.PeerPacket, self node.Node) {
	log.Printf("Recieved connection request")
	sharePkt := new(packets.ConnectionShareHeader)
	sharePkt.Initialize(request.Source, request.Packet.(*packets.ConnectionRequestHeader).Threshold)
	outputDirected <- packets.PeerPacket{Source: self, Packet: sharePkt}
}
