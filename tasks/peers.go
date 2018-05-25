package tasks

import (
	"swarmd/node"
	"swarmd/packets"
	"time"
	"log"
	"math/rand"
	"sync"
)

const minPeers = 2

func PeerManager(killFlag *bool, bootstrapper *node.Node, outputDirected chan packets.PeerPacket, peerMap *sync.Map, peerChan chan node.Node) {
	threshold := uint8(minPeers)
	bootstrapAfter := time.After(0 * time.Second)
	pingAfter := time.After(120 * time.Second)
	statusAfter := time.After(0 * time.Second)
	peerCount := 0
	countPeers := func(key, value interface{}) bool { peerCount += 1; return true }
	for !*killFlag {
		select {
		case peer := <-peerChan:
			log.Printf("Accepting connection from %s:%d", peer.Address, peer.Port)
			peerMap.Store(peer, 0)
			peerCount = 0
			peerMap.Range(countPeers)
			log.Printf("Number of peers: %d", peerCount)
		case <-statusAfter:
			peerCount = 0
			peerMap.Range(countPeers)
			log.Printf("Number of peers: %d", peerCount)
			statusAfter = time.After(60 * time.Second)
		case <-bootstrapAfter:
			peerCount = 0
			peerMap.Range(countPeers)
			// Periodically send out a connection request with an increasing threshold if there are no peers
			if peerCount < int(threshold) && bootstrapper != nil {
				log.Print("Sending connection request to bootstrapper")
				pkt := new(packets.ConnectionRequestHeader)
				pkt.Initialize(threshold)
				outputDirected <- packets.PeerPacket{Packet: pkt, Source: *bootstrapper}
			}
			if peerCount >= int(threshold) {
				threshold = minPeers
				bootstrapAfter = time.After(120 * time.Second)
			} else if peerCount > 0 {
				threshold = minPeers
				bootstrapAfter = time.After(30 * time.Second)
			} else {
				if threshold < 10 {
					threshold += 1
				}
				bootstrapAfter = time.After(10 * time.Second)
			}
		case <-pingAfter:
			log.Print("Pinging peers:")
			// Ping routine
			deadPeers := make([]node.Node, 0)
			pkt := new(packets.MessageHeader)
			pkt.Initialize("__PING_REQ")
			// Ping peers that have responded recently
			peerMap.Range(func(key, value interface{}) bool {
				peer := key.(node.Node)
				pings := value.(int)
				log.Printf("\t%s:%d - %d", peer.Address, peer.Port, pings)
				if pings == 3 {
					deadPeers = append(deadPeers, peer)
				} else {
					outputDirected <- packets.PeerPacket{Packet: pkt, Source: peer}
					peerMap.Store(peer, pings+1)
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
	peerMap.Range(func(key, value interface{}) bool { peerCount += 1; return true })
	if peerCount < int(pkt.Threshold) {
		ack := new(packets.ConnectionAckHeader)
		ack.Initialize()
		peer := node.Node{Address: pkt.Requester, Port: pkt.RequesterPort}
		if _, ok := peerMap.Load(peer); !ok && !(peer.Address == self.Address && peer.Port == self.Port) {
			log.Printf("Acking connection to %s:%d", peer.Address, peer.Port)
			outputDirected <- packets.PeerPacket{Packet: ack, Source: peer}
			peerChan <- peer
			if pkt.Threshold > minPeers {
				pkt.Threshold = minPeers
			}
		} else if _, ok := peerMap.Load(peer); ok {
			log.Printf("Reestablishing connection to %s:%d", peer.Address, peer.Port)
			outputDirected <- packets.PeerPacket{Packet: ack, Source: peer}
			if pkt.Threshold > minPeers {
				pkt.Threshold = minPeers
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
