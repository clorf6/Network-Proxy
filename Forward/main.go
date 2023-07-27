package main

import (
	"Socks5"
)

func main() {
	//Socks5.StartProxy(":4444", true, false) // 是否启用 TLS 劫持
	Socks5.StartProxy(":8080", true, true)
}
