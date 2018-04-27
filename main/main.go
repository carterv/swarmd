package main

import (
	"os"
	"swarmd/tasks"
	"flag"
)

func main() {
	hostPtr := flag.String("host", "", "The address of the bootstrapping host")
	portPtr := flag.Int("port", 51234, "The port to connect to on the bootstrapping host")
	key := flag.String("key", "", "The encryption key")
	tasks.Run(*hostPtr, *portPtr, key)

	os.Exit(0)
}
