package packets

import (
	"fmt"
	"swarmd/node"
	"encoding/binary"
)

type ConnectionShareHeader struct {
	Common          CommonHeader
	RequesterLength uint16
	Requester       string
	RequesterPort   uint16
	Threshold uint8
}

func (h *ConnectionShareHeader) Initialize(Requester node.Node, Threshold uint8) {
	dataLength := 0
	h.RequesterLength = uint16(len(Requester.Address))
	dataLength += 2
	h.Requester = Requester.Address
	dataLength += len(Requester.Address)
	h.RequesterPort = Requester.Port
	dataLength += 2
	h.Threshold = Threshold
	dataLength += 1

	h.Common.Initialize(uint16(CommonHeaderSize+dataLength), h.PacketType())
}

func (h *ConnectionShareHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	offset := raw.PutCommonHeader(h.Common)
	offset = raw.PutUint16(offset, h.RequesterLength)
	offset = raw.PutArray(offset, []uint8(h.Requester), h.RequesterLength)
	offset = raw.PutUint16(offset, h.RequesterPort)
	offset = raw.PutUint8(offset, h.Threshold)

	raw.CalculateChecksum()

	return raw
}

func (h *ConnectionShareHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	offset := CommonHeaderSize
	h.RequesterLength = binary.BigEndian.Uint16(raw[offset:offset+2])
	offset += 2
	h.Requester = string(raw[offset:offset+int(h.RequesterLength)])
	offset += int(h.RequesterLength)
	h.RequesterPort = binary.BigEndian.Uint16(raw[offset:offset+2])
	offset += 2
	h.Threshold = raw[offset]
	offset += 1

	return true
}

func (h *ConnectionShareHeader) ToString() string {
	return fmt.Sprintf("%sRequester: %s:%d\n", h.Common.ToString(), h.Requester, h.RequesterPort)
}

func (h *ConnectionShareHeader) PacketType() uint8 {
	return PacketTypeConnectionShare
}

func (h *ConnectionShareHeader) IsValid() bool {
	return h.Common.IsValid()
}
