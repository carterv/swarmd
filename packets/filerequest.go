package packets

import (
	"fmt"
	"encoding/binary"
	"encoding/hex"
	"swarmd/node"
)

type FileRequestHeader struct {
	Common          CommonHeader
	FileHash        [16]uint8
	RequesterLength uint16
	Requester       string
	RequesterPort   uint16
}

func (h *FileRequestHeader) Initialize(FileHash [16]uint8, self node.Node) {
	dataLength := 0
	h.FileHash = FileHash
	dataLength += 16
	h.RequesterLength = uint16(len(self.Address))
	dataLength += 2
	h.Requester = self.Address
	dataLength += len(self.Address)
	h.RequesterPort = self.Port
	dataLength += 2

	h.Common.Initialize(uint16(CommonHeaderSize+dataLength), h.PacketType())
}

func (h *FileRequestHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	offset := raw.PutCommonHeader(h.Common)
	offset = raw.PutArray(CommonHeaderSize, h.FileHash[:], uint16(len(h.FileHash)))
	offset = raw.PutUint16(offset, h.RequesterLength)
	offset = raw.PutArray(offset, []uint8(h.Requester), h.RequesterLength)
	offset = raw.PutUint16(offset, h.RequesterPort)

	raw.CalculateChecksum()

	return raw
}

func (h *FileRequestHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	offset := CommonHeaderSize
	copy(h.FileHash[:], raw[offset:offset+16])
	offset += 16
	h.RequesterLength = binary.BigEndian.Uint16(raw[offset:offset+2])
	offset += 2
	h.Requester = string(raw[offset:offset+int(h.RequesterLength)])
	offset += int(h.RequesterLength)
	h.RequesterPort = binary.BigEndian.Uint16(raw[offset:offset+2])
	offset += 2

	return true
}

func (h *FileRequestHeader) ToString() string {
	return fmt.Sprintf("%sRequester: %s:%d\nFile Hash: %s\n", h.Common.ToString(), h.Requester, h.RequesterPort,
		hex.Dump(h.FileHash[:]))
}

func (h *FileRequestHeader) PacketType() uint8 {
	return PacketTypeFileRequestHeader
}

func (h *FileRequestHeader) IsValid() bool {
	return h.Common.IsValid()
}

func (h *FileRequestHeader) GetRequester() node.Node {
	return node.Node{Address: h.Requester, Port: h.RequesterPort}
}
