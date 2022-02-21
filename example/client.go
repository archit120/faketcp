package main

import (
	"fmt"
	"time"

	"github.com/archit120/faketcp/faketcp"
)

func main() {
	conn, err := faketcp.Dial("faketcp", "8.8.8.8:12222")
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("connected")
	go func() {
		for {
			buf := make([]byte, 1024)
			if n, err := conn.Read(buf); err == nil && n > 0 {
				fmt.Println("From server: ", string(buf[:n]))
			} else if err != nil {
				break
			}
		}
	}()

	for i := 0; i < 5; i++ {
		_, err := conn.Write([]byte(fmt.Sprintf("[%v] hello", time.Now())))
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(time.Second)
	}

	if err = conn.Close(); err != nil {
		fmt.Println("close error", err)
	}
}
