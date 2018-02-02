package packets

import (
	"encoding/binary"
	"fmt"
)

type Packet interface {
	Serialize() []byte
	Deserialize(raw []byte) bool
	ToString() string
}

type CommonHeader struct {
	PacketLength uint16
	PacketType uint8
}

func (h *CommonHeader) Deserialize(raw []byte) bool {
	if len(raw) < 3 {
		return false
	}

	h.PacketLength = binary.BigEndian.Uint16(raw[:2])
	h.PacketType = raw[2]

	return true
}

func (h *CommonHeader) Serialize() []byte {
	raw := make([]byte, 3)

	binary.BigEndian.PutUint16(raw[:2], h.PacketLength)
	raw[2] = h.PacketType

	return raw
}

func (h *CommonHeader) ToString() string {
	return fmt.Sprintf("Length: %d\nType: 0x%02X\n", h.PacketLength, h.PacketType)
}
