package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/immesys/spawnpoint/spawnable"
	"gopkg.in/immesys/bw2bind.v5"

	"golang.org/x/net/ipv4"
)

func main() {
	bwc := bw2bind.ConnectOrExit("")
	bwc.OverrideAutoChainTo(true)
	bwc.SetEntityFromEnvironOrExit()

	params := spawnable.GetParamsOrExit()
	baseURI := params.MustString("svc_base_uri")
	if !strings.HasSuffix(baseURI, "/") {
		baseURI += "/"
	}
	socks, err := openSockets()
	if err != nil {
		panic(err)
	}
	ch := make(chan DiscoveryRecord, 10)
	go Discovery(socks, ch)
	tstats := make(map[string]chan DiscoveryRecord)
	tstatnames := make(map[string]string)
	for r := range ch {
		tstat, ok := tstats[r.MAC]
		if ok {
			tstat <- r
		} else {
			existing, ok := tstatnames[r.Name]
			if ok {
				fmt.Printf("WARNING: dropping thermostat with identical name (%s on %s clashes with %s on %s)\n", r.Name, r.MAC, r.Name, existing)
				continue
			}
			fmt.Println("registered new thermostat:")
			fmt.Println("  ip  : ", r.IP)
			fmt.Println("  mac : ", r.MAC)
			fmt.Println("  name: ", r.Name)
			fmt.Println("  type: ", r.Type)
			tstatnames[r.Name] = r.MAC
			tstats[r.MAC] = newThermostat(baseURI, bwc, r)
		}
	}
}

type DiscoveryRecord struct {
	IP   string
	USN  string
	MAC  string
	Name string
	Type string
}

func Discovery(socks []*ipv4.PacketConn, discovery chan DiscoveryRecord) {
	go DiscoverySend(socks)
	rch := make(chan []byte, 10)
	for _, s := range socks {
		go DiscoveryReceive(rch, s)
	}
top:
	for buf := range rch {
		sf := string(buf)
		if !strings.HasPrefix(sf, "HTTP/1.1 200 OK\r\n") {
			continue
		}
		sfn := strings.Split(sf, "\r\n")
		ip := ""
		usn := ""
		for _, ln := range sfn {
			if strings.HasPrefix(ln, "ST: ") {
				if strings.TrimSpace(ln[3:]) != "colortouch:ecp" {
					continue top
				}
			}
			if strings.HasPrefix(ln, "Location: http://") {
				ip = strings.TrimSpace(ln[17 : len(ln)-1])
			}
			if strings.HasPrefix(ln, "USN: ecp:") {
				usn = strings.TrimSpace(ln[9:])
			}
		}
		if ip != "" && usn != "" {
			MAC := usn[:17]
			Name := usn[23:]
			nameend := strings.Index(Name, ":")
			Type := Name[nameend+6:]
			Name = Name[:nameend]

			discovery <- DiscoveryRecord{IP: ip, USN: usn, Name: Name, Type: Type, MAC: MAC}
		}
	}
}

func DiscoverySend(socks []*ipv4.PacketConn) {
	solicitmsg := []byte("M-SEARCH * HTTP/1.1\r\nHost: 239.255.255.250:1900\r\nMan: ssdp:discover\r\nST: colortouch:ecp\r\n")
	grp := &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250), Port: 1900}
	for {
		for _, s := range socks {
			_, err := s.WriteTo(solicitmsg, nil, grp)
			if err != nil {
				panic(err)
			}
		}
		time.Sleep(15 * time.Second)
	}
}
func DiscoveryReceive(r chan []byte, s *ipv4.PacketConn) {
	for {
		buff := make([]byte, 1500)
		n, _, _, err := s.ReadFrom(buff)
		if err != nil {
			panic(err)
		}
		r <- buff[:n]
	}
}
