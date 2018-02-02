package packets

import "fmt"

type MessageHeader struct {
	Common  *CommonHeader
	Message string
}

func (h *MessageHeader) Serialize() []byte {
	raw := make([]byte, 3+len(h.Message))

	copy(raw[0:3], h.Common.Serialize())
	copy(raw[3:], []byte(h.Message))

	return raw
}

func (h *MessageHeader) Deserialize(raw []byte) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	h.Message = string(raw[3:])

	return true
}

func (h *MessageHeader) Initialize(message string) {
	h.Message = message
	h.Common = &CommonHeader{
		PacketLength: uint16(3+len(message)),
		PacketType: 1,
	}
}

func (h *MessageHeader) ToString() string {
	return fmt.Sprintf("%sMessage: %s\n", h.Common.ToString(), h.Message)
}