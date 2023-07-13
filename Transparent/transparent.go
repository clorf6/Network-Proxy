package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

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

func RedirectFlow(tunName string) {
	cmd := exec.Command("sudo", "ip", "route", "add", "default",
		"dev", tunName, "table", "1")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	cmd = exec.Command("sudo", "ip", "rule", "add", "from",
		"all", "to", "all", "pref", "1", "table", "1")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func handlePackets(ifce *water.Interface) {
	packetData := make([]byte, 4096)
	for {
		n, err := ifce.Read(packetData)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("read %d %v\n", n, packetData[:n])
		ver := (packetData[0] >> 4) & 0x0f
		if ver == 4 {
			packet := gopacket.NewPacket(packetData, layers.LayerTypeIPv4, gopacket.Default)
			ip4Layer := packet.Layer(layers.LayerTypeIPv4)
			if ip4Layer != nil {
				ip4Packet, _ := ip4Layer.(*layers.IPv4)
				fmt.Println("Src IP:", ip4Packet.SrcIP)
				fmt.Println("Dst IP", ip4Packet.DstIP)
				fmt.Println("Protocol:", ip4Packet.Protocol)
				ifce.Write(packetData[:n])
			}
		} else if ver == 6 {
			packet := gopacket.NewPacket(packetData, layers.LayerTypeIPv6, gopacket.Default)
			ip6Layer := packet.Layer(layers.LayerTypeIPv6)
			if ip6Layer != nil {
				ip6Packet, _ := ip6Layer.(*layers.IPv6)
				fmt.Println("Src IP:", ip6Packet.SrcIP)
				fmt.Println("Dst IP:", ip6Packet.DstIP)
				fmt.Println("NextHeader:", ip6Packet.NextHeader)
				ifce.Write(packetData[:n])
			}
		} else {
			fmt.Printf("No IPPacket")
		}
	}
}

func main() {
	ifce := createTUN()
	log.Printf("Interface Name: %s\n", ifce.Name())
	RedirectFlow(ifce.Name())
	go handlePackets(ifce)
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
	cmd := exec.Command("sudo", "ip", "rule", "del", "from",
		"all", "to", "all", "pref", "1", "table", "1")
	_ = cmd.Run()
	ifce.Close()
}
