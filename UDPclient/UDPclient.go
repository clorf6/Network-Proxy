package main

import (
	"log"
	"fmt"
	"net"
	"encoding/binary"
)

func GetAddr(atyp int, buffer []byte) string {
	var addr string
	var port uint16
	switch atyp {
		case 1: 
			addr = fmt.Sprintf("%d.%d.%d.%d", buffer[0], buffer[1], buffer[2], buffer[3])
			port = binary.BigEndian.Uint16(buffer[4:6])
		case 3:
			addrlen := int(buffer[0])
			addr = string(buffer[1: 1 + addrlen])
			port = binary.BigEndian.Uint16(buffer[1 + addrlen:3 + addrlen])
		case 4:
			addr = fmt.Sprintf("[%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x]", 
			buffer[0], buffer[1], buffer[2], buffer[3], buffer[4], buffer[5], buffer[6], buffer[7], 
			buffer[8], buffer[9], buffer[10], buffer[11], buffer[12], buffer[13], buffer[14], buffer[15])
			port = binary.BigEndian.Uint16(buffer[16:18])
		default:
			return ""
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func main() {
	TCPconn, err := net.Dial("tcp", "127.0.0.1:8080")
	if (err != nil) {
		log.Fatal(err)
	}
	defer TCPconn.Close()
	_, err = TCPconn.Write([]byte{0x05, 0x01, 0x00})
	if (err != nil) {
		log.Fatal(err)
	}
	var buffer []byte = make([]byte, 512)
	n, err := TCPconn.Read(buffer)
	if (err != nil) {
		log.Fatal(err)
	}
	fmt.Printf("Authback %d %v\n", n, buffer[:n])
	_, err = TCPconn.Write([]byte{0x05, 0x03, 0x00, 0x01, 
	0x7f, 0x00, 0x00, 0x01, 0x10, 0xe1})
	if (err != nil) {
		log.Fatal(err)
	}
	n, err = TCPconn.Read(buffer)
	if (err != nil) {
		log.Fatal(err)
	}
	fmt.Printf("Bindback %d %v\n", n, buffer[:n])
	ProxyAddr, err := net.ResolveUDPAddr("udp", GetAddr(int(buffer[3]), buffer[4:]))
	LocalAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:4321")
	if (err != nil) {
		log.Fatal(err)
	}
	fmt.Printf("localaddr %v\n", LocalAddr)
	fmt.Printf("proxyaddr %v\n", ProxyAddr)
	UDPconn1, err := net.ListenUDP("udp", LocalAddr)
	defer UDPconn1.Close()
	if (err != nil) {
		log.Fatal(err)
	}
	RemoteAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:9999")
	if (err != nil) {
		log.Fatal(err)
	}
	fmt.Printf("remoteaddr %v\n", RemoteAddr)
	UDPconn1.WriteToUDP([]byte{0x00, 0x00, 0x00, 0x01, 0x7f, 0x00, 
	0x00, 0x01, 0x27, 0x0f, 0x01, 0x02, 0x03, 0x04}, ProxyAddr)
	fmt.Printf("write\n")
	UDPconn2, err := net.ListenUDP("udp", RemoteAddr)
	defer UDPconn2.Close()
	if (err != nil) {
		log.Fatal(err)
	}
	for {
		func() {
			for {
				n, ProxyAddr, err = UDPconn2.ReadFromUDP(buffer)
				if (err != nil) {
					continue
				} else {
					fmt.Printf("fromLocal %d %v\n", n, buffer[:n])
					fmt.Printf("proxyaddr %v\n", ProxyAddr)
					break
				}
			} 
		}()
		UDPconn2.WriteToUDP([]byte{0x00, 0x00, 0x00, 0x01, 0x7f, 0x00, 
			0x00, 0x01, 0x10, 0xe1, 0x05, 0x06, 0x07, 0x08, 0x09}, ProxyAddr)
		if (err != nil) {
			log.Fatal(err)
		}
		func() {
			for {
				n, ProxyAddr, err = UDPconn1.ReadFromUDP(buffer)
				if (err != nil) {
					continue
				} else {
					fmt.Printf("fromRemote %d %v\n", n, buffer[:n])
					fmt.Printf("proxyaddr %v\n", ProxyAddr)
					break
				}
			}
		}()
		UDPconn1.WriteToUDP([]byte{0x00, 0x00, 0x00, 0x01, 0x7f, 0x00, 
			0x00, 0x01, 0x27, 0x0f, 0x01, 0x02, 0x03, 0x04}, ProxyAddr)
	}
}