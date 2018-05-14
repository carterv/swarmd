package util

import (
	"os/user"
	"path/filepath"
	"os"
	"archive/zip"
	"io"
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