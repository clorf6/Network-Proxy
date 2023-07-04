package main

import (
	"fmt"
	"net"
	"errors"
	"io"
	"encoding/binary"
	"time"
)

var buffer []byte = make([]byte, 255)

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
	if (cmd != 1) {
		return nil, errors.New("Unsupported connection")
	}
	var addr string
	switch atyp {
		case 1: 
			n, err = io.ReadFull(client, buffer[:4])
			if (n != 4 || err != nil) {
				return nil, err
			}
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
			addr = string(buffer[:addrlen])
		case 4:
			n, err = io.ReadFull(client, buffer[:16])
			if (n != 16 || err != nil) {
				return nil, err
			}
			addr = fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x", 
			buffer[0], buffer[1], buffer[2], buffer[3], buffer[4], buffer[5], buffer[6], buffer[7])
		default: 
			return nil, errors.New("Invalid DST.ADDR")
	}
	n, err = io.ReadFull(client, buffer[:2])
	if (n != 2 || err != nil) {
		return nil, err
	}
	port := binary.BigEndian.Uint16(buffer[:2])
	dest := fmt.Sprintf("%s:%d", addr, port)
	// fmt.Printf("Connect! %s\n", dest)
	dst, err := net.DialTimeout("tcp", dest, 3 * time.Second)
	if (err != nil) {
		return nil, err
	}
	n, err = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	if (err != nil) {
		dst.Close()
		return nil, err
	}
	return dst, nil
}

func Request(client, host net.Conn) {
	go io.Copy(client, host)
	go io.Copy(host, client)
}

func Communicate(client net.Conn) {
	defer client.Close()
	err := Authentication(client)
	if (err != nil) {
		fmt.Printf("Authentication error: %v\n", err)
		return 
	}
	host, err := Connect(client)
	defer host.Close()
	if (err != nil) {
		fmt.Printf("Connect error: %v\n", err)
		return 
	}
	Request(client, host)
}

func main() {
	server, err := net.Listen("tcp", ":1080")
	if (err != nil) {
		fmt.Printf("Listen error: %v\n", err)
		return 
	}
	for {
		client, err := server.Accept()
		//fmt.Printf("Accept!\n")
		if (err != nil) {
			fmt.Printf("Accept error: %v\n", err)
			continue
		}
		go Communicate(client)
	}
}