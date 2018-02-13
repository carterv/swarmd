package packets

import "fmt"

type FilePartHeader struct {
	Common CommonHeader
	PartNumber uint16
	Padding uint16
	Data []uint8
}

func (h *FilePartHeader) Initialize(PartNum uint16, Data []uint8) {
	var dataLength uint16
	if len(Data) < 1024 {
		dataLength = uint16(len(Data))
	} else {
		dataLength = 1024
	}
	h.PartNumber = PartNum
	h.Padding = 1024 - dataLength
	h.Data = make([]uint8, dataLength)
	copy(h.Data, Data)

	h.Common.Initialize(uint16(CommonHeaderSize)+4+dataLength, h.PacketType())
}

// TODO: Serialize the packet
func (h *FilePartHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, CommonHeaderSize)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())

	raw.CalculateChecksum()

	return raw
}

// TODO: Deserialize the packet
func (h *FilePartHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}


	return true
}

// TODO: Build the string representation
func (h *FilePartHeader) ToString() string {

	return ""
}

func (h *FilePartHeader) PacketType() uint8 {
	return 4
}