package util

import (
	"os/user"
	"path/filepath"
	"os"
)

func GetBasePath() string {
	usr, _ := user.Current()
	basePath := filepath.Join(usr.HomeDir, ".swarmd/")

	// Temporary stuff for testing on local machine
	portStr, present := os.LookupEnv("SWARMD_LOCAL_PORT")
	if !present {
		portStr = "51234"
	}
	basePath = filepath.Join(basePath, portStr)

	// Make the directory if it doesn't exist
	os.MkdirAll(basePath, 0700)

	return basePath
}
