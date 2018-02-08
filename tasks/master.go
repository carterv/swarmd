package tasks

import (
	"swarmd/packets"
	"fmt"
	"math/rand"
	"swarmd/node"
)

func Run() {
	inputChan := make(chan packets.Packet)
	outputChan := make(chan packets.Packet)
	peers := make(chan node.Node)
	i := 0

	go Listener(inputChan)
	go Talker(outputChan, peers)

	// Use self as peer for now
	peers <- node.Node{"localhost", 51234}

	for {
		select {
		case pkt := <-inputChan:
			switch pkt.PacketType() {
			case packets.MessageType:
				fmt.Print(pkt.ToString())
			}
		default:
			if i == 0 {
				r := rand.Int() % 100
				if r == 0 {
					var pkt packets.MessageHeader
					pkt.Initialize(fmt.Sprintf("Test message %d", i))
					outputChan <- &pkt
					i += 1
				}
			}
		}
	}
}
