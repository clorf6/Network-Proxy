package Shunt

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"os"
	"strings"
)

func parseShunt(IP string) (net.IP, net.IP) {
	_, network, err := net.ParseCIDR(IP)
	if err != nil {
		log.Panicf("ParseCIDR err: %v", err)
		return nil, nil
	}
	minIP := network.IP
	mask := network.Mask
	maxIP := make(net.IP, len(minIP))
	for i := 0; i < len(minIP); i++ {
		maxIP[i] = minIP[i] | ^mask[i]
	}
	return minIP, maxIP
}

func shuntIP(fileName string, addr string, atyp int) []string {
	var shuntlist []string = nil
	file, err := os.Open(fileName)
	if err != nil {
		log.Panicf("File open error: %v\n", err)
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var flag bool = false
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] == '0' {
			if shuntlist != nil {
				return shuntlist
			}
			flag = false
			L, R := parseShunt(line[2:])
			if L == nil || R == nil {
				continue
			}
			var IP net.IP = nil
			if atyp == 4 {
				addr = strings.TrimLeft(addr, "0")
			}
			ip := net.ParseIP(addr)
			if ip.To4() != nil && atyp == 1 {
				IP = ip.To4()
			} else {
				IP = ip
			}
			if bytes.Compare([]byte(IP), []byte(L)) >= 0 &&
				bytes.Compare([]byte(IP), []byte(R)) <= 0 {
				flag = true
			}
		} else if flag {
			shuntlist = append(shuntlist, line[2:])
		}
	}
	return shuntlist
}

func shuntDomain(fileName string, addr string) []string {
	var shuntlist []string = nil
	file, err := os.Open(fileName)
	if err != nil {
		log.Panicf("File open error: %v\n", err)
		return nil
	}
	defer file.Close()
	addrDomain := strings.SplitN(addr, ".", -1)
	scanner := bufio.NewScanner(file)
	var flag bool = false
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] == '0' {
			if shuntlist != nil {
				return shuntlist
			}
			flag = false
			subDomain := strings.SplitN(line[2:], ".", -1)
			if len(subDomain) == len(addrDomain) {
				flag = true
				for i := 0; i < len(subDomain); i++ {
					if subDomain[i] == "*" {
						continue
					}
					if subDomain[i] != addrDomain[i] {
						flag = false
						break
					}
				}
			}
		} else if flag {
			shuntlist = append(shuntlist, line[2:])
		}
	}
	return shuntlist
}

func Shunt(addr string, atyp int) []string {
	switch atyp {
	case 1:
		return shuntIP("../Shunt/IPv4", addr, 1)
	case 3:
		return shuntDomain("../Shunt/Domain", addr)
	case 4:
		return shuntIP("../Shunt/IPv6", addr, 4)
	default:
		return nil
	}
}
