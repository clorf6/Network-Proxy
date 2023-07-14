package Socks5

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
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
		addr = string(buffer[1 : 1+addrlen])
		port = binary.BigEndian.Uint16(buffer[1+addrlen : 3+addrlen])
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

func GetData(atyp int, buffer []byte) []byte {
	switch atyp {
	case 1:
		return buffer[6:]
	case 3:
		addrlen := int(buffer[0])
		return buffer[addrlen+3:]
	case 4:
		return buffer[18:]
	default:
		return nil
	}
}

func Authentication(client net.Conn) error {
	var buffer []byte = make([]byte, 512)
	n, err := io.ReadFull(client, buffer[:2])
	if n != 2 {
		return errors.New("Reading error")
	}
	if err != nil {
		return err
	}
	ver, nmethods := int(buffer[0]), int(buffer[1])
	if ver != 5 {
		return errors.New("Invalid version")
	}
	n, err = io.ReadFull(client, buffer[:nmethods])
	if n != nmethods {
		return errors.New("Auth is so long or so short")
	}
	if err != nil {
		return err
	}
	var flag bool = false
	for i := 0; i < nmethods; i++ {
		if buffer[i] == 0 {
			flag = true
			break
		}
	}
	if !flag {
		n, err = client.Write([]byte{0x05, 0xff})
		return errors.New("Refuse Auth")
	}
	n, err = client.Write([]byte{0x05, 0x00})
	if n != 2 {
		return errors.New("Reading error")
	}
	if err != nil {
		return err
	}
	return nil
}

func HandleConnect(client, host net.Conn) {
	go io.Copy(client, host)
	io.Copy(host, client)
}

func HandleUDP(LocalConn, RemoteConn *net.UDPConn, dest string) error {
	ProxyAddr := LocalConn.LocalAddr().(*net.UDPAddr)
	ProxyAddr2 := RemoteConn.LocalAddr().(*net.UDPAddr)
	fmt.Printf("ProxyAddr %v\n", ProxyAddr)
	fmt.Printf("ProxyAddr2 %v\n", ProxyAddr2)
	var buffer []byte = make([]byte, 512)
	n, LocalAddr, err := LocalConn.ReadFromUDP(buffer)
	if err != nil {
		fmt.Printf("%v\n", err)
		return err
	}
	fmt.Printf("DST %v\n", buffer[:n])
	fmt.Printf("LocalAddr %v\n", LocalAddr)
	if LocalAddr.String() == dest {
		RemoteAddr, _ := net.ResolveUDPAddr("udp", GetAddr(int(buffer[3]), buffer[4:]))
		fmt.Printf("RemoteAddr %v\n", RemoteAddr)
		var LocalBuffer []byte = make([]byte, 512)
		var RemoteBuffer []byte = make([]byte, 512)
		copy(LocalBuffer, buffer)
		Local_n := n
		Remote_n := 0
		for {
			RemoteConn.WriteToUDP(GetData(int(LocalBuffer[3]), LocalBuffer[4:Local_n]), RemoteAddr)
			fmt.Printf("write to remote %d %v\n", Local_n, LocalBuffer[:Local_n])
			Remote_n, RemoteAddr, _ = RemoteConn.ReadFromUDP(RemoteBuffer)
			fmt.Printf("write to local %d %v\n", Remote_n, RemoteBuffer[:Remote_n])
			LocalConn.WriteToUDP(GetData(int(RemoteBuffer[3]), RemoteBuffer[4:Remote_n]), LocalAddr)
			Local_n, LocalAddr, _ = LocalConn.ReadFromUDP(LocalBuffer)
		}
	}
	return nil
}

func Connect(client net.Conn) error {
	var buffer []byte = make([]byte, 512)
	n, err := io.ReadFull(client, buffer[:4])
	if n != 4 {
		return errors.New("Reading error")
	}
	if err != nil {
		return err
	}
	ver, cmd, _, atyp := int(buffer[0]), int(buffer[1]), int(buffer[2]), int(buffer[3])
	if ver != 5 {
		return errors.New("Invalid version")
	}
	switch atyp {
	case 1:
		n, err = io.ReadFull(client, buffer[:6])
		if n != 6 {
			return errors.New("Reading error")
		}
		if err != nil {
			return err
		}
	case 3:
		n, err = io.ReadFull(client, buffer[:1])
		if n != 1 {
			return errors.New("Reading error")
		}
		if err != nil {
			return err
		}
		addrlen := int(buffer[0])
		n, err = io.ReadFull(client, buffer[1:addrlen+3])
		if n != addrlen+2 {
			return errors.New("Reading error")
		}
		if err != nil {
			return err
		}
	case 4:
		n, err = io.ReadFull(client, buffer[:18])
		if n != 18 {
			return errors.New("Reading error")
		}
		if err != nil {
			return err
		}
	default:
		n, _ = client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return errors.New("ATYP is wrong")
	}
	dest := GetAddr(atyp, buffer)
	if cmd < 1 || cmd > 3 {
		n, _ = client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return errors.New("cmd is wrong")
	} else if cmd == 1 {
		dst, err := net.DialTimeout("tcp", dest, 3*time.Second)
		if err != nil {
			if strings.Contains(err.Error(), "lookup invalid") {
				n, _ = client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			} else if strings.Contains(err.Error(), "network is unreachable") {
				n, _ = client.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			} else {
				n, _ = client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			}
			return err
		}
		defer dst.Close()
		n, err = client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		if err != nil {
			return err
		}
		HandleConnect(client, dst)
		return nil
	} else if cmd == 3 {
		Addr := "127.0.0.1:0"
		EmptyAddr, _ := net.ResolveUDPAddr("udp", Addr)
		LocalConn, err := net.ListenUDP("udp", EmptyAddr)
		if err != nil {
			return err
		}
		ProxyAddr := LocalConn.LocalAddr().(*net.UDPAddr)
		fmt.Printf("UDP\n")
		fmt.Printf("dest %s\n", dest)
		var Reply []byte = make([]byte, 512)
		Reply[0] = 0x05
		Reply[1] = 0x00
		Reply[2] = 0x00
		if ProxyAddr.IP.To4() != nil {
			Reply[3] = 0x01
			copy(Reply[4:8], ProxyAddr.IP)
			binary.BigEndian.PutUint16(Reply[8:10], uint16(ProxyAddr.Port))
			n, err = client.Write(Reply[:10])
			fmt.Printf("rep %v\n", Reply[:10])
		} else if ProxyAddr.IP.To16() != nil {
			Reply[3] = 0x04
			copy(Reply[4:20], ProxyAddr.IP)
			binary.BigEndian.PutUint16(Reply[20:22], uint16(ProxyAddr.Port))
			n, err = client.Write(Reply[:22])
			fmt.Printf("rep %v\n", Reply[:22])
		}
		if err != nil {
			return err
		}
		RemoteConn, err := net.ListenUDP("udp", nil)
		if err != nil {
			return err
		}
		go func() error {
			err = HandleUDP(LocalConn, RemoteConn, dest)
			defer LocalConn.Close()
			defer RemoteConn.Close()
			return err
		}()
	}
	return nil
}

func Communicate(client net.Conn) {
	defer client.Close()
	err := Authentication(client)
	if err != nil {
		fmt.Printf("Authentication error: %v\n", err)
		return
	}
	err = Connect(client)
	if err != nil {
		fmt.Printf("Connect error: %v\n", err)
		return
	}
}

func StartProxy(ProxyAddr string) {
	server, err := net.Listen("tcp", ProxyAddr)
	if err != nil {
		fmt.Printf("Listen error: %v\n", err)
		return
	}
	for {
		client, err := server.Accept()
		if err != nil {
			fmt.Printf("Accept error: %v\n", err)
			continue
		}
		go Communicate(client)
	}
}
