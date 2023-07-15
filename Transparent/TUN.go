package main

import (
	"fmt"
	"log"
	"os/exec"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/songgao/water"
)

func createTUN() *water.Interface {
	ifce, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("sudo", "ip", "link", "set", "tun0", "up")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	return ifce
}

func RedirectFlowToTUN(tunName string) {
	cmd := exec.Command("sudo", "ip", "route", "add", "default", "dev", tunName, "table", "1")
	/*sudo ip route add default dev tun0 table 1*/
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	cmd = exec.Command("sudo", "ip", "rule", "add", "fwmark", "1", "pref", "1", "table", "1")
	/*sudo ip rule add from all to all pref 1 table 1*/
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func RedirectFlowToProxy(tunName string) {
	cmd := exec.Command("sudo", "ip", "route", "replace", "default", "via",
		"127.0.0.1", "dev", tunName, "table", "1")
	/*sudo ip route replace default via 127.0.0.1 dev tun0 table 1*/
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func handlePackets(ifce *water.Interface) {
	packetData := make([]byte, 1024)
	//socksData := make([]byte, 512)
	for {
		n, err := ifce.Read(packetData)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("read %d %v\n", n, packetData[:n])
		ver := (packetData[0] >> 4) & 0x0f
		if ver == 4 {
			ip4Packet := gopacket.NewPacket(packetData, layers.LayerTypeIPv4, gopacket.Default).Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			fmt.Println("Src IP:", ip4Packet.SrcIP)
			fmt.Println("Dst IP", ip4Packet.DstIP)
			fmt.Println("Protocol:", ip4Packet.Protocol)
			fmt.Println("Payload:", ip4Packet.Payload)
			if ip4Packet.Protocol.String() == "TCP" {
				tcpPacket := gopacket.NewPacket(ip4Packet.Payload, layers.LayerTypeTCP, gopacket.Default).Layer(layers.LayerTypeTCP).(*layers.TCP)
				fmt.Println("Src Port:", tcpPacket.SrcPort)
				fmt.Println("Dst Port:", tcpPacket.DstPort)
			}
			//ifce.Write(packetData[:n])
		} else if ver == 6 {
			ip6Packet := gopacket.NewPacket(packetData, layers.LayerTypeIPv6, gopacket.Default).Layer(layers.LayerTypeIPv6).(*layers.IPv6)
			fmt.Println("Src IP:", ip6Packet.SrcIP)
			fmt.Println("Dst IP:", ip6Packet.DstIP)
			fmt.Println("NextHeader:", ip6Packet.NextHeader)
			fmt.Println("Payload:", ip6Packet.Payload)
			if ip6Packet.NextHeader.String() == "TCP" {
				tcpPacket := gopacket.NewPacket(ip6Packet.Payload, layers.LayerTypeTCP, gopacket.Default).Layer(layers.LayerTypeTCP).(*layers.TCP)
				fmt.Println("Src Port:", tcpPacket.SrcPort)
				fmt.Println("Dst Port:", tcpPacket.DstPort)
			}
			//ifce.Write(packetData[:n])
		} else {
			fmt.Printf("No IPPacket")
		}
	}
}
