package main

import (
	"fmt"
	"net"
	"strings"
)

func convertAddrs(_addrs string) ([]string, error) {
	var (
		dest  []string
		addrs = strings.Split(_addrs, ",")
	)

	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		ip := net.ParseIP(addr)
		if ip != nil { // valid ip
			dest = append(dest, ip.String())
			continue
		}

		hosts, err := net.LookupHost(addr)
		if err != nil {
			return dest, err
		}
		if hosts == nil {
			return dest, fmt.Errorf("invalid addr %s ", addr)
		}
		ipa, err := net.ResolveIPAddr("ip", hosts[0])
		if err != nil {
			return dest, fmt.Errorf("failed to dns query addr %s ", addr)
		}

		dest = append(dest, ipa.String())
	}

	return dest, nil
}
