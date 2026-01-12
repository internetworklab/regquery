package utils

import (
	"fmt"
	"net"
)

type INetFamily int

const (
	INetFamilyUnknown INetFamily = iota
	INetFamilyIPv4    INetFamily = 4
	INetFamilyIPv6    INetFamily = 6
)

func ParseIP(address string) (ip net.IP, family INetFamily, err error) {
	ip = net.ParseIP(address)
	if ip4 := ip.To4(); ip4 != nil {
		family = INetFamilyIPv4
		ip = ip4
	} else if ip6 := ip.To16(); ip6 != nil {
		family = INetFamilyIPv6
		ip = ip6
	} else {
		return nil, INetFamilyUnknown, fmt.Errorf("%s is not an ip address of known family", address)
	}
	return ip, family, nil
}
