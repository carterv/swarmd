package packets

import (
	"fmt"
	"encoding/binary"
	"encoding/hex"
)

type FilePartHeader struct {
	Common     CommonHeader
	FileHash     [16]uint8
	PartNumber uint16
	Padding    uint16
	Data       []uint8
}

func (h *FilePartHeader) Initialize(FileHash [16]uint8, PartNumber uint16, Data []uint8) {
	var dataLength uint16
	if len(Data) < 1024 {
		dataLength = uint16(len(Data))
	} else {
		dataLength = 1024
	}
	h.FileHash = FileHash
	h.PartNumber = PartNumber
	h.Padding = 1024 - dataLength
	h.Data = make([]uint8, dataLength)
	copy(h.Data, Data)

	h.Common.Initialize(uint16(CommonHeaderSize)+20+dataLength, h.PacketType())
}

func (h *FilePartHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())
	copy(raw[CommonHeaderSize:CommonHeaderSize+16], h.FileHash[:])
	binary.BigEndian.PutUint16(raw[CommonHeaderSize+16:CommonHeaderSize+18], h.PartNumber)
	binary.BigEndian.PutUint16(raw[CommonHeaderSize+18:CommonHeaderSize+20], h.Padding)
	copy(raw[CommonHeaderSize+20:], h.Data)

	raw.CalculateChecksum()

	return raw
}

func (h *FilePartHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	copy(h.FileHash[:], raw[CommonHeaderSize:CommonHeaderSize+16])
	h.PartNumber = binary.BigEndian.Uint16(raw[CommonHeaderSize+16:CommonHeaderSize+18])
	h.Padding = binary.BigEndian.Uint16(raw[CommonHeaderSize+18:CommonHeaderSize+20])
	h.Data = make([]uint8, 1024-h.Padding)
	copy(h.Data, raw[CommonHeaderSize+20:h.Common.PacketLength])

	return true
}

func (h *FilePartHeader) ToString() string {
	return fmt.Sprintf("%sFile Hash: %s\nPartNumber: %d\nPartSize: %d\n", h.Common.ToString(),
		hex.Dump(h.FileHash[:]), h.PartNumber, 1024-h.Padding)
}

func (h *FilePartHeader) PacketType() uint8 {
	return 4
}
