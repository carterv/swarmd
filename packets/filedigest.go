package packets

import "fmt"

// TODO: Fill out the packet fields
type FileDigestHeader struct {
	Common CommonHeader
}

// TODO: Initialize the packet
func (h *FileDigestHeader) Initialize() {
	h.Common.Initialize(uint16(CommonHeaderSize), h.PacketType())
}

// TODO: Serialize the packet
func (h *FileDigestHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, CommonHeaderSize)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())
	raw.CalculateChecksum()

	return raw
}

// TODO: Deserialize the packet
func (h *FileDigestHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}
	return true
}

// TODO: Build the string representation of the packet
func (h *FileDigestHeader) ToString() string {
	return ""
}

func (h *FileDigestHeader) PacketType() uint8 {
	return 3
}