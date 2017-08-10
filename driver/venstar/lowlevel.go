package main

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"golang.org/x/net/ipv4"
)

// who said golang didn't require magic incantations?
func openSockets() ([]*ipv4.PacketConn, error) {
	rv := []*ipv4.PacketConn{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return rv, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return rv, err
		}
		for _, a := range addrs {
			ip, _, _ := net.ParseCIDR(a.String())
			if ip.IsUnspecified() || ip.IsLoopback() {
				continue
			}
			fmt.Printf("probing network interface: %s (subnet %s, addr %s)\n", iface.Name, a.String(), ip)
			sock, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
			if err != nil {
				return rv, err
			}
			err = syscall.SetsockoptInt(sock, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			if err != nil {
				return rv, err
			}
			syscall.SetsockoptString(sock, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface.Name)
			lsa := syscall.SockaddrInet4{Port: 1900}
			copy(lsa.Addr[:], ip.To4())
			if err = syscall.Bind(sock, &lsa); err != nil {
				return rv, err
			}
			f := os.NewFile(uintptr(sock), "")
			c, err := net.FilePacketConn(f)
			f.Close()
			if err != nil {
				return rv, err
			}
			p := ipv4.NewPacketConn(c)
			err = p.JoinGroup(&iface, &net.UDPAddr{IP: net.IPv4(239, 255, 255, 250)})
			if err != nil {
				return rv, err
			}
			rv = append(rv, p)
		}

	}
	return rv, nil
}
