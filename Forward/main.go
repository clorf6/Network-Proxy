package main

import (
	"Socks5"
)

func main() {
	Socks5.StartProxy(":8080", false) // 是否启用 TLS 劫持
}
