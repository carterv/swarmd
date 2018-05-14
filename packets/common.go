package packets

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"swarmd/node"
	"log"
	"crypto/sha1"
	"time"
	"math/rand"
)

type Packet interface {
	Serialize() SerializedPacket
	Deserialize(raw SerializedPacket) bool
	ToString() string
	PacketType() uint8
	IsValid() bool
}

type PeerPacket struct {
	Packet Packet
	Source node.Node
}

const ChecksumSize = 4
const NonceSize = 20
const CommonHeaderSize = 3 + ChecksumSize + NonceSize


// Packet identifiers
const PacketTypeMessageHeader = 1
const PacketTypeManifestHeader = 2
const PacketTypeFileDigestHeader = 3
const PacketTypeFilePartHeader = 4
const PacketTypeFilePartRequestHeader = 5
const PacketTypeFileRequestHeader = 6
const PacketTypeDeployment = 7
const PacketTypeConnectionRequest = 8
const PacketTypeConnectionShare = 9
const PacketTypeConnectionAck = 10


func InitializePacket(packet *Packet, packetType uint8) {
	switch packetType {
	case PacketTypeMessageHeader:
		*packet = new(MessageHeader)
	case PacketTypeManifestHeader:
		*packet = new(ManifestHeader)
	case PacketTypeFileDigestHeader:
		*packet = new(FileDigestHeader)
	case PacketTypeFilePartHeader:
		*packet = new(FilePartHeader)
	case PacketTypeFilePartRequestHeader:
		*packet = new(FilePartRequestHeader)
	case PacketTypeFileRequestHeader:
		*packet = new(FileRequestHeader)
	case PacketTypeDeployment:
		*packet = new(DeploymentHeader)
	case PacketTypeConnectionRequest:
		*packet = new(ConnectionRequestHeader)
	case PacketTypeConnectionAck:
		*packet = new(ConnectionAckHeader)
	case PacketTypeConnectionShare:
		*packet = new(ConnectionShareHeader)
	default:
		log.Printf("Unknown packet type: %d", packetType)
	}
}

type SerializedPacket []uint8

type CommonHeader struct {
	PacketLength  uint16
	PacketType    uint8
	ValidChecksum bool
	Nonce [NonceSize]uint8
}
// Initializes a common header
func (h *CommonHeader) Initialize(PacketLength uint16, PacketType uint8) {
	h.PacketLength = PacketLength
	h.PacketType = PacketType
	// Add a nonce to distinguish instantiates of the same message to the history maintainer
	nonce := fmt.Sprintf("%d_%d", time.Now().Unix(), rand.Int())
	h.Nonce = sha1.Sum([]byte(nonce))
	h.ValidChecksum = true
}

// Pulls a CommonHeader out of a byte array
func (h *CommonHeader) Deserialize(raw SerializedPacket) bool {
	if len(raw) < CommonHeaderSize {
		return false
	}

	offset := 0
	h.PacketLength = binary.BigEndian.Uint16(raw[offset:offset+2])
	offset += 2
	h.PacketType = raw[offset]
	offset += 1
	copy(h.Nonce[:], raw[offset:offset+NonceSize])
	offset += NonceSize
	h.ValidChecksum = raw.VerifyChecksum(h.PacketLength)

	return true
}

// Writes a CommonHeader
func (h *CommonHeader) Serialize() SerializedPacket {
	var raw = make(SerializedPacket, CommonHeaderSize)

	offset := 0
	binary.BigEndian.PutUint16(raw[offset:offset+2], h.PacketLength)
	offset += 2
	raw[offset] = h.PacketType
	offset += 1
	copy(raw[offset:offset+NonceSize], h.Nonce[:])
	offset += NonceSize

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

func (h *CommonHeader) IsValid() bool {
	return h.ValidChecksum
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

func (s SerializedPacket) VerifyChecksum(length uint16) bool {
	if len(s) < CommonHeaderSize {
		return false
	}

	receivedChecksum := s.GetChecksum()
	copy(s[CommonHeaderSize-ChecksumSize:CommonHeaderSize], make(SerializedPacket, ChecksumSize))
	checksum := crc32.ChecksumIEEE(s[0:length])
	binary.BigEndian.PutUint32(s[CommonHeaderSize-ChecksumSize:CommonHeaderSize], checksum)

	return checksum == receivedChecksum
}

func (s SerializedPacket) GetChecksum() uint32 {
	return binary.BigEndian.Uint32(s[CommonHeaderSize-ChecksumSize : CommonHeaderSize])
}

func (s SerializedPacket) PutCommonHeader(common CommonHeader) uint16 {
	copy(s[:CommonHeaderSize], common.Serialize())
	return CommonHeaderSize
}

func (s SerializedPacket) PutUint16(offset uint16, val uint16) uint16 {
	binary.BigEndian.PutUint16(s[offset:offset+2], val)
	return offset + 2
}

func (s SerializedPacket) PutUint8(offset uint16, val uint8) uint16 {
	s[offset] = val
	return offset + 1
}

func (s SerializedPacket) PutArray(offset uint16, arr []uint8, length uint16) uint16 {
	copy(s[offset:offset+length], arr)
	return offset + length
}

