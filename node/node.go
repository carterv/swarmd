package node

import (
	"fmt"
	"net"
	"strings"
	"strconv"
)

type Node struct {
	Address string
	Port uint16
}

func (n Node) Message(msg string) {
	fmt.Printf("%s: %s\n", n.Address, msg)
}

func BuildNode(addr net.Addr) (Node, error) {
	addrParts := strings.Split(addr.String(), ":")
	address := strings.Join(addrParts[:len(addrParts)-1], ":")
	port, err := strconv.ParseUint(addrParts[len(addrParts)-1], 10, 16)
	return Node{
		Address:address,
		Port:uint16(port),
	}, err
}