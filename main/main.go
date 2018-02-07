package main

import (
	"os"
	"swarmd/packets"
	"fmt"
	//"encoding/hex"
)

func main () {
	var msg packets.MessageHeader
	var serialized packets.SerializedPacket
	var msgcpy packets.MessageHeader

	msg.Initialize("abc")

	serialized = msg.Serialize()
	msgcpy.Deserialize(serialized)

	//fmt.Printf("Bytes:\n%s\n", hex.Dump(serial))
	//fmt.Printf("String:\n%s\n", msg.ToString())
	fmt.Printf("Valid Checksum: %t\n", msgcpy.Common.ValidChecksum)

	os.Exit(0)
}