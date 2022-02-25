package main

import (
	"fmt"
	"time"

	// "time"

	"github.com/archit120/faketcp/faketcp"
)

func main() {
	// faketcp.Init("eth0")

	ln, err := faketcp.ListenPacket("faketcp", ":12222")
	if err != nil {
		fmt.Println(err)
		return
	}
	var t1 time.Time
	var t2 time.Time
	buffer := make([]byte, 1024)
	for {
		n, from, _ := ln.ReadFrom(buffer)
		ln.WriteTo(buffer[:n], from)
	}
	fmt.Println(t2.Sub(t1))
}
