package tasks

import (
	"swarmd/node"
	"swarmd/packets"
	"time"
	"log"
	"math/rand"
)

const minPeers = 2

func PeerManager(config *commonStruct, bootstrapper *node.Node) {
	threshold := uint8(minPeers)
	bootstrapAfter := time.After(0 * time.Second)
	pingAfter := time.After(120 * time.Second)
	statusAfter := time.After(0 * time.Second)
	peerCount := 0
	countPeers := func(key, value interface{}) bool { peerCount += 1; return true }
	for !*config.KillFlag {
		select {
		case peer := <-config.Peers:
			log.Printf("Accepting connection from %s:%d", peer.Address, peer.Port)
			config.PeerMap.Store(peer, 0)
			peerCount = 0
			config.PeerMap.Range(countPeers)
			log.Printf("Number of peers: %d", peerCount)
		case <-statusAfter:
			peerCount = 0
			config.PeerMap.Range(countPeers)
			log.Printf("Number of peers: %d", peerCount)
			statusAfter = time.After(60 * time.Second)
		case <-bootstrapAfter:
			peerCount = 0
			config.PeerMap.Range(countPeers)
			// Periodically send out a connection request with an increasing threshold if there are no peers
			if peerCount < int(threshold) && bootstrapper != nil {
				log.Print("Sending connection request to bootstrapper")
				pkt := new(packets.ConnectionRequestHeader)
				pkt.Initialize(threshold)
				config.Output <- packets.PeerPacket{Packet: pkt, Source: *bootstrapper}
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
			config.PeerMap.Range(func(key, value interface{}) bool {
				peer := key.(node.Node)
				pings := value.(int)
				log.Printf("\t%s:%d - %d", peer.Address, peer.Port, pings)
				if pings == 3 {
					deadPeers = append(deadPeers, peer)
				} else {
					config.Output <- packets.PeerPacket{Packet: pkt, Source: peer}
					config.PeerMap.Store(peer, pings+1)
				}
				return true
			})
			// Remove dead peers (failed to respond to five pings)
			for _, peer := range deadPeers {
				config.PeerMap.Delete(peer)
			}
			duration := time.Duration(90 + rand.Int()%60) // 120 +/- 25%
			pingAfter = time.After(duration * time.Second)
		}
	}
}

func HandleConnectionShare(config *commonStruct, self node.Node, pkt packets.ConnectionShareHeader) {
	peerCount := 0
	config.PeerMap.Range(func(key, value interface{}) bool { peerCount += 1; return true })
	if peerCount < int(pkt.Threshold) {
		ack := new(packets.ConnectionAckHeader)
		ack.Initialize()
		peer := node.Node{Address: pkt.Requester, Port: pkt.RequesterPort}
		if _, ok := config.PeerMap.Load(peer); !ok && !(peer.Address == self.Address && peer.Port == self.Port) {
			log.Printf("Acking connection to %s:%d", peer.Address, peer.Port)
			config.Output <- packets.PeerPacket{Packet: ack, Source: peer}
			config.Peers <- peer
			if pkt.Threshold > minPeers {
				pkt.Threshold = minPeers
			}
		} else if _, ok := config.PeerMap.Load(peer); ok {
			log.Printf("Reestablishing connection to %s:%d", peer.Address, peer.Port)
			config.Output <- packets.PeerPacket{Packet: ack, Source: peer}
			if pkt.Threshold > minPeers {
				pkt.Threshold = minPeers
			}
		}
	}
	if pkt.Threshold > 0 {
		//log.Printf("Sharing packet with threshold: %d", pkt.Threshold)
		config.Broadcast <- &pkt
	}
}

func HandleConnectionRequest(config *commonStruct, request packets.PeerPacket, self node.Node) {
	log.Printf("Recieved connection request")
	sharePkt := new(packets.ConnectionShareHeader)
	sharePkt.Initialize(request.Source, request.Packet.(*packets.ConnectionRequestHeader).Threshold)
	config.Output <- packets.PeerPacket{Source: self, Packet: sharePkt}
}
