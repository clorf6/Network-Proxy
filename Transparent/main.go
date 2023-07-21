package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

func main() {
	ifce := createTUN()
	log.Printf("Interface Name: %s\n", ifce.Name())
	RedirectFlowToTUN(ifce.Name())
	go handlePackets(ifce)
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
	cmd := exec.Command("sudo", "ip", "rule", "del", "from",
		"all", "to", "all", "pref", "1", "table", "1")
	_ = cmd.Run()
	ifce.Close()
}
