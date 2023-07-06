package main

import (
	"fmt"
	"net"
	"errors"
	"io"
	"encoding/binary"
	"time"
)

var buffer []byte = make([]byte, 512)

func Authentication(client net.Conn) error {
	n, err := io.ReadFull(client, buffer[:2])
	if (n != 2 || err != nil) {
		return err
	}
	ver, nmethods := int(buffer[0]), int(buffer[1])
	if (ver != 5) {
		return errors.New("Invalid version")
	}
	n, err = io.ReadFull(client, buffer[:nmethods])
	if (n != nmethods || err != nil) {
		return err
	}
	n, err = client.Write([]byte{0x05, 0x00})
	if (n != 2 || err != nil) {
		return err
	}
	return nil
}

func Connect(client net.Conn) (net.Conn, error) {
	n, err := io.ReadFull(client, buffer[:4])
	if (n != 4 || err != nil) {
		return nil, err
	}
	ver, cmd, _, atyp := int(buffer[0]), int(buffer[1]), int(buffer[2]), int(buffer[3])
	if (ver != 5) {
		return nil, errors.New("Invalid version")
	}
	var addr string
	var Addr []byte = make([]byte, 512)
	var Port []byte = make([]byte, 2)
	var rep byte = 0x00
	var length int = 0
	switch atyp {
		case 1: 
			n, err = io.ReadFull(client, buffer[:4])
			if (n != 4 || err != nil) {
				return nil, err
			}
			copy(Addr, buffer)
			length = n
			addr = fmt.Sprintf("%d.%d.%d.%d", buffer[0], buffer[1], buffer[2], buffer[3])
		case 3:
			n, err = io.ReadFull(client, buffer[:1])
			if (n != 1 || err != nil) {
				return nil, err
			}
			addrlen := int(buffer[0])
			n, err = io.ReadFull(client, buffer[:addrlen])
			if (n != addrlen || err != nil) {
				return nil, err
			}
			copy(Addr[1:], buffer)
			Addr[0] = byte(addrlen)
			length = n + 1
			addr = string(buffer[:addrlen])
		case 4:
			n, err = io.ReadFull(client, buffer[:16])
			if (n != 16 || err != nil) {
				return nil, err
			}
			copy(Addr, buffer)
			length = n
			addr = fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x", 
			buffer[0], buffer[1], buffer[2], buffer[3], buffer[4], buffer[5], buffer[6], buffer[7])
		default: 
			rep = 0x08
	}
	n, err = io.ReadFull(client, buffer[:2])
	if (n != 2 || err != nil) {
		return nil, err
	}
	copy(Port[:2], buffer[:2])
	port := binary.BigEndian.Uint16(buffer[:2])
	dest := fmt.Sprintf("%s:%d", addr, port)
	fmt.Printf("get %s\n", dest)
	var Reply []byte = make([]byte, 512)
	Reply[0] = 0x05
	Reply[1] = rep
	Reply[2] = 0x00
	if (cmd == 1) {
		dst, err := net.DialTimeout("tcp", dest, 3 * time.Second)
		if (err != nil) {
			return nil, err
		}
		Reply[3] = byte(atyp)
		copy(Reply[4:], Addr[:length])
		copy(Reply[(4 + length):], Port)
		n, err = client.Write(Reply[:length + 6])
		if (err != nil) {
			dst.Close()
			return nil, err
		}
		return dst, nil
	} else if (cmd == 3) {
		fmt.Printf("UDP\n");
		Reply[3] = 0x01
		Reply[4] = 0x00
		Reply[5] = 0x00
		Reply[6] = 0x00
		Reply[7] = 0x00
		Reply[8] = 0xd2
		Reply[9] = 0x04
		n, err = client.Write(Reply[:10])
		if (err != nil) {
			return nil, err
		}
		ProxyAddr, _ := net.ResolveUDPAddr("udp", "0.0.0.0:1234")
		UDPserver, err := net.ListenUDP("udp", ProxyAddr)
		defer UDPserver.Close()
		n, LocalAddr, err := UDPserver.ReadFromUDP(buffer)
		if (err != nil) {
			return nil, err
		}
		if (LocalAddr.String() == dest) {
			atyp = int(buffer[3])
			switch atyp {
				case 1: 
					addr = fmt.Sprintf("%d.%d.%d.%d", buffer[4], buffer[5], buffer[6], buffer[7])
					port = binary.BigEndian.Uint16(buffer[8:10])
				case 3:
					addrlen := int(buffer[4])
					addr = string(buffer[5: 5 + addrlen])
					port = binary.BigEndian.Uint16(buffer[5 + addrlen:7 + addrlen])
				case 4:
					addr = fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x", 
					buffer[4], buffer[5], buffer[6], buffer[7], buffer[8], buffer[9], buffer[10], buffer[11])
					port = binary.BigEndian.Uint16(buffer[12:14])
				default:
			}
			dest = fmt.Sprintf("%s:%d", addr, port)
			RemoteAddr, _ := net.ResolveUDPAddr("udp", dest)
			conn, err := net.DialUDP("udp", ProxyAddr, RemoteAddr)
			if (err != nil) {
				return nil, err
			}
			defer conn.Close()
			for {
				_, err := conn.WriteToUDP(buffer[:n], RemoteAddr)
				if (err != nil) {
					continue
				}
				n, _, err := conn.ReadFromUDP(buffer)
				if (err != nil) {
					continue
				}
				_, err = UDPserver.WriteToUDP(buffer[:n], LocalAddr)
				if (err != nil) {
					continue
				}
				n, _, err = UDPserver.ReadFromUDP(buffer)
			}
		}
		fmt.Println("addr: ", addr, "message: ", string(buffer))
	}
	return nil, errors.New("fuck you")
}

func Request(client, host net.Conn) {
	go io.Copy(host, client)
	io.Copy(client, host)
}

func Communicate(client net.Conn) {
	err := Authentication(client)
	if (err != nil) {
		fmt.Printf("Authentication error: %v\n", err)
		return 
	}
	defer client.Close()
	host, err := Connect(client)
	if (err != nil) {
		fmt.Printf("Connect error: %v\n", err)
		return 
	}
	defer host.Close()
	Request(client, host)
}

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