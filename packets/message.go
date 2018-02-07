package packets

import "fmt"

type MessageHeader struct {
	Common  CommonHeader
	Message string
}

func (h *MessageHeader) Initialize(message string) {
	h.Message = message
	h.Common.Initialize(uint16(CommonHeaderSize+len(message)), 1)
}

func (h *MessageHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, CommonHeaderSize+len(h.Message))

	copy(raw[CommonHeaderSize:], []uint8(h.Message))
	copy(raw[:CommonHeaderSize], h.Common.Serialize())

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
