package util

import (
	"os/user"
	"path/filepath"
	"os"
	"archive/zip"
	"io"
	"swarmd/node"
	"net"
	"fmt"
	"log"
	"swarmd/packets"
	"swarmd/authentication"
)

func GetBasePath() string {
	usr, _ := user.Current()
	basePath := filepath.Join(usr.HomeDir, ".swarmd/")

	// Temporary stuff for testing on local machine
	debug, present := os.LookupEnv("SWARMD_DEBUG")
	if present && debug == "true" {
		portStr, present := os.LookupEnv("SWARMD_LOCAL_PORT")
		if !present {
			portStr = "51234"
		}
		basePath = filepath.Join(basePath, portStr)
	}

	// Make the directory if it doesn't exist
	os.MkdirAll(basePath, 0700)

	return basePath
}

func ZipFiles(filename string, files []string) error {
	newfile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer newfile.Close()

	zipWriter := zip.NewWriter(newfile)
	defer zipWriter.Close()

	// Add files to zip
	for _, file := range files {

		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()

		// Get the file information
		info, err := zipfile.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Change to deflate to gain better compression
		// see http://golang.org/pkg/archive/zip/#pkg-constants
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, zipfile)
		if err != nil {
			return err
		}
	}
	return nil
}

func Unzip(src string, dest string) ([]string, error) {
	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}
		defer rc.Close()

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)
		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {

			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)

		} else {

			// Make File
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return filenames, err
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return filenames, err
			}

			_, err = io.Copy(outFile, rc)

			// Close the file without defer to close before next iteration of loop
			outFile.Close()

			if err != nil {
				return filenames, err
			}

		}
	}
	return filenames, nil
}

func GetAddr(n node.Node) net.Addr {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", n.Address, n.Port))
	if err != nil {
		log.Fatal(err)
	}
	return addr
}

func SendPacket(conn net.PacketConn, addr net.Addr, key [32]uint8, pkt packets.Packet) {
	data := authentication.EncryptPacket(pkt.Serialize(), key)
	_, err := conn.WriteTo(data, addr)
	if err != nil {
		fmt.Printf("%v\n", err)
	}
}