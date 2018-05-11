package packets

import (
	"fmt"
	"crypto/sha1"
	"time"
	"math/rand"
)

const nonceSize = 20

type MessageHeader struct {
	Common  CommonHeader
	Nonce [nonceSize]uint8
	Message string
}

// TODO: Max this out at 1024 characters in a message
func (h *MessageHeader) Initialize(message string) {
	h.Message = message
	// Add a nonce to distinguish instantiates of the same message to the history maintainer
	nonce := fmt.Sprintf("%d_%d", time.Now().Unix(), rand.Int())
	h.Nonce = sha1.Sum([]byte(nonce))
	h.Common.Initialize(uint16(CommonHeaderSize+nonceSize+len(message)), h.PacketType())
}

func (h *MessageHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())
	offset := CommonHeaderSize
	copy(raw[offset:offset+nonceSize], h.Nonce[:])
	offset += nonceSize
	copy(raw[offset:], []uint8(h.Message))

	raw.CalculateChecksum()

	return raw
}

func (h *MessageHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	offset := CommonHeaderSize
	copy(h.Nonce[:], raw[offset:offset+nonceSize])
	offset += nonceSize
	h.Message = string(raw[offset:])

	return true
}

func (h *MessageHeader) ToString() string {
	return fmt.Sprintf("%sMessage: %s\n", h.Common.ToString(), h.Message)
}

func (h *MessageHeader) PacketType() uint8 {
	return PacketTypeMessageHeader
}

func (h *MessageHeader) IsValid() bool {
	return h.Common.IsValid()
}