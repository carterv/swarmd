package packets

import "fmt"

type MessageHeader struct {
	Common  CommonHeader
	Message string
}

func (h *MessageHeader) Initialize(message string) {
	h.Message = message
	h.Common.Initialize(uint16(CommonHeaderSize+len(message)), h.PacketType())
}

func (h *MessageHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())
	copy(raw[CommonHeaderSize:], []uint8(h.Message))

	raw.CalculateChecksum()

	return raw
}

func (h *MessageHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	h.Message = string(raw[CommonHeaderSize:])

	return true
}

func (h *MessageHeader) ToString() string {
	return fmt.Sprintf("%sMessage: %s\n", h.Common.ToString(), h.Message)
}

func (h *MessageHeader) PacketType() uint8 {
	return 1
}