package tasks

import (
	"swarmd/node"
	"swarmd/packets"
	"path/filepath"
	"os/user"
	"os"
	"crypto/md5"
	"io"
)

type FileDigest struct {
	FileSize         uint32
	RelativeFilePath string
}

type FileManifest map[[16]uint8]FileDigest

func GetSharePath() string {
	usr, _ := user.Current()
	sharePath := filepath.Join(usr.HomeDir, ".swarmd/share/")

	// Make the share directory if it doesn't exist
	os.MkdirAll(sharePath, 0700)

	return sharePath

}

func GetFileManifest() FileManifest {
	sharePath := GetSharePath()
	files := make(map[[16]uint8]FileDigest)

	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer file.Close()

		hash := md5.New()

		if _, err := io.Copy(hash, file); err != nil {
			return nil
		}

		relPath, err := filepath.Rel(sharePath, path)
		if err != nil {
			return nil
		}

		f := FileDigest{
			RelativeFilePath: relPath,
			FileSize:         uint32(info.Size()),
		}

		var checksum [16]uint8
		copy(checksum[:], hash.Sum(nil)[:16])

		files[checksum] = f

		return nil
	}

	filepath.Walk(sharePath, walkFunc)

	return files
}

func FileShare(output chan packets.Packet, input chan node.PeerPacket) {
	for {
		select {
		case nodePkt := <-input:
			switch nodePkt.Packet.PacketType() {
			case packets.PacketTypeManifestHeader:
				// TODO: Compare the manifest to the local manifest and request digest headers from peers
			case packets.PacketTypeFileDigestHeader:
				go FileDownloader(*(nodePkt.Packet.(*packets.FileDigestHeader)))
			case packets.PacketTypeFilePartHeader:
				// TODO: Extract the data and save it as a file part
			case packets.PacketTypeFilePartRequestHeader:
				// TODO: Respond with the contents of the file part.
			}
		}
	}
}

func FileDownloader(fileInfo packets.FileDigestHeader) {
	// TODO: Query peers for file parts
}
