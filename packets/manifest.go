package packets

import "fmt"

// TODO: Fill out the packet fields
type ManifestHeader struct {
	Common CommonHeader
}

func (h *ManifestHeader) Initialize() {
	h.Common.Initialize(uint16(CommonHeaderSize), h.PacketType())
}

// TODO: Serialize the packet
func (h *ManifestHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, CommonHeaderSize)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())

	raw.CalculateChecksum()

	return raw
}

// TODO: Deserialize the packet
func (h *ManifestHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	return true
}

// TODO: Build the string
func (h *ManifestHeader) ToString() string {
	return ""
}

func (h *ManifestHeader) PacketType() uint8 {
	return 2
}