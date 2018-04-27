package util

import (
	"os/user"
	"path/filepath"
	"os"
)

func GetBasePath() string {
	usr, _ := user.Current()
	basePath := filepath.Join(usr.HomeDir, ".swarmd/")

	// Make the directory if it doesn't exist
	os.MkdirAll(basePath, 0700)

	return basePath
}
