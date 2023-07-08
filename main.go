package main

import (
	"fmt"
	"net"
)

func main() {
	server, err := net.Listen("tcp", ":8080")
	if (err != nil) {
		fmt.Printf("Listen error: %v\n", err)
		return 
	}
	for {
		client, err := server.Accept()
		if (err != nil) {
			fmt.Printf("Accept error: %v\n", err)
			continue
		}
		go Communicate(client)
	}
}