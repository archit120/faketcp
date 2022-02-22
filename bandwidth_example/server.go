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
	buffer := make([]byte, 1024*1024*100)
	c :=0
	for {
		
		n, _, _ := ln.ReadFrom(buffer)
		if c==0 {
			t1 = time.Now()
		}
		c+=n
		if c >= 512*1024*1024 {
			t2 = time.Now()
			break
		}
		// fmt.Println(c)
	}
	fmt.Println(t2.Sub(t1))
}
