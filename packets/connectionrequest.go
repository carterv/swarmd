package packets

import (
	"fmt"
)

type ConnectionRequestHeader struct {
	Common    CommonHeader
	Threshold uint8
}

func (h *ConnectionRequestHeader) Initialize(Threshold uint8) {
	h.Threshold = Threshold

	h.Common.Initialize(uint16(CommonHeaderSize)+8, h.PacketType())
}

func (h *ConnectionRequestHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	offset := raw.PutCommonHeader(h.Common)
	offset = raw.PutUint8(offset, h.Threshold)

	raw.CalculateChecksum()

	return raw
}

func (h *ConnectionRequestHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	h.Threshold = raw[CommonHeaderSize]

	return true
}

func (h *ConnectionRequestHeader) ToString() string {
	return fmt.Sprintf("%Threshold: %d\n", h.Common.ToString(), h.Threshold)
}

func (h *ConnectionRequestHeader) PacketType() uint8 {
	return PacketTypeConnectionRequest
}

func (h *ConnectionRequestHeader) IsValid() bool {
	return h.Common.IsValid()
}
