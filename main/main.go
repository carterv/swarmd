package main

import (
	"os"
	"swarmd/tasks"
	"flag"
	"log"
)

func main() {
	hostPtr := flag.String("host", "", "The address of the bootstrapping host")
	portPtr := flag.Int("port", 51234, "The port to connect to on the bootstrapping host")
	keyPtr := flag.String("key", "", "The encryption key")
	flag.Parse()
	log.Printf("Starting node with configuration: ")
	if *hostPtr != "" {
		log.Printf("\tBootstrap node: %s:%d", *hostPtr, *portPtr)
	}

	killFlag := false

	tasks.Run(&killFlag, *hostPtr, *portPtr, *keyPtr)

	os.Exit(0)
}
