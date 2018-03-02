package packets

type FileDigest struct {
	FileSize         uint32
	RelativeFilePath string
}

type FileManifest map[[16]uint8]FileDigest

type ManifestHeader struct {
	Common     CommonHeader
	FileHashes [][16]uint8
}

func (h *ManifestHeader) Initialize(manifest FileManifest) {
	dataLength := 0

	for checksum := range manifest {
		dataLength += 16
		h.FileHashes = append(h.FileHashes, checksum)
	}

	h.Common.Initialize(uint16(CommonHeaderSize+dataLength), h.PacketType())
}

func (h *ManifestHeader) Serialize() SerializedPacket {
	raw := make(SerializedPacket, h.Common.PacketLength)

	copy(raw[:CommonHeaderSize], h.Common.Serialize())
	for i, hash := range h.FileHashes {
		copy(raw[CommonHeaderSize+i*16:CommonHeaderSize+(i+1)*16], hash[:])
	}

	raw.CalculateChecksum()

	return raw
}

// TODO: Test this
func (h *ManifestHeader) Deserialize(raw SerializedPacket) bool {
	if !h.Common.Deserialize(raw) {
		return false
	}
	h.FileHashes = make([][16]uint8, 0)

	numHashes := (h.Common.PacketLength - CommonHeaderSize) / 16
	for i := uint16(0); i < numHashes; i++ {
		var hash [16]uint8
		copy(hash[:], raw[CommonHeaderSize+i*16:CommonHeaderSize+(i+1)*16])
		h.FileHashes = append(h.FileHashes, hash)
	}

	return true
}

// TODO: Build the string
func (h *ManifestHeader) ToString() string {
	return ""
}

func (h *ManifestHeader) PacketType() uint8 {
	return PacketTypeManifestHeader
}

func (h *ManifestHeader) IsValid() bool {
	return h.Common.IsValid()
}
