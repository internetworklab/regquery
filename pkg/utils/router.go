package utils

import (
	"bytes"
	"fmt"
	"log"
	"net"

	"github.com/google/btree"
)

type RouteTable struct {
	routeGroups  *btree.BTree
	routeGroups6 *btree.BTree
}

func NewRouteTable() *RouteTable {
	return &RouteTable{
		routeGroups:  btree.New(2),
		routeGroups6: btree.New(2),
	}
}

func parseCIDR(cidr string) (*net.IPNet, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CIDR %s: %v", cidr, err)
	}
	if ipnet == nil {
		return nil, fmt.Errorf("failed to parse CIDR %s: the ipnet is nil", cidr)
	}
	return ipnet, nil
}

func parseIP(ip string) (net.IP, int, error) {
	ipaddr := net.ParseIP(ip)

	if ipaddr == nil {
		return nil, 0, fmt.Errorf("failed to parse IP %s: it is not an valid ip address", ip)
	}

	if ip4 := ipaddr.To4(); ip4 != nil {
		return ip4, 32, nil
	}

	return ipaddr, 128, nil
}

func parseIPOrCIDR(ipOrCIDR string) (*net.IPNet, error) {
	ipNet, err := parseCIDR(ipOrCIDR)
	if err == nil {
		return ipNet, nil
	}
	ip, bits, err := parseIP(ipOrCIDR)
	if err == nil {
		mask := net.CIDRMask(bits, bits)
		return &net.IPNet{IP: ip, Mask: mask}, nil
	}
	return nil, fmt.Errorf("string %s is neither a valid CIDR nor a valid IP address: %w", ipOrCIDR, err)
}

func NewRouteTableFromMap(data map[string]interface{}, skipInvalids bool) (*RouteTable, error) {
	table := NewRouteTable()
	for cidr, value := range data {
		ipnet, err := parseIPOrCIDR(cidr)
		if err != nil {
			if skipInvalids {
				log.Printf("skipping invalid CIDR %s: %v", cidr, err)
				continue
			}
			return nil, fmt.Errorf("failed to parse CIDR %s: %v", cidr, err)
		}
		if ipnet == nil {
			log.Printf("invalid CIDR %s: the ipnet is nil", cidr)
		}
		table.Insert(&Route{CIDR: *ipnet, Value: value})
	}
	return table, nil
}

func (routeTable *RouteTable) getRouteCollection(bits int) *btree.BTree {
	if bits == 32 {
		return routeTable.routeGroups
	} else if bits == 128 {
		return routeTable.routeGroups6
	} else {
		panic("bits is not valid (impossible)")
	}
}

func (routeTable *RouteTable) Insert(route *Route) {
	ones, bits := route.CIDR.Mask.Size()
	routeCollection := routeTable.getRouteCollection(bits)

	var routeGroup *RouteGroup
	routeGroupItem := routeCollection.Get(&RouteGroup{PrefixLen: ones})
	if routeGroupItem == nil {
		routeGroup = NewRouteGroup(ones)
		routeCollection.ReplaceOrInsert(routeGroup)
	} else {
		var ok bool
		routeGroup, ok = routeGroupItem.(*RouteGroup)
		if !ok {
			panic("routeGroupItem is not a *RouteGroup (impossible)")
		}
	}

	routeGroup.Insert(route)
}

func (routeTable *RouteTable) Get(ipnet net.IPNet) *Route {
	_, bits := ipnet.Mask.Size()

	result := make(chan *Route, 2)
	routeTable.getRouteCollection(bits).Descend(func(item btree.Item) bool {
		if route := item.(*RouteGroup).Get(ipnet); route != nil {
			result <- route
			return false
		}

		return true
	})
	result <- nil

	return <-result
}

type RouteGroup struct {
	store     *btree.BTree
	PrefixLen int
}

func NewRouteGroup(prefixLen int) *RouteGroup {
	return &RouteGroup{
		store:     btree.New(2),
		PrefixLen: prefixLen,
	}
}

func (routeGroup *RouteGroup) Less(other btree.Item) bool {
	otherRouteGroup, ok := other.(*RouteGroup)
	if !ok {
		panic("other is not a *RouteGroup (impossible)")
	}
	return routeGroup.PrefixLen < otherRouteGroup.PrefixLen
}

type Route struct {
	CIDR  net.IPNet
	Value interface{}
}

func (route *Route) Less(other btree.Item) bool {
	return bytes.Compare(route.CIDR.IP, other.(*Route).CIDR.IP) < 0
}

func (routeGroup *RouteGroup) Insert(route *Route) {
	routeGroup.store.ReplaceOrInsert(route)
}

func (routeGroup *RouteGroup) Get(ipnet net.IPNet) *Route {
	_, bits := ipnet.Mask.Size()
	mask := net.CIDRMask(routeGroup.PrefixLen, bits)
	if item := routeGroup.store.Get(&Route{CIDR: net.IPNet{IP: ipnet.IP.Mask(mask), Mask: mask}}); item != nil {
		return item.(*Route)
	}
	return nil
}
