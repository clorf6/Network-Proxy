package main

import (
	"fmt"
	"net"
	"errors"
	"io"
	"strings"
	"encoding/binary"
	"time"
)

func Authentication(client net.Conn) error {
	var buffer []byte = make([]byte, 512)
	n, err := io.ReadFull(client, buffer[:2])
	if (n != 2) {
		return errors.New("Reading error")
	}
	if (err != nil) {
		return err
	}
	ver, nmethods := int(buffer[0]), int(buffer[1])
	if (ver != 5) {
		return errors.New("Invalid version")
	}
	n, err = io.ReadFull(client, buffer[:nmethods])
	//n, err = client.Read(buffer)
	if (n != nmethods) {
		return errors.New("Auth is so long or so short")
	}
	if (err != nil) {
		return err
	}
	var flag bool = false
	for i := 0; i < nmethods; i++ {
		if (buffer[i] == 0) {
			flag = true
			break
		}
	}
	if (!flag) {
		n, err = client.Write([]byte{0x05, 0xff})
		return errors.New("Refuse Auth")
	}
	n, err = client.Write([]byte{0x05, 0x00})
	if (n != 2) {
		return errors.New("Reading error")
	}
	if (err != nil) {
		return err
	}
	return nil
}

func Connect(client net.Conn) (net.Conn, error) {
	var buffer []byte = make([]byte, 512)
	n, err := io.ReadFull(client, buffer[:4])
	if (n != 4) {
		return nil, errors.New("Reading error")
	}
	if (err != nil) {
		return nil, err
	}
	ver, cmd, _, atyp := int(buffer[0]), int(buffer[1]), int(buffer[2]), int(buffer[3])
	if (ver != 5) {
		return nil, errors.New("Invalid version")
	}
	var addr string
	var Addr []byte = make([]byte, 512)
	var length int = 0
	switch atyp {
		case 1: 
			n, err = io.ReadFull(client, buffer[:4])
			if (n != 4) {
				return nil, errors.New("Reading error")
			}
			if (err != nil) {
				return nil, err
			}
			copy(Addr, buffer)
			length = n
			addr = fmt.Sprintf("%d.%d.%d.%d", buffer[0], buffer[1], buffer[2], buffer[3])
		case 3:
			n, err = io.ReadFull(client, buffer[:1])
			if (n != 1) {
				return nil, errors.New("Reading error")
			}
			if (err != nil) {
				return nil, err
			}
			addrlen := int(buffer[0])
			n, err = io.ReadFull(client, buffer[:addrlen])
			if (n != addrlen) {
				return nil, errors.New("Reading error")
			}
			if (err != nil) {
				return nil, err
			}
			copy(Addr[1:], buffer)
			Addr[0] = byte(addrlen)
			length = n + 1
			addr = string(buffer[:addrlen])
		case 4:
			n, err = io.ReadFull(client, buffer[:16])
			if (n != 16) {
				return nil, errors.New("Reading error")
			}
			if (err != nil) {
				return nil, err
			}
			copy(Addr, buffer)
			length = n
			addr = fmt.Sprintf("[%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x]", 
			buffer[0], buffer[1], buffer[2], buffer[3], buffer[4], buffer[5], buffer[6], buffer[7], 
			buffer[8], buffer[9], buffer[10], buffer[11], buffer[12], buffer[13], buffer[14], buffer[15])
		default: 
			n, _ = client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return nil, errors.New("ATYP is wrong")
		}
	n, err = io.ReadFull(client, buffer[:2])
	if (n != 2) {
		return nil, errors.New("Reading error")
	}
	if (err != nil) {
		return nil, err
	}
	port := binary.BigEndian.Uint16(buffer[:2])
	var Port []byte = make([]byte, 2)
	copy(Port[:2], buffer[:2])
	dest := fmt.Sprintf("%s:%d", addr, port)
	var Reply []byte = make([]byte, 512)
	Reply[0] = 0x05
	Reply[1] = 0x00
	Reply[2] = 0x00
	Reply[3] = byte(atyp)
	copy(Reply[4:], Addr[:length])
	copy(Reply[(4 + length):], Port)
	if (cmd < 1 || cmd > 3 ) {
		n, _ = client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return nil, errors.New("cmd is wrong")
	} else if (cmd == 1) {
		dst, err := net.DialTimeout("tcp", dest, 3 * time.Second)
		if (err != nil) {
			if strings.Contains(err.Error(), "lookup invalid") {
				n, _ = client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			} else if strings.Contains(err.Error(), "network is unreachable") {
				n, _ = client.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			} else {
				n, _ = client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			}
			return nil, err
		}
		n, err = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		if (err != nil) {
			dst.Close()
			return nil, err
		}
		return dst, nil
	} else if (cmd == 3) {
		ProxyAddr, _ := net.ResolveUDPAddr("udp", "0.0.0.0:1234")
		fmt.Printf("UDP\n");
		Reply[3] = 0x01
		copy(Reply[4:8], ProxyAddr.IP)
		Reply[8] = 0xd2
		Reply[9] = 0x04
		n, err = client.Write(Reply[:10])
		if (err != nil) {
			return nil, err
		}
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
	return nil, nil
}

func Request(client, host net.Conn) {
	go io.Copy(client, host)
	io.Copy(host, client)
}

func Communicate(client net.Conn) {
	defer client.Close()
	err := Authentication(client)
	if (err != nil) {
		fmt.Printf("Authentication error: %v\n", err)
		return 
	}
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