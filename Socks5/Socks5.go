package Socks5

import (
	"Shunt"
	"TLS"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func GetAddr(atyp int, buffer []byte) (string, uint16, string) {
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
		addr = fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x:%02x%02x",
			buffer[0], buffer[1], buffer[2], buffer[3], buffer[4], buffer[5], buffer[6], buffer[7],
			buffer[8], buffer[9], buffer[10], buffer[11], buffer[12], buffer[13], buffer[14], buffer[15])
		port = binary.BigEndian.Uint16(buffer[16:18])
	default:
		return "", 0, ""
	}
	var dest string = ""
	if atyp == 4 {
		dest = fmt.Sprintf("[%s]:%d", addr, port)
	} else {
		dest = fmt.Sprintf("%s:%d", addr, port)
	}
	return addr, port, dest
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
		return errors.New("authentic error")
	}
	if err != nil {
		return err
	}
	ver, nmethods := int(buffer[0]), int(buffer[1])
	if ver != 5 {
		return errors.New("invalid version")
	}
	n, err = io.ReadFull(client, buffer[:nmethods])
	if n != nmethods {
		return errors.New("auth is so long or so short")
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
		client.Write([]byte{0x05, 0xff})
		return errors.New("refuse auth")
	}
	n, err = client.Write([]byte{0x05, 0x00})
	if n != 2 {
		return errors.New("reading error")
	}
	if err != nil {
		return err
	}
	return nil
}

func TransmitConnect(client, host net.Conn) {
	defer client.Close()
	defer host.Close()
	go io.Copy(client, host)
	io.Copy(host, client)
}

func TransmitUDP(LocalConn, RemoteConn *net.UDPConn, dest string) error {
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
		_, _, remoteAddr := GetAddr(int(buffer[3]), buffer[4:])
		RemoteAddr, _ := net.ResolveUDPAddr("udp", remoteAddr)
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

func GetReply(IP net.IP, port int) []byte {
	var Reply []byte = make([]byte, 0)
	Reply = append(Reply, 0x05, 0x00, 0x00)
	Port := make([]byte, 2)
	binary.BigEndian.PutUint16(Port, uint16(port))
	if IP.To4() != nil {
		Reply = append(Reply, 0x01)
		IP = IP.To4()
	} else if IP.To16() != nil {
		Reply = append(Reply, 0x04)
		IP = IP.To16()
	}
	Reply = append(Reply, IP...)
	Reply = append(Reply, Port...)
	return Reply
}

func GetSend(IP net.IP, port int) []byte {
	var Send []byte = make([]byte, 0)
	Send = append(Send, 0x05, 0x01, 0x00)
	Port := make([]byte, 2)
	binary.BigEndian.PutUint16(Port, uint16(port))
	if IP.To4() != nil {
		Send = append(Send, 0x01)
		IP = IP.To4()
	} else if IP.To16() != nil {
		Send = append(Send, 0x04)
		IP = IP.To16()
	}
	Send = append(Send, IP...)
	Send = append(Send, Port...)
	return Send
}

func HandleConnect(dest string, client net.Conn, hijack bool) error {
	remote, _ := net.DialTimeout("tcp", dest, 3*time.Second)
	defer remote.Close()
	if hijack {
		nextByte := make([]byte, 4096)
		n, err := client.Read(nextByte)
		if err != nil {
			fmt.Println("Failed to read client handshake:", err)
			return err
		}
		handshakeBuffer := bytes.NewBuffer(nextByte[:n])
		if hijack && nextByte[0] == 22 && nextByte[1] == 3 {
			fmt.Print("TLS\n")
			remote.Close()
			lis := TLS.StartProxyListen()
			TLSaddr := lis.Addr().String()
			go TLS.StartProxyHandle(lis, dest)
			remote, err = net.DialTimeout("tcp", TLSaddr, 3*time.Second)
			if err != nil {
				return err
			}
			defer remote.Close()
		}
		remote.Write(handshakeBuffer.Bytes())
	}
	TransmitConnect(client, remote)
	return nil
}

func HandleUDP(dest string, client net.Conn) error {
	Addr := "127.0.0.1:0"
	EmptyAddr, _ := net.ResolveUDPAddr("udp", Addr)
	LocalConn, err := net.ListenUDP("udp", EmptyAddr)
	if err != nil {
		return err
	}
	ProxyAddr := LocalConn.LocalAddr().(*net.UDPAddr)
	fmt.Printf("UDP\n")
	client.Write(GetReply(ProxyAddr.IP, ProxyAddr.Port))
	if err != nil {
		return err
	}
	RemoteConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return err
	}
	go func() error {
		err = TransmitUDP(LocalConn, RemoteConn, dest)
		defer LocalConn.Close()
		defer RemoteConn.Close()
		return err
	}()
	return nil
}

func HandleMultiAgent(shuntlist []string, client net.Conn,
	dest string, atyp int) error {
	var buffer []byte = make([]byte, 4096)
	remote, err := net.Dial("tcp", shuntlist[0])
	if err != nil {
		fmt.Println("Dial shuntlist[0] err")
		return err
	}
	fmt.Println("Multi Agent")
	for i := 0; i < len(shuntlist); i++ {
		remote.Write([]byte{0x05, 0x01, 0x00})
		n, err := io.ReadFull(remote, buffer[:2])
		if err != nil || n != 2 {
			fmt.Println("Read Auth err")
			return err
		}
		if buffer[0] != 0x05 || buffer[1] != 0x00 {
			return errors.New("auth fail")
		}
		if i != len(shuntlist)-1 {
			addr, err := net.ResolveTCPAddr("tcp", shuntlist[i+1])
			if err != nil {
				fmt.Println("Resolve err")
				return err
			}
			remote.Write(GetSend(addr.IP, addr.Port))
		} else {
			switch atyp {
			case 1:
				fallthrough
			case 4:
				addr, err := net.ResolveTCPAddr("tcp", dest)
				if err != nil {
					fmt.Println("Resolve err")
					return err
				}
				remote.Write(GetSend(addr.IP, addr.Port))
			case 3:
				host, portStr, _ := net.SplitHostPort(dest)
				port, _ := strconv.Atoi(portStr)
				Send := make([]byte, 0)
				Send = append(Send, 0x05, 0x01, 0x00, 0x03, byte(len(host)))
				Send = append(Send, []byte(host)...)
				Port := make([]byte, 2)
				binary.BigEndian.PutUint16(Port, uint16(port))
				Send = append(Send, Port...)
				remote.Write(Send)
			}
		}
		n, err = remote.Read(buffer)
		if n <= 6 || err != nil {
			fmt.Println("Read Bind err")
			return err
		}
		if buffer[1] != 0x00 {
			return errors.New("DST Connect err")
		}
	}
	TransmitConnect(client, remote)
	return nil
}

func Connect(client net.Conn, hijack bool, shunt bool) error {
	var buffer []byte = make([]byte, 4096)
	n, err := io.ReadFull(client, buffer[:4])
	if n != 4 {
		return errors.New("reading error")
	}
	if err != nil {
		return err
	}
	ver, cmd, _, atyp := int(buffer[0]), int(buffer[1]), int(buffer[2]), int(buffer[3])
	if ver != 5 {
		return errors.New("invalid version")
	}
	switch atyp {
	case 1:
		n, err = io.ReadFull(client, buffer[:6])
		if n != 6 {
			return errors.New("reading error")
		}
		if err != nil {
			return err
		}
	case 3:
		n, err = io.ReadFull(client, buffer[:1])
		if n != 1 {
			return errors.New("reading error")
		}
		if err != nil {
			return err
		}
		addrlen := int(buffer[0])
		n, err = io.ReadFull(client, buffer[1:addrlen+3])
		if n != addrlen+2 {
			return errors.New("reading error")
		}
		if err != nil {
			return err
		}
	case 4:
		n, err = io.ReadFull(client, buffer[:18])
		if n != 18 {
			return errors.New("reading error")
		}
		if err != nil {
			return err
		}
	default:
		client.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return errors.New("atyp is wrong")
	}
	addr, _, dest := GetAddr(atyp, buffer)
	remote, err := net.DialTimeout("tcp", dest, 3*time.Second)
	if err != nil {
		if strings.Contains(err.Error(), "lookup invalid") {
			client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		} else if strings.Contains(err.Error(), "network is unreachable") {
			client.Write([]byte{0x05, 0x03, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		} else {
			client.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		}
		return err
	}
	remoteAddr := remote.RemoteAddr().(*net.TCPAddr)
	remote.Close()
	_, err = client.Write(GetReply(remoteAddr.IP, remoteAddr.Port))
	if err != nil {
		return err
	}
	if cmd < 1 || cmd > 3 {
		client.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return errors.New("cmd is wrong")
	} else if cmd == 1 {
		if !shunt {
			return HandleConnect(dest, client, hijack)
		} else {
			shuntlist := Shunt.Shunt(addr, atyp)
			// for i := 0; i < len(shuntlist); i++ {
			// 	fmt.Printf("i %d %v\n", i, shuntlist[i])
			// }
			if shuntlist == nil {
				return HandleConnect(dest, client, hijack)
			} else {
				return HandleMultiAgent(shuntlist, client, dest, atyp)
			}
		}
	} else if cmd == 3 {
		return HandleUDP(dest, client)
	}
	return nil
}

func Communicate(client net.Conn, hijack bool, shunt bool) {
	defer client.Close()
	err := Authentication(client)
	if err != nil {
		fmt.Printf("Authentication error: %v\n", err)
		return
	}
	err = Connect(client, hijack, shunt)
	if err != nil {
		fmt.Printf("Connect error: %v\n", err)
		return
	}
}

func StartProxy(ProxyAddr string, hijack bool, shunt bool) {
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
		go Communicate(client, hijack, shunt)
	}
}
