package main

import (
	"os"
	"swarmd/tasks"
)

func main () {
	tasks.Run()

	os.Exit(0)
}