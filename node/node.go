package node

import "fmt"

type Node struct {
	Address string
}

func (n Node) Message(msg string) {
	fmt.Printf("%s: %s\n", n.Address, msg)
}