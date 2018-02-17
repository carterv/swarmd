package main

import (
	"os"
	"swarmd/tasks"
	"fmt"
	"encoding/hex"
)

func main() {
	//tasks.Run()

	manifest := tasks.GetFileManifest()

	for checksum, fileDigest := range manifest {
		fmt.Printf("%s: %d bytes\n%s", fileDigest.RelativeFilePath, fileDigest.FileSize, hex.Dump(checksum[:]))
	}

	os.Exit(0)
}
