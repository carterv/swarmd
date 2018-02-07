package packets

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

type Packet interface {
	Serialize() SerializedPacket
	Deserialize(raw SerializedPacket) bool
	ToString() string
}

const ChecksumSize = 4
const CommonHeaderSize = 3 + ChecksumSize

type SerializedPacket []uint8

type CommonHeader struct {
	PacketLength  uint16
	PacketType    uint8
	ValidChecksum bool
}

// Initializes a common header
func (h *CommonHeader) Initialize(PacketLength uint16, PacketType uint8) {
	h.PacketLength = PacketLength
	h.PacketType = PacketType
	h.ValidChecksum = true
}

// Pulls a CommonHeader out of a byte array
func (h *CommonHeader) Deserialize(raw SerializedPacket) bool {
	if len(raw) < CommonHeaderSize {
		return false
	}

	h.PacketLength = binary.BigEndian.Uint16(raw[:2])
	h.PacketType = raw[2]
	h.ValidChecksum = raw.VerifyChecksum()

	return true
}

// Writes a CommonHeader
func (h *CommonHeader) Serialize() SerializedPacket {
	var raw = make(SerializedPacket, 3+ChecksumSize)

	binary.BigEndian.PutUint16(raw[:2], h.PacketLength)
	raw[2] = h.PacketType

	return raw
}

// Prints the string representation of a header
func (h *CommonHeader) ToString() string {
	s := ""
	s += fmt.Sprintf("Length: %d\n", h.PacketLength)
	s += fmt.Sprintf("Type: 0x%02X\n", h.PacketType)
	s += fmt.Sprintf("Valid Checksum: %t\n", h.ValidChecksum)
	return s
}

func (s SerializedPacket) CalculateChecksum() bool {
	if len(s) < CommonHeaderSize {
		return false
	}

	copy(s[CommonHeaderSize-ChecksumSize:CommonHeaderSize], make(SerializedPacket, ChecksumSize))
	checksum := crc32.ChecksumIEEE(s)
	binary.BigEndian.PutUint32(s[CommonHeaderSize-ChecksumSize:CommonHeaderSize], checksum)

	return true
}

func (s SerializedPacket) VerifyChecksum() bool {
	if len(s) < CommonHeaderSize {
		return false
	}

	receivedChecksum := binary.BigEndian.Uint32(s[CommonHeaderSize-ChecksumSize:CommonHeaderSize])
	copy(s[CommonHeaderSize-ChecksumSize:CommonHeaderSize], make(SerializedPacket, ChecksumSize))
	checksum := crc32.ChecksumIEEE(s)

	return checksum == receivedChecksum
}
