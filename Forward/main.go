package main

import "Socks5"

func main() {
	// go Socks5.StartProxy(":9090", true, false) // 是否启用 TLS 劫持，是否启用多级代理
	// go Socks5.StartProxy(":3333", true, false)
	// go Socks5.StartProxy(":4444", true, false)
	// go Socks5.StartProxy(":8080", true, true)
	// select {}
	Socks5.StartProxy(":8080", false, false)
}
