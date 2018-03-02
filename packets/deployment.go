package packets

import (
	"fmt"
	"encoding/hex"
)

type DeploymentHeader struct {
	Common     CommonHeader
	FileHash     [16]uint8
}

func (h *DeploymentHeader) Initialize(FileHash [16]uint8) {
	h.FileHash = FileHash

	h.Common.Initialize(uint16(CommonHeaderSize)+16, h.PacketType())
}

func (h *DeploymentHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())
	copy(raw[CommonHeaderSize:CommonHeaderSize+16], h.FileHash[:])

	raw.CalculateChecksum()

	return raw
}

func (h *DeploymentHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}

	copy(h.FileHash[:], raw[CommonHeaderSize:CommonHeaderSize+16])

	return true
}

func (h *DeploymentHeader) ToString() string {
	return fmt.Sprintf("%sFile Hash: %s\n", h.Common.ToString(), hex.Dump(h.FileHash[:]))
}

func (h *DeploymentHeader) PacketType() uint8 {
	return PacketTypeDeployment
}

func (h *DeploymentHeader) IsValid() bool {
	return h.Common.IsValid()
}