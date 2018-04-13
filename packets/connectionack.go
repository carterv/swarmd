package packets

import (
	"fmt"
)

type ConnectionAckHeader struct {
	Common     CommonHeader
}

func (h *ConnectionAckHeader) Initialize() {
	h.Common.Initialize(uint16(CommonHeaderSize), h.PacketType())
}

func (h *ConnectionAckHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	raw.PutCommonHeader(h.Common)

	raw.CalculateChecksum()

	return raw
}

func (h *ConnectionAckHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	return true
}

func (h *ConnectionAckHeader) ToString() string {
	return fmt.Sprintf("%s\n", h.Common.ToString())
}

func (h *ConnectionAckHeader) PacketType() uint8 {
	return PacketTypeConnectionAck
}

func (h *ConnectionAckHeader) IsValid() bool {
	return h.Common.IsValid()
}