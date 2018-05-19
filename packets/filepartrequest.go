package packets

import (
	"fmt"
	"encoding/binary"
	"encoding/hex"
)

type FilePartRequestHeader struct {
	Common     CommonHeader
	FileHash   [16]uint8
	PartNumber uint16
}

func (h *FilePartRequestHeader) Initialize(FileHash [16]uint8, PartNumber uint16) {
	h.FileHash = FileHash
	h.PartNumber = PartNumber

	h.Common.Initialize(uint16(CommonHeaderSize)+18, h.PacketType())
}

func (h *FilePartRequestHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	offset := raw.PutCommonHeader(h.Common)
	offset = raw.PutArray(offset, h.FileHash[:], uint16(len(h.FileHash)))
	offset = raw.PutUint16(offset, h.PartNumber)

	raw.CalculateChecksum()

	return raw
}

func (h *FilePartRequestHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	copy(h.FileHash[:], raw[CommonHeaderSize:CommonHeaderSize+16])
	h.PartNumber = binary.BigEndian.Uint16(raw[CommonHeaderSize+16 : h.Common.PacketLength])

	return true
}

func (h *FilePartRequestHeader) ToString() string {
	return fmt.Sprintf("%sFile Hash: %s\nPartNumber: %d\n", h.Common.ToString(),
		hex.Dump(h.FileHash[:]), h.PartNumber)
}

func (h *FilePartRequestHeader) PacketType() uint8 {
	return PacketTypeFilePartRequestHeader
}

func (h *FilePartRequestHeader) IsValid() bool {
	return h.Common.IsValid()
}
