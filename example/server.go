package main

import (
	"fmt"
	"time"

	"github.com/archit120/faketcp/faketcp"
)

func main() {
	faketcp.Init("eth0")
	ln, err := faketcp.Listen("faketcp", "127.0.0.1:12222")
	if err != nil {
		fmt.Println(err)
		return
	}

	for {
		if conn, err := ln.Accept(); err == nil {
			fmt.Println("new connection: ", conn.RemoteAddr())
			go func() {
				for {
					conn.Write([]byte(fmt.Sprintf("[%v] Hello", time.Now())))
					time.Sleep(time.Second)
				}
			}()

			go func() {
				buf := make([]byte, 100)
				for {
					n, err := conn.Read(buf)
					if err == nil && n > 0 {
						fmt.Printf("From %v: %v\n", conn.RemoteAddr(), string(buf[:n]))
					} else if err != nil {
						fmt.Printf("%v error: %v\n", conn.RemoteAddr(), err)
						break
					}
				}
			}()
		}
	}

}
