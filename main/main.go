package main

import (
	"os"
	"swarmd/tasks"
	"flag"
)

func main() {
	hostPtr := flag.String("host", "", "The address of the bootstrapping host")
	portPtr := flag.Int("port", 51234, "The port to connect to on the bootstrapping host")
	//randomSeedPtr := flag.String("randomSeed", "", "The random seed to initial network encryption with")
	tasks.Run(*hostPtr, *portPtr)

	os.Exit(0)
}
