package main

import (
	"os"
	"swarmd/node"
)

func main () {
	n := node.Node{Address: "192.168.1.1"}

	n.Message("Test")

	os.Exit(0)
}