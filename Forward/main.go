package main

import (
	"Socks5"
)

func main() {
	Socks5.StartProxy(":8080", true)
}
