package node

import "fmt"

type Node struct {
	Address string
	Port uint16
}

func (n Node) Message(msg string) {
	fmt.Printf("%s: %s\n", n.Address, msg)
}

