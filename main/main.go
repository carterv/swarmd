package main

import (
	"os"
	"swarmd/packets"
	"fmt"
	"encoding/hex"
)

func main () {
	var msg packets.MessageHeader

	msg.Initialize("abc")

	fmt.Printf("Bytes:\n%s\n", hex.Dump(msg.Serialize()))
	fmt.Printf("String:\n%s\n", msg.ToString())

	os.Exit(0)
}