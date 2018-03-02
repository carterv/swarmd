package packets

import (
	"fmt"
	"encoding/binary"
	"encoding/hex"
)

type FileDigestHeader struct {
	Common   CommonHeader
	FileHash [16]uint8
	FileSize uint32
	FileName string
}

func (h *FileDigestHeader) Initialize(FileHash [16]uint8, FileSize uint32, FileName string) {
	h.FileHash = FileHash
	h.FileSize = FileSize
	h.FileName = FileName

	h.Common.Initialize(uint16(CommonHeaderSize+20+len(FileName)), h.PacketType())
}

func (h *FileDigestHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())
	copy(raw[CommonHeaderSize:CommonHeaderSize+16], h.FileHash[:])
	binary.BigEndian.PutUint32(raw[CommonHeaderSize+16:CommonHeaderSize+20], h.FileSize)
	copy(raw[CommonHeaderSize+20:h.Common.PacketLength], []uint8(h.FileName))

	raw.CalculateChecksum()

	return raw
}

func (h *FileDigestHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	copy(h.FileHash[:], raw[CommonHeaderSize:CommonHeaderSize+16])
	h.FileSize = binary.BigEndian.Uint32(raw[CommonHeaderSize+16:CommonHeaderSize+20])
	h.FileName = string(raw[CommonHeaderSize+20:h.Common.PacketLength])

	return true
}

func (h *FileDigestHeader) ToString() string {
	return fmt.Sprintf("%sFile Name: %s\nFile Size: %d\nFile Hash: %s\n", h.Common.ToString(), h.FileName,
		h.FileSize, hex.Dump(h.FileHash[:]))
}

func (h *FileDigestHeader) PacketType() uint8 {
	return PacketTypeFileDigestHeader
}

func (h *FileDigestHeader) IsValid() bool {
	return h.Common.IsValid()
}